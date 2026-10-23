//go:build integration && darwin

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/layout"
	"github.com/bytedance/herdr-ui/internal/session"
)

func TestLayoutOperations(t *testing.T) {
	binary, err := exec.LookPath("herdr")
	if err != nil {
		t.Skip("Herdr not installed")
	}
	dir, err := os.MkdirTemp("/tmp", "herdr-m3-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target := herdr.Target{Socket: filepath.Join(dir, "api.sock")}
	_, env, err := target.Attach(binary, "unused", os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(config, []byte("[terminal]\ndefault_shell='/bin/sh'\nshell_mode='non_login'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	env = append(env, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+dir, "HERDR_CONFIG_PATH="+config)
	server := exec.Command(binary, "server")
	server.Env = env
	logFile, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	server.Stdout = logFile
	server.Stderr = logFile
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		herdr.Call(context.Background(), target, "server.stop", nil, nil)
		server.Process.Kill()
		server.Wait()
	}()
	wait(t, 5*time.Second, func() bool { return herdr.Call(context.Background(), target, "ping", nil, nil) == nil })

	var created struct {
		RootPane struct {
			ID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	if err := herdr.Call(context.Background(), target, "workspace.create", map[string]any{"cwd": dir, "label": "M3 Layout", "focus": true}, &created); err != nil {
		t.Fatal(err)
	}
	state := session.New(target)
	state.Start(context.Background(), nil)
	defer state.Close(context.Background())
	wait(t, 5*time.Second, func() bool {
		p := state.Latest()
		if p.Snapshot == nil || p.Stale {
			return false
		}
		_, ok := p.Snapshot.Pane(created.RootPane.ID)
		return p.State == session.Ready && ok
	})
	execute := func(kind session.OperationKind, paneID, direction string, modify func(*session.Operation)) session.OperationResult {
		t.Helper()
		wait(t, 5*time.Second, func() bool { p := state.Latest(); return p.State == session.Ready && !p.Stale })
		p := state.Latest()
		pane, ok := p.Snapshot.Pane(paneID)
		if !ok {
			t.Fatal("missing pane", paneID)
		}
		op := session.Operation{Kind: kind, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: pane.WorkspaceID, TabID: pane.TabID, PaneID: pane.ID, TerminalID: pane.TerminalID, Direction: direction}
		if modify != nil {
			modify(&op)
		}
		result := <-state.Execute(context.Background(), op)
		for deadline := time.Now().Add(5 * time.Second); result.Status == session.StaleTarget && time.Now().Before(deadline); {
			wait(t, 5*time.Second, func() bool { p := state.Latest(); return p.State == session.Ready && !p.Stale })
			result = <-state.Execute(context.Background(), op)
		}
		if result.Status != session.Succeeded {
			t.Fatalf("operation %v: %+v", kind, result)
		}
		state.Refresh()
		return result
	}
	awaitPanes := func(n int) {
		t.Helper()
		wait(t, 5*time.Second, func() bool {
			p := state.Latest()
			if p.Snapshot == nil || p.Stale {
				return false
			}
			pane, _ := p.Snapshot.Pane(created.RootPane.ID)
			return len(p.Snapshot.Panes(pane.TabID)) == n
		})
	}
	first := created.RootPane.ID
	right := execute(session.Split, first, "right", nil).PaneID
	awaitPanes(2)
	down := execute(session.Split, right, "down", nil).PaneID
	awaitPanes(3)
	wait(t, 5*time.Second, func() bool { return state.Latest().Snapshot.Metadata().FocusedPaneID == down })
	if got := execute(session.Neighbor, right, "down", nil).PaneID; got != down {
		t.Fatalf("neighbor=%q want %q", got, down)
	}
	// Read-only directional navigation must not change shared server focus.
	wait(t, 5*time.Second, func() bool { return state.Latest().Snapshot.Metadata().FocusedPaneID == down })
	pane, _ := state.Latest().Snapshot.Pane(first)
	execute(session.Zoom, first, "", nil)
	wait(t, 5*time.Second, func() bool {
		v, _ := state.Latest().Snapshot.Layout(pane.TabID)
		return v.Zoomed && len(v.Panes) == 1 && v.Panes[0].PaneID == first
	})
	execute(session.Zoom, first, "", nil)
	wait(t, 5*time.Second, func() bool { v, _ := state.Latest().Snapshot.Layout(pane.TabID); return !v.Zoomed && len(v.Panes) == 3 })
	exported := execute(session.ExportLayout, right, "", nil)
	paths, err := layout.SplitPaths(exported.Root)
	if err != nil || len(paths) != 2 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	path := []bool{true}
	node, err := layout.ResolvePath(exported.Root, path)
	if err != nil || node.Direction != "down" {
		t.Fatalf("nested split=%+v err=%v", node, err)
	}
	execute(session.SetRatio, right, "", func(op *session.Operation) { op.Path = path; op.Fingerprint = exported.Fingerprint; op.Ratio = .65 })
	fresh := execute(session.ExportLayout, right, "", nil)
	node, err = layout.ResolvePath(fresh.Root, path)
	if err != nil || node.Ratio != .65 {
		t.Fatalf("ratio=%+v err=%v", node, err)
	}
	if fresh.Fingerprint != exported.Fingerprint {
		t.Fatal("ratio altered structure fingerprint")
	}
	t.Log("real split right/down, read-only neighbor, zoom visibility and nested ratio validated")
}

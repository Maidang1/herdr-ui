//go:build integration && darwin

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/endpoint"
	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/session"
)

func TestShortcutRPCParity(t *testing.T) {
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
	if err := os.WriteFile(config, []byte(fmt.Sprintf("[terminal]\ndefault_shell='/bin/sh'\nshell_mode='non_login'\n[worktrees]\ndirectory='%s/worktrees'\n[[keys.command]]\nkey='prefix+y'\ncommand='true'\n", dir)), 0600); err != nil {
		t.Fatal(err)
	}
	env = append(env, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+dir, "HERDR_CONFIG_PATH="+config)
	for _, args := range [][]string{{"init"}, {"-c", "user.name=Keyboard Test", "-c", "user.email=keyboard@example.invalid", "commit", "--allow-empty", "-m", "test"}} {
		command := exec.Command("git", args...)
		command.Dir = dir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s %v", args, output, err)
		}
	}
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

	state := session.New(target)
	state.Start(context.Background(), nil)
	defer state.Close(context.Background())
	wait(t, 5*time.Second, func() bool { p := state.Latest(); return p.State == session.Ready && !p.Stale })
	run := func(action, paneID, text string, index int) session.OperationResult {
		t.Helper()
		wait(t, 5*time.Second, func() bool { p := state.Latest(); return p.State == session.Ready && !p.Stale })
		p := state.Latest()
		op := session.Operation{Kind: session.Shortcut, Action: action, Text: text, Index: index, TargetID: p.TargetID, Epoch: p.Epoch}
		if paneID != "" {
			pane, ok := p.Snapshot.Pane(paneID)
			if !ok {
				t.Fatalf("missing pane %s", paneID)
			}
			op.PaneID = pane.ID
			op.TerminalID = pane.TerminalID
			op.WorkspaceID = pane.WorkspaceID
			op.TabID = pane.TabID
		}
		result := <-state.Execute(context.Background(), op)
		if result.Status != session.Succeeded {
			t.Fatalf("%s: %+v", action, result)
		}
		state.Refresh()
		wait(t, 5*time.Second, func() bool {
			next := state.Latest()
			return next.State == session.Ready && !next.Stale && next.Revision > p.Revision
		})
		return result
	}
	created := run("new_workspace", "", dir, 0)
	paneID := created.PaneID
	if paneID == "" {
		t.Fatal("workspace create did not identify root pane")
	}
	run("rename_workspace", paneID, "Keyboard workspace", 0)
	run("rename_tab", paneID, "First tab", 0)
	run("rename_pane", paneID, "First pane", 0)
	second := run("new_tab", paneID, "", 0).PaneID
	run("move_tab_previous", second, "", 0)
	run("move_tab_next", second, "", 2)
	run("close_tab", second, "", 0)
	if err := herdr.Call(context.Background(), target, "pane.send_text", map[string]any{"pane_id": paneID, "text": "i=1; while [ $i -le 1200 ]; do printf 'HISTORY-%04d\n' \"$i\"; i=$((i+1)); done\n"}, nil); err != nil {
		t.Fatal(err)
	}
	// Wait for terminal output to settle; history capture must cover >1000 rows.
	wait(t, 5*time.Second, func() bool {
		var out struct {
			Read struct {
				Text string `json:"text"`
			} `json:"read"`
		}
		err := herdr.Call(context.Background(), target, "pane.read", map[string]any{"pane_id": paneID, "source": "recent"}, &out)
		return err == nil && strings.Contains(out.Read.Text, "HISTORY-1200")
	})
	history := run("read_scrollback", paneID, "", 0)
	var capture struct {
		Read struct {
			Text string `json:"text"`
		} `json:"read"`
	}
	if err := json.Unmarshal(history.Data, &capture); err != nil || !strings.Contains(capture.Read.Text, "HISTORY-0001") {
		t.Fatalf("history: %s %v", history.Data, err)
	}
	worktree := run("new_worktree", paneID, "keyboard-branch", 0)
	var worktreeData struct {
		Worktree struct {
			Path string `json:"path"`
		} `json:"worktree"`
	}
	if err := json.Unmarshal(worktree.Data, &worktreeData); err != nil {
		t.Fatal(err)
	}
	run("open_worktree", paneID, worktreeData.Worktree.Path, 0)
	run("remove_worktree", worktree.PaneID, "", 0)
	run("reload_config", paneID, "", 0)
	control := endpoint.New(target.Socket)
	if err := control.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer control.Close()
	wait(t, 5*time.Second, func() bool { return control.Latest().Snapshot != nil })
	if err := herdr.Call(context.Background(), target, "notification.show", map[string]any{"title": "Keyboard test", "pane_id": paneID}, nil); err != nil {
		t.Fatal(err)
	}
	wait(t, 5*time.Second, func() bool { return len(control.Latest().Notifications) > 0 })
	commands := control.Latest().Snapshot.Commands
	if len(commands) != 1 {
		t.Fatalf("commands: %+v", commands)
	}
	publication := state.Latest()
	pane, _ := publication.Snapshot.Pane(paneID)
	selected := "HISTORY-0001"
	invocation := <-state.Execute(context.Background(), session.Operation{Kind: session.Shortcut, Action: "invoke_command", CommandID: commands[0].CommandID, SelectedText: &selected, TargetID: publication.TargetID, Epoch: publication.Epoch, WorkspaceID: pane.WorkspaceID, TabID: pane.TabID, PaneID: pane.ID, TerminalID: pane.TerminalID})
	if invocation.Status != session.Succeeded {
		t.Fatalf("command with resolved selection: %+v", invocation)
	}
	var imageBytes bytes.Buffer
	if err := png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	control.SetSurfaceSize(80, 24)
	imageCtx, cancelImage := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelImage()
	if err := control.SendImage(imageCtx, paneID, "png", imageBytes.Bytes()); err != nil {
		t.Fatalf("image paste: %v", err)
	}

	run("close_workspace", paneID, "", 0)
}

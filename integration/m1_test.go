//go:build integration && (darwin || linux)

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
)

func TestEventDrivenSession(t *testing.T) {
	binary := unixTestBinary(t)
	dir, err := os.MkdirTemp("/tmp", "herdr-m1-")
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
	state := session.New(target)
	state.Start(context.Background(), nil)
	defer state.Close(context.Background())
	waitReady := func(ok func(*session.Snapshot) bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			p := state.Latest()
			if p.State == session.Incompatible {
				t.Fatalf("incompatible: %s", p.Error)
			}
			if p.State == session.Ready && ok(p.Snapshot) {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("state timed out: %+v", state.Latest())
	}
	waitReady(func(s *session.Snapshot) bool { return true })
	// These mutations are external to Session, which sends read-only requests.
	var created struct {
		Workspace struct {
			ID string `json:"workspace_id"`
		} `json:"workspace"`
		RootPane struct {
			ID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	if err := herdr.Call(context.Background(), target, "workspace.create", map[string]any{"cwd": dir, "label": "M1 Events", "focus": false}, &created); err != nil {
		t.Fatal(err)
	}
	waitReady(func(s *session.Snapshot) bool {
		w, ok := s.Workspace(created.Workspace.ID)
		return ok && w.Label == "M1 Events"
	})
	epoch := state.Latest().Epoch
	if err := herdr.Call(context.Background(), target, "workspace.rename", map[string]any{"workspace_id": created.Workspace.ID, "label": "M1 Renamed"}, nil); err != nil {
		t.Fatal(err)
	}
	waitReady(func(s *session.Snapshot) bool {
		w, ok := s.Workspace(created.Workspace.ID)
		return ok && w.Label == "M1 Renamed"
	})
	if err := herdr.Call(context.Background(), target, "pane.report_agent", map[string]any{"pane_id": created.RootPane.ID, "source": "m1-test", "agent": "codex", "state": "working", "seq": 1}, nil); err != nil {
		t.Fatal(err)
	}
	waitReady(func(s *session.Snapshot) bool {
		p, ok := s.Pane(created.RootPane.ID)
		return ok && p.Agent == "codex" && p.Status == "working"
	})
	if err := herdr.Call(context.Background(), target, "pane.report_agent", map[string]any{"pane_id": created.RootPane.ID, "source": "m1-test", "agent": "codex", "state": "blocked", "seq": 2}, nil); err != nil {
		t.Fatal(err)
	}
	waitReady(func(s *session.Snapshot) bool {
		p, ok := s.Pane(created.RootPane.ID)
		return ok && p.Status == "blocked"
	})
	if err := herdr.Call(context.Background(), target, "workspace.close", map[string]any{"workspace_id": created.Workspace.ID}, nil); err != nil {
		t.Fatal(err)
	}
	waitReady(func(s *session.Snapshot) bool { _, ok := s.Workspace(created.Workspace.ID); return !ok })
	if state.Latest().Epoch != epoch {
		t.Fatal("subscription replacement caused spurious reconnect")
	}
	t.Log("external creation, rename, agent working/blocked and close updated the session through events")
}

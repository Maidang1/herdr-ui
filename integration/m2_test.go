//go:build integration && darwin

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/egoist/mygo/plugins/terminal"
)

func TestDefaultWindowAttachesAndClosesLocally(t *testing.T) {
	windowBinary := os.Getenv("HERDR_UI_M0_WINDOW_BINARY")
	if windowBinary == "" {
		t.Skip("set HERDR_UI_M0_WINDOW_BINARY to test the real window")
	}
	binary, err := exec.LookPath("herdr")
	if err != nil {
		t.Skip("Herdr not installed")
	}
	dir, err := os.MkdirTemp("/tmp", "herdr-m2-")
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
	env = append(env, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+dir, "HERDR_CONFIG_PATH="+config, "HERDR_SESSION=", "HERDR_PANE_ID=", "HERDR_TAB_ID=", "HERDR_WORKSPACE_ID=")
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
			ID         string `json:"pane_id"`
			TerminalID string `json:"terminal_id"`
		} `json:"root_pane"`
	}
	if err := herdr.Call(context.Background(), target, "workspace.create", map[string]any{"cwd": dir, "label": "M2 Window", "focus": true, "env": map[string]string{"PS1": "M2_READY> "}}, &created); err != nil {
		t.Fatal(err)
	}
	paneID, terminalID := created.RootPane.ID, created.RootPane.TerminalID
	args, attachEnv, err := target.Attach(binary, terminalID, env)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := terminal.New(terminal.Options{Command: args, Env: attachEnv})
	if err != nil {
		t.Fatal(err)
	}
	seed.Resize(80, 24)
	waitScreen(t, seed, "M2_READY>")
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-seed.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("seed attach process did not exit")
	}
	window := exec.Command(windowBinary, "-socket", target.Socket, "-close-after", "5s")
	window.Env = env
	windowLog, err := os.Create(filepath.Join(dir, "window.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer windowLog.Close()
	window.Stdout = windowLog
	window.Stderr = windowLog
	if err := window.Start(); err != nil {
		t.Fatal(err)
	}
	defer window.Process.Kill()
	wait(t, 5*time.Second, func() bool {
		var result struct {
			Snapshot struct {
				Panes []struct {
					ID     string `json:"pane_id"`
					Scroll struct {
						Rows int `json:"viewport_rows"`
					} `json:"scroll"`
				} `json:"panes"`
			} `json:"snapshot"`
		}
		if herdr.Call(context.Background(), target, "session.snapshot", nil, &result) != nil {
			return false
		}
		for _, p := range result.Snapshot.Panes {
			if p.ID == paneID {
				return p.Scroll.Rows > 31
			}
		}
		return false
	})
	// The native default window has the direct attach owner; it must not have
	// merely rendered a static pane summary or started a replacement shell.
	second, err := terminal.New(terminal.Options{Command: args, Env: attachEnv})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	second.Resize(80, 24)
	select {
	case <-second.Done():
		if second.ExitCode() == 0 {
			t.Fatal("default window did not own attach")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second owner not rejected")
	}
	done := make(chan error, 1)
	go func() { done <- window.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			data, _ := os.ReadFile(filepath.Join(dir, "window.log"))
			t.Fatalf("window: %v %s", err, data)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("window shutdown exceeded budget")
	}
	third := reattach(t, func() *terminal.Terminal {
		term, err := terminal.New(terminal.Options{Command: args, Env: attachEnv})
		if err != nil {
			t.Fatal(err)
		}
		return term
	}, "M2_READY>")
	defer third.Close()
	third.Send([]byte("printf '\\n\\1152_ALIVE_AFTER_WINDOW\\n'\r"))
	waitScreen(t, third, "M2_ALIVE_AFTER_WINDOW")
	data, _ := os.ReadFile(filepath.Join(dir, "window.log"))
	if !strings.Contains(string(data), "state=Ready") {
		t.Fatalf("session never Ready: %s", data)
	}
	t.Log("default state window owns a real attach, drives size, closes within budget, and preserves the same remote terminal")

	// Verify M2's explicit scroll operation and visible-pane subscription against
	// the real service; output creates authoritative history before scrolling.
	third.Send([]byte("i=0; while [ $i -lt 80 ]; do echo M2_LINE_$i; i=$((i+1)); done; printf '\\1152_HISTORY_READY\\n'\r"))
	waitScreen(t, third, "M2_HISTORY_READY")
	state := session.New(target)
	state.Start(context.Background(), nil)
	defer state.Close(context.Background())
	wait(t, 5*time.Second, func() bool { return state.Latest().State == session.Ready })
	state.ObserveScroll(paneID, terminalID)
	wait(t, 5*time.Second, func() bool {
		p := state.Latest()
		if p.State != session.Ready {
			return false
		}
		pane, ok := p.Snapshot.Pane(paneID)
		return ok && pane.Scroll.Known && pane.Scroll.MaxOffset > 10
	})
	p := state.Latest()
	result := <-state.Execute(context.Background(), session.Operation{Kind: session.Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: paneID, TerminalID: terminalID, Offset: 5})
	if result.Status != session.Succeeded || result.Scroll.Offset != 5 {
		t.Fatal(result)
	}
	wait(t, 5*time.Second, func() bool {
		p := state.Latest()
		if p.State != session.Ready {
			return false
		}
		pane, _ := p.Snapshot.Pane(paneID)
		return pane.Scroll.Offset == 5
	})
	t.Log("server scroll operation and authoritative scroll metrics passed")
}

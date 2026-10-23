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

func TestDefaultWindowAttachesAndClosesLocally(t *testing.T) { testWindowLifecycle(t, false) }
func TestMultiPaneWindowAttachesAllVisibleAndClosesLocally(t *testing.T) {
	testWindowLifecycle(t, true)
}
func testWindowLifecycle(t *testing.T, multi bool) {
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
	if multi {
		// Keep the remote shell producing output while the native multi-pane
		// window takes ownership and applies its first real content dimensions.
		seed.Send([]byte("printf '\\n\\1154_OUTPUT_STARTED\\n'; i=0; while [ $i -lt 200 ]; do echo M4_OUTPUT_$i; i=$((i+1)); sleep 0.01; done; printf '\\n\\1154_OUTPUT_DONE\\n'\r"))
		waitScreen(t, seed, "M4_OUTPUT_STARTED")
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-seed.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("seed attach process did not exit")
	}
	terminals := []string{terminalID}
	if multi {
		source := paneID
		for _, direction := range []string{"right", "down"} {
			var split struct {
				Pane struct {
					ID         string `json:"pane_id"`
					TerminalID string `json:"terminal_id"`
				} `json:"pane"`
			}
			if err := herdr.Call(context.Background(), target, "pane.split", map[string]any{"target_pane_id": source, "direction": direction, "focus": true}, &split); err != nil {
				t.Fatal(err)
			}
			source = split.Pane.ID
			terminals = append(terminals, split.Pane.TerminalID)
		}
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
	// Every visible terminal must have an attach owner in the native window.
	for _, id := range terminals {
		attachArgs, attachEnvironment, err := target.Attach(binary, id, env)
		if err != nil {
			t.Fatal(err)
		}
		second, err := terminal.New(terminal.Options{Command: attachArgs, Env: attachEnvironment})
		if err != nil {
			t.Fatal(err)
		}
		second.Resize(80, 24)
		select {
		case <-second.Done():
			if second.ExitCode() == 0 {
				t.Fatalf("window did not own %s", id)
			}
		case <-time.After(3 * time.Second):
			second.Close()
			t.Fatalf("second owner not rejected for %s", id)
		}
		second.Close()
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
	for _, id := range terminals[1:] {
		attachArgs, attachEnvironment, err := target.Attach(binary, id, env)
		if err != nil {
			t.Fatal(err)
		}
		term := reattach(t, func() *terminal.Terminal {
			v, err := terminal.New(terminal.Options{Command: attachArgs, Env: attachEnvironment})
			if err != nil {
				t.Fatal(err)
			}
			return v
		}, "$")
		term.Close()
	}
	third := reattach(t, func() *terminal.Terminal {
		term, err := terminal.New(terminal.Options{Command: args, Env: attachEnv})
		if err != nil {
			t.Fatal(err)
		}
		return term
	}, "M2_READY>")
	defer third.Close()
	if multi {
		waitScreen(t, third, "M4_OUTPUT_DONE")
	}
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
	var result session.OperationResult
	// Output/scroll events can start a resync after the Ready observation.
	// StaleTarget guarantees the request was not sent; only that test-side
	// rejection may be retried. Never repeat an unknown mutation outcome.
	wait(t, 5*time.Second, func() bool {
		p := state.Latest()
		if p.State != session.Ready || p.Stale {
			return false
		}
		result = <-state.Execute(context.Background(), session.Operation{Kind: session.Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: paneID, TerminalID: terminalID, Offset: 5})
		return result.Status != session.StaleTarget
	})
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

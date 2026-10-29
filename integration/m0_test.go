//go:build integration && (darwin || linux)

// These tests use the real native library and a disposable Herdr server.
// They must never connect to the user's inherited socket/session.
package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/egoist/mygo/plugins/terminal"
)

func TestDirectAttach(t *testing.T) {
	binary := unixTestBinary(t)
	// Short paths avoid macOS's Unix-domain socket path length limit.
	dir, err := os.MkdirTemp("/tmp", "herdr-ui-m0-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	target := herdr.Target{Socket: filepath.Join(dir, "herdr.sock")}
	_, env, err := target.Attach(binary, "unused", os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+dir, "HERDR_CONFIG_PATH="+filepath.Join(dir, "config.toml"))
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[terminal]\ndefault_shell = '/bin/sh'\nshell_mode = 'non_login'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	server := exec.Command(binary, "server")
	server.Env = env
	log, err := os.Create(filepath.Join(dir, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	server.Stdout = log
	server.Stderr = log
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		herdr.Call(ctx, target, "server.stop", nil, nil)
		server.Process.Kill()
		server.Wait()
	}()
	wait(t, 5*time.Second, func() bool { return herdr.Call(context.Background(), target, "ping", nil, nil) == nil })
	var created struct {
		RootPane struct {
			PaneID     string `json:"pane_id"`
			TerminalID string `json:"terminal_id"`
			TabID      string `json:"tab_id"`
		} `json:"root_pane"`
	}
	if err := herdr.Call(context.Background(), target, "workspace.create", map[string]any{"cwd": dir, "label": "M0 disposable", "focus": false, "env": map[string]string{"PS1": "M0_PROMPT_READY> "}}, &created); err != nil {
		t.Fatal(err)
	}
	p := created.RootPane
	if p.PaneID == "" || p.TerminalID == "" || p.TabID == "" {
		t.Fatalf("missing IDs: %+v", created)
	}
	args, env, err := target.Attach(binary, p.TerminalID, env)
	if err != nil {
		t.Fatal(err)
	}
	newTerm := func() *terminal.Terminal {
		term, err := terminal.New(terminal.Options{Command: args, Env: env})
		if err != nil {
			t.Fatal(err)
		}
		return term
	}
	first := newTerm()
	defer first.Close()
	first.Resize(91, 27) // Only in the headless probe; production sizing is View-owned.
	waitScreen(t, first, "M0_PROMPT_READY>")
	first.Send([]byte("printf '\\n\\1150_STREAM_OK\\n'\r"))
	waitScreen(t, first, "M0_STREAM_OK")
	second := newTerm()
	defer second.Close()
	second.Resize(91, 27)
	select {
	case <-second.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("second attach was not rejected")
	}
	if second.ExitCode() == 0 {
		t.Fatalf("second owner succeeded: %q", second.Text())
	}
	t.Logf("second owner rejected (exit=%d)", second.ExitCode())
	select {
	case <-first.Done():
		t.Fatal("second owner disconnected first")
	default:
	}
	// Resize the local PTY through the public plugin and read authoritative rows.
	first.Resize(103, 31)
	wait(t, 5*time.Second, func() bool {
		var snapshot struct {
			Snapshot struct {
				Panes []struct {
					PaneID string `json:"pane_id"`
					Scroll struct {
						Rows int `json:"viewport_rows"`
					} `json:"scroll"`
				} `json:"panes"`
			} `json:"snapshot"`
		}
		return herdr.Call(context.Background(), target, "session.snapshot", nil, &snapshot) == nil && func() bool {
			for _, x := range snapshot.Snapshot.Panes {
				if x.PaneID == p.PaneID {
					return x.Scroll.Rows == 31
				}
			}
			return false
		}()
	})
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("closed local attach process did not exit")
	}
	// Closing the plugin must leave the same remote terminal alive and attachable.
	third := reattach(t, newTerm, "M0_PROMPT_READY>")
	defer third.Close()
	third.Send([]byte("printf '\\n\\1150_SURVIVED_CLOSE\\n'\r"))
	waitScreen(t, third, "M0_SURVIVED_CLOSE")
	third.Send([]byte("stty raw -echo; printf '\\n\\1150_RAW_READY\\n'; od -An -tx1 -N2; stty sane\r"))
	waitScreen(t, third, "M0_RAW_READY")
	third.Send([]byte{2, 2, 'q'})
	waitScreen(t, third, "02 71")
	select {
	case <-third.Done():
		t.Fatal("literal Ctrl-B followed by q detached the client")
	default:
	}
	t.Log("literal Ctrl-B/q crossed the attach escape layer without detach")
	t.Log("stream, single owner, 103x31 resize, close and reattach passed")
	if windowBinary := os.Getenv("HERDR_UI_M0_WINDOW_BINARY"); windowBinary != "" {
		if err := third.Close(); err != nil {
			t.Fatal(err)
		}
		window := exec.Command(windowBinary, "-herdr", binary, "-socket", target.Socket, "-terminal", p.TerminalID, "-close-after", "2s")
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
			var out struct {
				Snapshot struct {
					Panes []struct {
						PaneID string `json:"pane_id"`
						Scroll struct {
							Rows int `json:"viewport_rows"`
						} `json:"scroll"`
					} `json:"panes"`
				} `json:"snapshot"`
			}
			if herdr.Call(context.Background(), target, "session.snapshot", nil, &out) != nil {
				return false
			}
			for _, pane := range out.Snapshot.Panes {
				if pane.PaneID == p.PaneID {
					return pane.Scroll.Rows > 31
				}
			}
			return false
		})
		done := make(chan error, 1)
		go func() { done <- window.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(6 * time.Second):
			t.Fatal("native window did not exit within budget")
		}
		data, err := os.ReadFile(filepath.Join(dir, "window.log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "M0 terminal closed") {
			t.Fatalf("native close missing: %s", data)
		}
		t.Log("real native View drove remote rows above 31 and closed its attach")
	}

	// Build a nested tree only inside this disposable server, then round-trip
	// the exact bool path to the server and check the returned node.
	var split struct {
		Pane struct {
			PaneID string `json:"pane_id"`
		} `json:"pane"`
	}
	if err := herdr.Call(context.Background(), target, "pane.split", map[string]any{"target_pane_id": p.PaneID, "direction": "right", "ratio": 0.5, "focus": false}, &split); err != nil {
		t.Fatal(err)
	}
	if split.Pane.PaneID == "" {
		t.Fatal("split returned no pane ID")
	}
	if err := herdr.Call(context.Background(), target, "pane.split", map[string]any{"target_pane_id": split.Pane.PaneID, "direction": "down", "ratio": 0.5, "focus": false}, nil); err != nil {
		t.Fatal(err)
	}
	var changed struct {
		Layout struct {
			Root layout.Node `json:"root"`
		} `json:"layout"`
	}
	if err := herdr.Call(context.Background(), target, "layout.set_split_ratio", map[string]any{"tab_id": p.TabID, "path": []bool{true}, "ratio": 0.63}, &changed); err != nil {
		t.Fatal(err)
	}
	node, err := layout.ResolvePath(&changed.Layout.Root, []bool{true})
	if err != nil || node.Type != "split" || node.Ratio < 0.62 || node.Ratio > 0.64 {
		b, _ := json.Marshal(changed)
		t.Fatalf("ratio path mismatch: %s %v", b, err)
	}
	if changed.Layout.Root.Ratio != 0.5 {
		t.Fatal("nested path changed root ratio")
	}
	t.Log("real nested layout ratio path [true] passed")
}

func waitScreen(t *testing.T, term *terminal.Terminal, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(strings.Join(strings.Fields(term.Text()), " "), marker) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("missing %q; isolated screen=%q", marker, term.Text())
}

// A server can observe the local process's socket EOF on a later event-loop
// turn. Only this test probe retries owner rejection after explicit local close;
// application reconnect remains an explicit Retry and never uses takeover.
func reattach(t *testing.T, create func() *terminal.Terminal, marker string) *terminal.Terminal {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var screen string
	for time.Now().Before(deadline) {
		term := create()
		term.Resize(91, 27)
		for time.Now().Before(deadline) {
			screen = term.Text()
			if strings.Contains(screen, marker) {
				return term
			}
			select {
			case <-term.Done():
				screen = term.Text()
				term.Close()
				if !strings.Contains(screen, "already has an attached client") {
					t.Fatalf("unexpected reattach failure: %q", screen)
				}
				time.Sleep(20 * time.Millisecond)
				goto retry
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
		term.Close()
	retry:
	}
	t.Fatalf("server did not release prior attach owner: %q", screen)
	return nil
}

func wait(t *testing.T, timeout time.Duration, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}

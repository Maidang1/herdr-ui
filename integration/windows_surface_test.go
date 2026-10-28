//go:build windows && integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/localipc"
	"github.com/Microsoft/go-winio"
)

// TestWindowsNativeSurface runs only with an explicitly supplied runtime. Every
// daemon, pipe, config and child shell belongs to this test's temporary root.
func TestWindowsNativeSurface(t *testing.T) {
	binary := os.Getenv("HERDR_TEST_BINARY")
	if binary == "" {
		t.Skip("set HERDR_TEST_BINARY to an explicit Herdr Windows executable")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(absolute); err != nil || info.IsDir() {
		t.Fatalf("invalid HERDR_TEST_BINARY %q: %v", absolute, err)
	}
	root := t.TempDir()
	target := herdr.Target{Socket: filepath.Join(root, "isolated-api.sock")}
	config := filepath.Join(root, "config.toml")
	shell, err := exec.LookPath("pwsh.exe")
	if err != nil {
		shell, err = exec.LookPath("powershell.exe")
	}
	if err != nil {
		t.Fatal("PowerShell unavailable")
	}
	// TOML literal strings preserve Windows path separators.
	if err := os.WriteFile(config, []byte(fmt.Sprintf("[terminal]\ndefault_shell = '%s'\nshell_mode = 'non_login'\n", strings.ReplaceAll(shell, "'", "''"))), 0600); err != nil {
		t.Fatal(err)
	}
	env := isolatedWindowsEnv(root, config, target.Socket)
	log, err := os.Create(filepath.Join(root, "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	server := exec.Command(absolute, "server")
	server.Env = env
	server.Dir = root
	server.Stdout = log
	server.Stderr = log
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Wait() }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = herdr.Call(ctx, target, "server.stop", nil, nil)
		select {
		case <-done:
		case <-ctx.Done():
			_ = server.Process.Kill()
			<-done
		}
		if t.Failed() {
			if data, err := os.ReadFile(log.Name()); err == nil {
				if len(data) > 65536 {
					data = data[len(data)-65536:]
				}
				t.Logf("isolated daemon log:\n%s", data)
			}
		}
	}()
	waitWindows(t, 20*time.Second, func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		return herdr.Call(ctx, target, "ping", nil, nil) == nil
	})
	// Dial the exact namespaced pipe used by the real daemon, without Unix sockets.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pipe, err := winio.DialPipeContext(ctx, localipc.WindowsPipeName(target.Socket))
	if err != nil {
		t.Fatalf("API named pipe: %v", err)
	}
	pipe.Close()
	var created struct {
		RootPane struct {
			PaneID     string `json:"pane_id"`
			TabID      string `json:"tab_id"`
			TerminalID string `json:"terminal_id"`
		} `json:"root_pane"`
	}
	if err := herdr.Call(ctx, target, "workspace.create", map[string]any{"cwd": root, "label": "Windows isolated surface", "focus": true}, &created); err != nil {
		t.Fatal(err)
	}
	pane := created.RootPane
	if pane.PaneID == "" || pane.TabID == "" || pane.TerminalID == "" {
		t.Fatalf("missing pane identities: %+v", created)
	}
	connect := func() *endpoint.Client {
		t.Helper()
		client := endpoint.NewSurface(target.Socket)
		t.Cleanup(func() {
			if t.Failed() {
				state := client.Latest()
				t.Logf("endpoint snapshot=%+v surface projection=%+v error=%v\nvisible surface:\n%s", state.Snapshot, state.SurfaceSnapshot, state.Err, windowsSurfaceText(state.Surface))
			}
		})
		t.Cleanup(func() { _ = client.Close() })
		connectionCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		// Start's context owns the connection, so use the test lifetime instead.
		if err := client.Start(context.Background()); err != nil {
			t.Fatal(err)
		}
		waitWindows(t, 10*time.Second, func() bool { s := client.Latest(); return s.Connected && s.Snapshot != nil })
		for _, action := range []func() error{
			func() error { return client.ResizeSurface(connectionCtx, 100, 32) },
			func() error { return client.SetSurfaceActive(connectionCtx, true) },
			func() error { return client.FocusTab(connectionCtx, pane.TabID) },
			func() error { return client.FocusPane(connectionCtx, pane.PaneID) },
			func() error { return client.FocusSurface(connectionCtx, true) },
		} {
			if err := action(); err != nil {
				client.Close()
				t.Fatal(err)
			}
		}
		waitWindows(t, 10*time.Second, func() bool { return client.Latest().Surface != nil })
		return client
	}
	client := connect()
	defer client.Close()
	boot := client.Latest().Snapshot.BootID
	// The contiguous marker is absent from the command's echo; only execution can
	// produce it. TextCommit contains CJK and a non-BMP scalar, followed by Enter.
	marker := "HERDR_汉字_😀_OUTPUT"
	command := "$OutputEncoding=[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new(); Write-Output ('HERDR_'+'汉字_😀_OUTPUT')"
	event, err := endpoint.TextEvent(command, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendPaneEvent(ctx, boot, pane.PaneID, event, false); err != nil {
		t.Fatal(err)
	}
	enter, err := endpoint.KeyEvent(1, 0, 0, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendPaneEvent(ctx, boot, pane.PaneID, enter, false); err != nil {
		t.Fatal(err)
	}
	waitWindows(t, 30*time.Second, func() bool { return strings.Contains(windowsSurfaceText(client.Latest().Surface), marker) })
	resizeCtx, resizeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer resizeCancel()
	if err := client.ResizeSurface(resizeCtx, 120, 40); err != nil {
		t.Fatal(err)
	}
	waitWindows(t, 10*time.Second, func() bool {
		s := client.Latest().Surface
		return s != nil && s.Frame.Width == 120 && s.Frame.Height == 40
	})
	client.Close()
	reconnected := connect()
	defer reconnected.Close()
	waitWindows(t, 10*time.Second, func() bool {
		s := reconnected.Latest()
		return s.Connected && s.Snapshot != nil && s.Snapshot.BootID == boot && s.Surface != nil && s.SurfaceSnapshot != nil && s.SurfaceSnapshot.FocusedTabID == pane.TabID
	})
	// Resizing a PowerShell/PSReadLine viewport can move old output into
	// scrollback. Persistence is a history assertion, not a fixed-screen one.
	var history struct {
		Text string `json:"text"`
	}
	if err := herdr.Call(ctx, target, "pane.read", map[string]any{"pane_id": pane.PaneID, "source": "recent_unwrapped", "format": "text", "lines": 200}, &history); err != nil || !strings.Contains(history.Text, marker) {
		t.Fatalf("reconnected pane lost output: %v\n%s", err, history.Text)
	}
	continued, err := endpoint.TextEvent("Write-Output ('RECONNECT_'+'EXECUTED')", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := reconnected.SendPaneEvent(ctx, boot, pane.PaneID, continued, false); err != nil {
		t.Fatal(err)
	}
	if err := reconnected.SendPaneEvent(ctx, boot, pane.PaneID, enter, false); err != nil {
		t.Fatal(err)
	}
	waitWindows(t, 20*time.Second, func() bool {
		return strings.Contains(windowsSurfaceText(reconnected.Latest().Surface), "RECONNECT_EXECUTED")
	})
	// Closing a frontend connection must leave its own daemon and child pane alive.
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := herdr.Call(pingCtx, target, "ping", nil, nil); err != nil {
		t.Fatalf("daemon lost after reconnect: %v", err)
	}
}

func isolatedWindowsEnv(root, config, socket string) []string {
	env := make([]string, 0, len(os.Environ())+12)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "HERDR_") {
			continue
		}
		switch upper {
		case "HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME":
			continue
		}
		env = append(env, item)
	}
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		env = append(env, key+"="+root)
	}
	return append(env, "HERDR_CONFIG_PATH="+config, "HERDR_SOCKET_PATH="+socket, "HERDR_SESSION=windows-surface-test", "TERM=xterm-256color")
}
func waitWindows(t *testing.T, timeout time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("isolated Windows surface condition timed out after %s", timeout)
}
func windowsSurfaceText(frame *endpoint.SurfaceFrame) string {
	if frame == nil {
		return ""
	}
	var out strings.Builder
	for i, cell := range frame.Frame.Cells {
		if !cell.Skip {
			out.WriteString(cell.Symbol)
		}
		if frame.Frame.Width > 0 && (i+1)%int(frame.Frame.Width) == 0 {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

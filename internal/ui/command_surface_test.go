package ui

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

type fakeCommandSurface struct {
	closed  chan struct{}
	release chan struct{}
	once    sync.Once
}

func (f *fakeCommandSurface) View(*native.Context) native.Element {
	panic("test must not initialize UI")
}
func (f *fakeCommandSurface) Close() error {
	f.once.Do(func() { close(f.closed) })
	if f.release != nil {
		<-f.release
	}
	return nil
}

type surfaceHarness struct {
	w       *Window
	posts   chan func()
	options chan terminal.Options
	results chan commandSurfaceTerminal
}

func newSurfaceHarness(t *testing.T) *surfaceHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	h := &surfaceHarness{posts: make(chan func(), 30), options: make(chan terminal.Options, 10), results: make(chan commandSurfaceTerminal, 10)}
	h.w = &Window{terminal: &terminalPanel{ctx: ctx, post: func(f func()) { h.posts <- f }, focused: "pane"}}
	t.Cleanup(func() { h.w.CloseCommandSurface(); cancel() })
	return h
}
func (h *surfaceHarness) factory(o terminal.Options) (commandSurfaceTerminal, error) {
	h.options <- o
	return <-h.results, nil
}
func receiveSurface[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		t.Fatal("command surface operation timed out")
		var zero T
		return zero
	}
}
func drainSurfacePosts(h *surfaceHarness) {
	for {
		select {
		case f := <-h.posts:
			f()
		default:
			return
		}
	}
}

func TestCommandSurfaceClosesLateCompletion(t *testing.T) {
	h := newSurfaceHarness(t)
	dir := t.TempDir()
	binary := filepath.Join(dir, "herdr")
	socketEnv := "HERDR_SOCKET_PATH=" + filepath.Join(dir, "test.sock")
	cmd, env := []string{binary, "--session", "test"}, []string{socketEnv}
	if err := h.w.openCommandSurface("Herdr", cmd, env, dir, h.factory); err != nil {
		t.Fatal(err)
	}
	o := receiveSurface(t, h.options)
	cmd[0] = "/different"
	env[0] = "HERDR_SOCKET_PATH=/wrong"
	if o.Command[0] != binary || o.Dir != dir || o.Env[len(o.Env)-1] != socketEnv || o.OnKey != nil {
		t.Fatal(o)
	}
	if h.w.terminal.focused != "" {
		t.Fatal("main input not blurred")
	}
	h.w.CloseCommandSurface()
	f := &fakeCommandSurface{closed: make(chan struct{})}
	h.results <- f
	receiveSurface(t, f.closed)
	drainSurfacePosts(h)
	if h.w.commandSurfaceState != nil {
		t.Fatal("closed overlay resurrected")
	}
}
func TestCommandSurfaceReplacementIgnoresOldExit(t *testing.T) {
	h := newSurfaceHarness(t)
	if err := h.w.openCommandSurface("First", []string{filepath.Join(t.TempDir(), "first")}, nil, "", h.factory); err != nil {
		t.Fatal(err)
	}
	firstOpts := receiveSurface(t, h.options)
	first := &fakeCommandSurface{closed: make(chan struct{})}
	h.results <- first
	receiveSurface(t, h.posts)() // Installation repaint.
	if err := h.w.openCommandSurface("Second", []string{filepath.Join(t.TempDir(), "second")}, nil, "", h.factory); err != nil {
		t.Fatal(err)
	}
	receiveSurface(t, h.options)
	current := h.w.commandSurfaceState
	second := &fakeCommandSurface{closed: make(chan struct{})}
	h.results <- second
	firstOpts.OnExit(0)
	drainSurfacePosts(h)
	if h.w.commandSurfaceState != current {
		t.Fatal("old child exit closed new child")
	}
	receiveSurface(t, first.closed)
	h.w.CloseCommandSurface()
	receiveSurface(t, second.closed)
}
func TestCommandSurfaceExitBeforeFactoryReturn(t *testing.T) {
	h := newSurfaceHarness(t)
	f := &fakeCommandSurface{closed: make(chan struct{})}
	if err := h.w.openCommandSurface("Short", []string{filepath.Join(t.TempDir(), "true")}, nil, "", func(o terminal.Options) (commandSurfaceTerminal, error) { o.OnExit(0); return f, nil }); err != nil {
		t.Fatal(err)
	}
	receiveSurface(t, f.closed)
	receiveSurface(t, h.posts)()
	drainSurfacePosts(h)
	if h.w.commandSurfaceState != nil {
		t.Fatal("already exited child installed")
	}
}
func TestCommandSurfaceCloseNeverWaitsForPTY(t *testing.T) {
	h := newSurfaceHarness(t)
	f := &fakeCommandSurface{closed: make(chan struct{}), release: make(chan struct{})}
	defer close(f.release)
	if err := h.w.openCommandSurface("Blocking", []string{filepath.Join(t.TempDir(), "slow")}, nil, "", func(terminal.Options) (commandSurfaceTerminal, error) { return f, nil }); err != nil {
		t.Fatal(err)
	}
	receiveSurface(t, h.posts)()
	// This call would deadlock the test if cleanup ran on the UI thread.
	h.w.CloseCommandSurface()
	receiveSurface(t, f.closed)
	if h.w.commandSurfaceState != nil {
		t.Fatal("closed overlay retained")
	}
}
func TestCommandSurfaceStartupErrorAndValidation(t *testing.T) {
	h := newSurfaceHarness(t)
	if err := h.w.OpenCommandSurface("Bad", []string{"relative"}, nil, ""); err == nil {
		t.Fatal("relative executable accepted")
	}
	want := errors.New("could not create PTY")
	if err := h.w.openCommandSurface("Failure", []string{filepath.Join(t.TempDir(), "missing")}, nil, "", func(terminal.Options) (commandSurfaceTerminal, error) { return nil, want }); err != nil {
		t.Fatal(err)
	}
	receiveSurface(t, h.posts)()
	instance, err := h.w.commandSurfaceState.current()
	if instance != nil || !errors.Is(err, want) {
		t.Fatal(instance, err)
	}
}

package terminalhost

import (
	"context"
	"github.com/bytedance/herdr-ui/internal/herdr"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

type fakeTerminal struct {
	mu     sync.Mutex
	closes int
	views  int
	sent   []byte
}

func (f *fakeTerminal) Close() error { f.mu.Lock(); defer f.mu.Unlock(); f.closes++; return nil }
func (f *fakeTerminal) Send(p []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, p...)
}
func (f *fakeTerminal) View(*ui.Context) ui.Element {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.views++
	return ui.Element{}
}

type harness struct {
	h       *Host
	options chan terminal.Options
	release chan struct{}
	posts   chan func()
	f       *fakeTerminal
}

func setup() *harness {
	x := &harness{options: make(chan terminal.Options, 20), release: make(chan struct{}, 20), posts: make(chan func(), 40), f: &fakeTerminal{}}
	x.h = newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/herdr.sock"}, Post: func(f func()) { x.posts <- f }}, func(o terminal.Options) (instance, error) { x.options <- o; <-x.release; return x.f, nil })
	return x
}
func receive[T any](t *testing.T, c <-chan T) T {
	t.Helper()
	select {
	case v := <-c:
		return v
	case <-time.After(time.Second):
		t.Fatal("timeout")
		var z T
		return z
	}
}

var demand = Spec{TargetID: 1, TerminalID: "terminal", PaneID: "pane"}

func reconcile(t *testing.T, x *harness) Binding {
	t.Helper()
	b, e := x.h.Reconcile([]Spec{demand})
	if e != nil {
		t.Fatal(e)
	}
	return b[0]
}
func finish(t *testing.T, x *harness) {
	t.Helper()
	ctx, c := context.WithTimeout(context.Background(), time.Second)
	defer c()
	if err := x.h.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestRetainAndValidate(t *testing.T) {
	x := setup()
	b := reconcile(t, x)
	receive(t, x.options)
	if reconcile(t, x) != b {
		t.Fatal("binding replaced")
	}
	if _, e := x.h.Reconcile([]Spec{demand, demand}); e == nil {
		t.Fatal("duplicate accepted")
	}
	x.release <- struct{}{}
	receive(t, x.posts)()
	if x.h.Status(b).State != Attached {
		t.Fatal(x.h.Status(b))
	}
	finish(t, x)
}
func TestCloseBeforePostedInstall(t *testing.T) {
	x := setup()
	b := reconcile(t, x)
	receive(t, x.options)
	x.release <- struct{}{}
	install := receive(t, x.posts)
	finish(t, x)
	install()
	if x.h.Status(b).State != Closed {
		t.Fatal("late install")
	}
	if x.f.closes != 1 {
		t.Fatal(x.f.closes)
	}
}
func TestLateNewAfterRemoval(t *testing.T) {
	x := setup()
	reconcile(t, x)
	receive(t, x.options)
	x.h.Reconcile(nil)
	x.release <- struct{}{}
	finish(t, x)
	if x.f.closes != 1 {
		t.Fatal(x.f.closes)
	}
}
func TestStaleCallbacksAndRetry(t *testing.T) {
	x := setup()
	b := reconcile(t, x)
	old := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	old.OnExit(9)
	receive(t, x.posts)()
	if s := x.h.Status(b); s.State != Failed || s.ExitCode != 9 {
		t.Fatal(s)
	}
	x.h.Retry(b)
	fresh := reconcile(t, x)
	if fresh.Generation == b.Generation {
		t.Fatal("retry retained generation")
	}
	receive(t, x.options)
	old.OnExit(2)
	old.OnTitle("old")
	x.release <- struct{}{}
	receive(t, x.posts)()
	if x.h.Status(fresh).State != Attached {
		t.Fatal(x.h.Status(fresh))
	}
	finish(t, x)
}
func TestKeyClearScroll(t *testing.T) {
	x := setup()
	var rows int
	x.h.cfg.OnScroll = func(_ Binding, r int) { rows = r }
	b := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	if !o.OnKey(ui.Ctrl, ui.KeyB) || o.OnKey(ui.Ctrl|ui.Shift, ui.KeyB) {
		t.Fatal("key consume")
	}
	if string(x.f.sent) != string([]byte{2, 2}) {
		t.Fatal(x.f.sent)
	}
	if !o.OnClear(terminal.Clear(0)) || x.h.Status(b).State != Attached || x.h.Status(b).Message == "" {
		t.Fatal("clear")
	}
	if !o.OnScroll(-3) || rows != -3 {
		t.Fatal("scroll")
	}
	finish(t, x)
}
func TestCloseExitRace(t *testing.T) {
	x := setup()
	reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			o.OnExit(i)
		}
		close(done)
	}()
	finish(t, x)
	<-done
	for len(x.posts) > 0 {
		(<-x.posts)()
	}
}

func TestRemovedReaddedDiscardsQueuedCallbacks(t *testing.T) {
	x := setup()
	oldBinding := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	o.OnExit(7)
	stale := receive(t, x.posts)
	if _, err := x.h.Reconcile(nil); err != nil {
		t.Fatal(err)
	}
	fresh := reconcile(t, x)
	receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	stale()
	if fresh.Generation == oldBinding.Generation || x.h.Status(fresh).State != Attached {
		t.Fatal("stale exit changed fresh terminal")
	}
	finish(t, x)
}
func TestCloseBudgetIncludesPendingNew(t *testing.T) {
	x := setup()
	reconcile(t, x)
	receive(t, x.options)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := x.h.Close(ctx); err != context.Canceled {
		t.Fatal(err)
	}
	x.release <- struct{}{}
	finish(t, x)
	if x.f.closes != 1 {
		t.Fatal(x.f.closes)
	}
	finish(t, x)
}
func TestPaneRemappingRecreatesBinding(t *testing.T) {
	x := setup()
	b := reconcile(t, x)
	receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	s := demand
	s.PaneID = "other"
	bindings, err := x.h.Reconcile([]Spec{s})
	if err != nil || bindings[0].Generation == b.Generation {
		t.Fatal(bindings, err)
	}
	receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	finish(t, x)
}
func TestOptionsUseArgvAndPluginPalette(t *testing.T) {
	x := setup()
	x.h.cfg.Env = []string{"HERDR_SOCKET_PATH=/wrong", "HERDR_PANE_ID=parent", "OTHER=kept"}
	reconcile(t, x)
	o := receive(t, x.options)
	if len(o.Command) != 4 || o.Command[0] != "/bin/herdr" || o.Command[3] != "terminal" {
		t.Fatal(o.Command)
	}
	if o.RightClick != terminal.RightClickCopyOrPaste || o.ClipboardRead != terminal.ClipboardDeny || o.Font.Size != 13 || o.Padding.X != 16 || o.Padding.Y != 12 {
		t.Fatal("wrong plugin defaults")
	}
	if o.Theme.Palette != terminal.LightTheme().Palette || o.DarkTheme.Palette != terminal.DarkTheme().Palette {
		t.Fatal("ANSI palettes changed")
	}
	x.release <- struct{}{}
	finish(t, x)
}

func TestInitializationFailureRequiresExplicitRetry(t *testing.T) {
	posts := make(chan func(), 10)
	attempts := make(chan struct{}, 10)
	h := newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/socket"}, Post: func(f func()) { posts <- f }}, func(terminal.Options) (instance, error) { attempts <- struct{}{}; return nil, context.DeadlineExceeded })
	bindings, err := h.Reconcile([]Spec{demand})
	if err != nil {
		t.Fatal(err)
	}
	receive(t, attempts)
	receive(t, posts)()
	b := bindings[0]
	if h.Status(b).State != Failed {
		t.Fatal(h.Status(b))
	}
	again, err := h.Reconcile([]Spec{demand})
	if err != nil || again[0] != b {
		t.Fatal(again, err)
	}
	h.Retry(b)
	again, err = h.Reconcile([]Spec{demand})
	if err != nil || again[0].Generation == b.Generation {
		t.Fatal(again, err)
	}
	receive(t, attempts)
	receive(t, posts)()
	if err := h.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestDuplicatePaneRejectedAtomically(t *testing.T) {
	x := setup()
	s := demand
	s.TerminalID = "other-terminal"
	if _, err := x.h.Reconcile([]Spec{demand, s}); err == nil {
		t.Fatal("duplicate pane accepted")
	}
	if len(x.options) != 0 {
		t.Fatal("invalid demand started attach")
	}
	finish(t, x)
}

func TestExitBeforeInstallRetainsDiagnosticScreen(t *testing.T) {
	x := setup()
	b := reconcile(t, x)
	o := receive(t, x.options)
	o.OnExit(1)
	receive(t, x.posts)()
	x.release <- struct{}{}
	receive(t, x.posts)()
	if x.h.Status(b).State != Failed {
		t.Fatal("installation overwrote early exit")
	}
	x.h.View(nil, b)
	if x.f.views != 1 {
		t.Fatal("failed screen hidden")
	}
	finish(t, x)
}

type blockingCloseTerminal struct {
	fakeTerminal
	started chan struct{}
	release chan struct{}
}

func (f *blockingCloseTerminal) Close() error {
	close(f.started)
	<-f.release
	return f.fakeTerminal.Close()
}
func assertNoFactory(t *testing.T, c <-chan terminal.Options) {
	t.Helper()
	select {
	case <-c:
		t.Fatal("replacement factory ran before predecessor cleanup")
	case <-time.After(40 * time.Millisecond):
	}
}
func TestReplacementWaitsForPreviousClose(t *testing.T) {
	calls := make(chan terminal.Options, 10)
	posts := make(chan func(), 10)
	old := &blockingCloseTerminal{started: make(chan struct{}), release: make(chan struct{})}
	fresh := &fakeTerminal{}
	n := 0
	h := newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/socket"}, Post: func(f func()) { posts <- f }}, func(o terminal.Options) (instance, error) {
		calls <- o
		n++
		if n == 1 {
			return old, nil
		}
		return fresh, nil
	})
	h.Reconcile([]Spec{demand})
	receive(t, calls)
	receive(t, posts)()
	h.Reconcile(nil)
	h.Reconcile([]Spec{demand})
	receive(t, old.started)
	assertNoFactory(t, calls)
	close(old.release)
	receive(t, calls)
	receive(t, posts)()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestSkippedGenerationPreservesDelayedNewBarrier(t *testing.T) {
	calls := make(chan terminal.Options, 10)
	posts := make(chan func(), 10)
	newRelease := make(chan struct{})
	old := &blockingCloseTerminal{started: make(chan struct{}), release: make(chan struct{})}
	fresh := &fakeTerminal{}
	n := 0
	h := newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/socket"}, Post: func(f func()) { posts <- f }}, func(o terminal.Options) (instance, error) {
		calls <- o
		n++
		if n == 1 {
			<-newRelease
			return old, nil
		}
		return fresh, nil
	})
	h.Reconcile([]Spec{demand})
	receive(t, calls)
	h.Reconcile(nil)
	h.Reconcile([]Spec{demand})
	h.Reconcile(nil)
	h.Reconcile([]Spec{demand})
	assertNoFactory(t, calls)
	close(newRelease)
	receive(t, old.started)
	assertNoFactory(t, calls)
	close(old.release)
	receive(t, calls)
	receive(t, posts)()
	if n != 2 {
		t.Fatal("retired intermediate generation opened", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownRetiresQueuedGenerations(t *testing.T) {
	calls := make(chan terminal.Options, 10)
	posts := make(chan func(), 10)
	newRelease := make(chan struct{})
	old := &blockingCloseTerminal{started: make(chan struct{}), release: make(chan struct{})}
	h := newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/socket"}, Post: func(f func()) { posts <- f }}, func(o terminal.Options) (instance, error) { calls <- o; <-newRelease; return old, nil })
	h.Reconcile([]Spec{demand})
	receive(t, calls)
	h.Reconcile(nil)
	h.Reconcile([]Spec{demand})
	h.Reconcile(nil)
	h.Reconcile([]Spec{demand})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.Close(ctx); err != context.Canceled {
		t.Fatal(err)
	}
	close(newRelease)
	receive(t, old.started)
	assertNoFactory(t, calls)
	close(old.release)
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 || len(posts) != 0 {
		t.Fatal("shutdown opened or posted a retired generation")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.tails) != 0 || h.tasks != 0 {
		t.Fatal("lifetime barriers leaked", len(h.tails), h.tasks)
	}
}

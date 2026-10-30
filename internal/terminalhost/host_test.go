package terminalhost

import (
	"context"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/egoist/mygo/transfer"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

type fakeTerminal struct {
	mu        sync.Mutex
	closes    int
	views     int
	sent      []byte
	searches  []string
	nexts     int
	prevs     int
	ended     int
	copies    int
	contrasts []float32
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
func (f *fakeTerminal) SetFont(terminal.Font) {}
func (f *fakeTerminal) SetCopyOnSelect(terminal.CopyOnSelect) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copies++
}
func (f *fakeTerminal) SetMinimumContrast(ratio float32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.contrasts = append(f.contrasts, ratio)
}
func (f *fakeTerminal) Search(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.searches = append(f.searches, text)
}
func (f *fakeTerminal) SearchNext() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nexts++
}
func (f *fakeTerminal) SearchPrevious() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prevs++
}
func (f *fakeTerminal) EndSearch() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ended++
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

func TestOptionsCarryKeyboardAndCursorSettings(t *testing.T) {
	x := setup()
	x.h.SetOptionAsAlt(true)
	x.h.SetCursor(terminal.CursorBar, true)
	reconcile(t, x)
	o := receive(t, x.options)
	if !o.OptionAsAlt || o.Cursor != terminal.CursorBar || !o.NoBlink {
		t.Fatalf("attach ignored settings: option as alt %v, cursor %v, no blink %v", o.OptionAsAlt, o.Cursor, o.NoBlink)
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

func TestInputHookBeforeLiteralAndStale(t *testing.T) {
	x := setup()
	calls := 0
	x.h.cfg.OnKey = func(b Binding, mods ui.Modifiers, k ui.Key, state InputState) KeyResult {
		calls++
		if state.Guarded {
			t.Fatal("fake without optional capability reported guarded")
		}
		// Querying from the callback must not deadlock the host.
		x.h.Status(b)
		if k == ui.KeyB {
			return KeyResult{Consumed: true, Send: []byte("hook")}
		}
		return KeyResult{}
	}
	b := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	if x.h.InputState(b).Guarded {
		t.Fatal("unexpected guard")
	}
	if !o.OnKey(ui.Ctrl, ui.KeyB) || string(x.f.sent) != "hook" {
		t.Fatal(x.f.sent)
	}
	x.h.cfg.OnKey = func(Binding, ui.Modifiers, ui.Key, InputState) KeyResult { return KeyResult{} }
	if !o.OnKey(ui.Ctrl, ui.KeyB) || string(x.f.sent) != "hook\x02\x02" {
		t.Fatal(x.f.sent)
	}
	finish(t, x)
	x.h.cfg.OnKey = func(Binding, ui.Modifiers, ui.Key, InputState) KeyResult { calls++; return KeyResult{} }
	o.OnKey(0, ui.KeyH)
	if calls != 1 {
		t.Fatal("stale hook called", calls)
	}
}

type composingTerminal struct {
	*fakeTerminal
	composing bool
}

func (f *composingTerminal) InputState() InputState {
	return InputState{Guarded: true, Composing: f.composing}
}

func TestCompositionCapabilityReachesHook(t *testing.T) {
	x := setup()
	native := &composingTerminal{fakeTerminal: x.f, composing: true}
	x.h.create = func(o terminal.Options) (instance, error) { x.options <- o; <-x.release; return native, nil }
	x.h.cfg.OnKey = func(b Binding, _ ui.Modifiers, _ ui.Key, state InputState) KeyResult {
		if state != (InputState{Composing: true, Guarded: true}) {
			t.Fatal(state)
		}
		return KeyResult{}
	}
	b := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	if state := x.h.InputState(b); !state.Guarded || !state.Composing {
		t.Fatal(state)
	}
	if o.OnKey(ui.Ctrl, ui.KeyB) {
		t.Fatal("composition used literal transport fallback")
	}
	if o.OnKey(0, ui.KeyV) {
		t.Fatal("composition key intercepted")
	}
	native.composing = false
	if state := x.h.InputState(b); !state.Guarded || state.Composing {
		t.Fatal(state)
	}
	finish(t, x)
	if state := x.h.InputState(b); state != (InputState{}) {
		t.Fatal("stale capability", state)
	}
}

func TestPluginCapabilityDoesNotRequireNativeLoad(t *testing.T) {
	p := &pluginInstance{terminal: &terminal.Terminal{}}
	_, supported := any(p.terminal).(interface{ Composing() bool })
	if state := p.InputState(); state.Guarded != supported || state.Composing {
		t.Fatal(state, supported)
	}
}

func TestRapidVisibleSetChangesRetainSharedPaneAndCloseEveryInstance(t *testing.T) {
	var mu sync.Mutex
	created := []*fakeTerminal{}
	posts := make(chan func(), 1024)
	h := newHost(Config{Binary: "/bin/herdr", Target: herdr.Target{Socket: "/tmp/m4-test.sock"}, Post: func(fn func()) { posts <- fn }}, func(terminal.Options) (instance, error) {
		term := &fakeTerminal{}
		mu.Lock()
		created = append(created, term)
		mu.Unlock()
		return term, nil
	})
	left := Spec{TargetID: 1, PaneID: "left", TerminalID: "left-terminal"}
	shared := Spec{TargetID: 1, PaneID: "shared", TerminalID: "shared-terminal"}
	bindings, err := h.Reconcile([]Spec{left, shared})
	if err != nil {
		t.Fatal(err)
	}
	retained := bindings[1]
	for h.Status(retained).State != Attached {
		receive(t, posts)()
	}
	for i := 0; i < 100; i++ {
		bindings, err = h.Reconcile([]Spec{shared})
		if err != nil || bindings[0] != retained {
			t.Fatal("shared visible binding replaced", err)
		}
		bindings, err = h.Reconcile([]Spec{left, shared})
		if err != nil || bindings[1] != retained {
			t.Fatal("restoring another pane replaced shared binding", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := h.Close(ctx); err != nil {
		t.Fatal(err)
	}
	// Posted installation/title work may arrive late; it must not revive owners.
	for {
		select {
		case fn := <-posts:
			fn()
		default:
			goto drained
		}
	}
drained:
	mu.Lock()
	defer mu.Unlock()
	for _, term := range created {
		term.mu.Lock()
		closes := term.closes
		term.mu.Unlock()
		if closes != 1 {
			t.Fatalf("instance closed %d times", closes)
		}
	}
	if h.Status(retained).State != Closed {
		t.Fatal("late UI work revived a closed binding")
	}
}

func attach(t *testing.T, x *harness) (Binding, terminal.Options) {
	t.Helper()
	b := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	return b, o
}

func TestSearchRoutesByBindingAndDropsStale(t *testing.T) {
	x := setup()
	b, _ := attach(t, x)
	x.h.SearchPane(b, "query")
	x.h.SearchNext(b)
	x.h.SearchPrevious(b)
	x.h.EndSearch(b)
	// A replaced binding must not address the replacement's terminal.
	x.h.SearchPane(Binding{b.TargetID, b.TerminalID, b.Generation + 1}, "stale")
	x.f.mu.Lock()
	searches, nexts, prevs, ended := len(x.f.searches), x.f.nexts, x.f.prevs, x.f.ended
	x.f.mu.Unlock()
	if searches != 1 || x.f.searches[0] != "query" || nexts != 1 || prevs != 1 || ended != 1 {
		t.Fatal(searches, nexts, prevs, ended)
	}
	finish(t, x)
	x.h.SearchPane(b, "late")
	x.f.mu.Lock()
	searches = len(x.f.searches)
	x.f.mu.Unlock()
	if searches != 1 {
		t.Fatal("closed host searched")
	}
}

func TestOnSearchPostedWithBindingAndDroppedWhenRetired(t *testing.T) {
	x := setup()
	var got []terminal.SearchStatus
	x.h.SetOnSearch(func(b Binding, s terminal.SearchStatus) {
		if b.TargetID != demand.TargetID || b.TerminalID != demand.TerminalID {
			t.Fatal("search reported for a foreign binding")
		}
		got = append(got, s)
	})
	b, o := attach(t, x)
	o.OnSearch(terminal.SearchStatus{Text: "q", Total: 3, Selected: 1})
	receive(t, x.posts)()
	o.OnSearch(terminal.SearchStatus{})
	receive(t, x.posts)()
	if len(got) != 2 || got[0].Total != 3 || got[1] != (terminal.SearchStatus{}) {
		t.Fatal(got)
	}
	x.h.Reconcile(nil)
	o.OnSearch(terminal.SearchStatus{Text: "retired"})
	select {
	case fn := <-x.posts:
		fn()
		t.Fatal("retired generation posted a search report")
	default:
	}
	finish(t, x)
	_ = b
}

func TestNativePasteHasApplicationHandler(t *testing.T) {
	x := setup()
	calls := 0
	var gotBinding Binding
	x.h.cfg.OnPaste = func(binding Binding, data transfer.Data) bool {
		calls++
		gotBinding = binding
		image, _ := data.Read(transfer.PNG)
		return len(image) > 0
	}
	binding := reconcile(t, x)
	o := receive(t, x.options)
	x.release <- struct{}{}
	receive(t, x.posts)()
	defer finish(t, x)
	if o.OnPaste == nil {
		t.Fatal("native paste bypasses application image upload")
	}
	data := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte("png"))))
	if !o.OnPaste(data) || calls != 1 || gotBinding != binding {
		t.Fatal("image not routed to owning binding")
	}
	if o.OnPaste(transfer.TextData("text")) || calls != 2 {
		t.Fatal("text fallback lost")
	}
	if _, err := x.h.Reconcile(nil); err != nil {
		t.Fatal(err)
	}
	if !o.OnPaste(data) || calls != 2 {
		t.Fatal("stale terminal dispatched paste")
	}
}

type themedTerminal struct {
	*fakeTerminal
	themes [][2]*terminal.Theme
	onSet  func()
}

func (f *themedTerminal) SetTheme(light, dark *terminal.Theme) {
	if f.onSet != nil {
		f.onSet()
	}
	f.themes = append(f.themes, [2]*terminal.Theme{light, dark})
}

func TestHostSetTheme(t *testing.T) {
	light, dark := terminal.LightTheme(), terminal.DarkTheme()
	updatedLight, updatedDark := terminal.LightTheme(), terminal.DarkTheme()
	updatedLight.Background, updatedDark.Background = ui.RGB(235, 220, 205), ui.RGB(20, 30, 40)
	t.Run("running and future attaches", func(t *testing.T) {
		x := setup()
		first := &themedTerminal{fakeTerminal: x.f}
		second := &themedTerminal{fakeTerminal: &fakeTerminal{}}
		x.h.cfg.Theme, x.h.cfg.DarkTheme = light, dark
		x.h.create = func(o terminal.Options) (instance, error) {
			x.options <- o
			<-x.release
			if o.Command[len(o.Command)-1] == demand.TerminalID {
				return first, nil
			}
			return second, nil
		}
		b := reconcile(t, x)
		first.onSet = func() { x.h.Status(b) } // Setters must run outside the host lock.
		o := receive(t, x.options)
		if o.Theme != light || o.DarkTheme != dark {
			t.Fatal("initial attach ignored configured theme")
		}
		x.release <- struct{}{}
		receive(t, x.posts)()
		x.h.SetTheme(updatedLight, updatedDark)
		if got := first.themes[len(first.themes)-1]; got != ([2]*terminal.Theme{updatedLight, updatedDark}) {
			t.Fatal("running terminal retained old theme", got)
		}
		next := Spec{TargetID: 1, TerminalID: "next", PaneID: "next-pane"}
		if _, err := x.h.Reconcile([]Spec{demand, next}); err != nil {
			t.Fatal(err)
		}
		o = receive(t, x.options)
		if o.Theme != updatedLight || o.DarkTheme != updatedDark {
			t.Fatal("future attach ignored updated theme")
		}
		x.release <- struct{}{}
		receive(t, x.posts)()
		if got := second.themes[len(second.themes)-1]; got != ([2]*terminal.Theme{updatedLight, updatedDark}) {
			t.Fatal("future terminal installed wrong theme", got)
		}
		x.h.SetTheme(nil, nil)
		defaultLight, defaultDark := terminalThemes()
		for _, f := range []*themedTerminal{first, second} {
			got := f.themes[len(f.themes)-1]
			if *got[0] != *defaultLight || *got[1] != *defaultDark {
				t.Fatal("clearing theme did not restore defaults")
			}
		}
		finish(t, x)
	})
	for _, queued := range []bool{false, true} {
		name := "opening"
		if queued {
			name = "queued installation"
		}
		t.Run(name, func(t *testing.T) {
			x := setup()
			native := &themedTerminal{fakeTerminal: x.f}
			x.h.create = func(o terminal.Options) (instance, error) {
				x.options <- o
				<-x.release
				return native, nil
			}
			b := reconcile(t, x)
			native.onSet = func() { x.h.Status(b) }
			receive(t, x.options)
			var install func()
			if queued {
				x.release <- struct{}{}
				install = receive(t, x.posts)
			}
			x.h.SetTheme(updatedLight, updatedDark)
			if len(native.themes) != 0 {
				t.Fatal("theme applied before installation")
			}
			if !queued {
				x.release <- struct{}{}
				install = receive(t, x.posts)
			}
			install()
			if len(native.themes) != 1 || native.themes[0] != ([2]*terminal.Theme{updatedLight, updatedDark}) {
				t.Fatal("installation did not apply latest theme", native.themes)
			}
			finish(t, x)
		})
	}
}

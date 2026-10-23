package ui

import (
	"errors"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/input"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/bytedance/herdr-ui/internal/terminalhost"
	native "github.com/egoist/mygo/ui"
	"testing"
)

func keyWindow(t *testing.T) (*Window, terminalhost.Binding) {
	t.Helper()
	b, err := input.Resolve(nil, "test defaults")
	if err != nil {
		t.Fatal(err)
	}
	w := New(app.New(), nil)
	w.ConfigureBindings(b, nil)
	binding := terminalhost.Binding{TargetID: 1, TerminalID: "terminal", Generation: 1}
	child := &terminalPanel{binding: binding, spec: terminalhost.Spec{TargetID: 1, PaneID: "pane", TerminalID: "terminal"}}
	w.terminal = &terminalPanel{panes: map[string]*terminalPanel{"pane": child}}
	return w, binding
}
func TestNativePrefixCompositionAndLiteralTransport(t *testing.T) {
	w, b := keyWindow(t)
	guarded := terminalhost.InputState{Guarded: true}
	if r := w.keyPublished(b, native.Ctrl, native.KeyB, guarded, keyPublication(t, "terminal")); !r.Consumed || !w.controller.machine.Awaiting {
		t.Fatal(r)
	}
	if r := w.keyPublished(b, native.Ctrl, native.KeyB, guarded, keyPublication(t, "terminal")); !r.Consumed || string(r.Send) != "\x02\x02" {
		t.Fatalf("literal: %+v", r)
	}
	w.keyPublished(b, native.Ctrl, native.KeyB, guarded, keyPublication(t, "terminal"))
	r := w.keyPublished(b, 0, native.KeyV, terminalhost.InputState{Guarded: true, Composing: true}, keyPublication(t, "terminal"))
	if r.Consumed || w.controller.machine.Awaiting {
		t.Fatal("composition intercepted or mode retained")
	}
	w.keyPublished(b, native.Ctrl, native.KeyB, guarded, keyPublication(t, "terminal"))
	w.BlurInput()
	if w.controller.machine.Awaiting {
		t.Fatal("blur retained prefix")
	}
}
func TestUnavailableCompositionOrMalformedConfigNeverDispatchesPrefix(t *testing.T) {
	w, b := keyWindow(t)
	if r := w.keyPublished(b, native.Ctrl, native.KeyB, terminalhost.InputState{}, keyPublication(t, "terminal")); r.Consumed || w.controller.machine.Awaiting {
		t.Fatal(r)
	}
	w.ConfigureBindings(input.Bindings{}, errors.New("unsupported keys.navigate"))
	if r := w.keyPublished(b, native.Ctrl, native.KeyB, terminalhost.InputState{Guarded: true}, keyPublication(t, "terminal")); r.Consumed {
		t.Fatal(r)
	}
	w.Dispatch("toggle_sidebar")
	if !w.hidden {
		t.Fatal("config failure disabled local toolbar action")
	}
}

func TestToolbarActionsRestoreSamePaneFocusAndHelpClosesWithoutTerminal(t *testing.T) {
	w, _ := keyWindow(t)
	w.terminal.focused = "pane"
	w.Dispatch("toggle_sidebar")
	if w.terminal.focused != "" {
		t.Fatal("toolbar retained keyboard focus")
	}
	w.terminal.focused = "pane"
	w.Dispatch("help")
	if w.terminal.focused != "pane" {
		t.Fatal("opening reference forced terminal focus")
	}
	w.Dispatch("help")
	if w.terminal.focused != "" {
		t.Fatal("closing reference did not restore terminal focus")
	}
	plain := New(app.New(), nil)
	bindings, _ := input.Resolve(nil, "test")
	plain.ConfigureBindings(bindings, nil)
	plain.Dispatch("help")
	tester := native.NewTester(plain.View, 820, 560)
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
}

func keyPublication(t *testing.T, terminal string) session.PublishedSnapshot {
	t.Helper()
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"pane","workspace_id":"w","tab_id":"t","terminal_id":"` + terminal + `"}],"agents":[],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	return session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Snapshot: snapshot}
}
func TestOldTerminalKeyCannotDispatchAfterAuthoritativeReplacement(t *testing.T) {
	w, b := keyWindow(t)
	w.controller.machine.Awaiting = true
	result := w.keyPublished(b, 0, native.KeyB, terminalhost.InputState{Guarded: true}, keyPublication(t, "replacement"))
	if result.Consumed || w.hidden || w.controller.machine.Awaiting {
		t.Fatal("old terminal dispatched sidebar action or retained prefix")
	}
	// With no authoritative mapping, the public entry point must reject too.
	result = w.Key(b, native.Ctrl, native.KeyB, terminalhost.InputState{Guarded: true})
	if result.Consumed {
		t.Fatal("accepted key without latest session mapping")
	}
}
func TestRatioCompletionOnlyCancelsItsOwnCurrentDrag(t *testing.T) {
	w, _ := keyWindow(t)
	old := &ratioDrag{}
	fresh := &ratioDrag{}
	w.controller.drag = fresh
	w.controller.ratioGeneration = 2
	failed := session.OperationResult{Status: session.Rejected, Error: "late failure"}
	w.completeRatio(failed, old, true, 1)
	if w.controller.drag != fresh || w.controller.message != "" {
		t.Fatal("old drag completion affected new drag")
	}
	w.completeRatio(failed, nil, true, 1)
	if w.controller.drag != fresh {
		t.Fatal("keyboard resize completion canceled drag")
	}
	w.controller.message = ""
	w.completeRatio(failed, fresh, false, 2)
	if w.controller.drag != fresh || w.controller.message != "" {
		t.Fatal("old view completion affected current view")
	}
	w.completeRatio(failed, fresh, true, 2)
	if w.controller.drag != nil || w.controller.message == "" {
		t.Fatal("current drag failure did not roll back")
	}
}

func TestFinalRatioFailureVisibleUntilNewerResizeGeneration(t *testing.T) {
	w, _ := keyWindow(t)
	ended := &ratioDrag{}
	w.controller.ratioGeneration = 3
	failed := session.OperationResult{Status: session.OutcomeUnknown, Error: "final response lost"}
	// Releasing clears the preview before the asynchronous result arrives.
	w.completeRatio(failed, ended, true, 3)
	if w.controller.message == "" {
		t.Fatal("final-release failure was hidden")
	}
	w.controller.message = ""
	w.controller.ratioGeneration = 4
	w.completeRatio(failed, ended, true, 3)
	if w.controller.message != "" {
		t.Fatal("previous ended drag overwrote newer ended drag result")
	}
}

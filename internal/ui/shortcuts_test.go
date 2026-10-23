package ui

import (
	"strings"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func navigationWindow(t *testing.T) *Window {
	t.Helper()
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"a","label":"Alpha"},{"workspace_id":"b","label":"Beta"}],"tabs":[{"workspace_id":"a","tab_id":"t1"},{"workspace_id":"a","tab_id":"t2"},{"workspace_id":"b","tab_id":"t3"}],"panes":[{"workspace_id":"a","tab_id":"t1","pane_id":"p1","terminal_id":"term1"},{"workspace_id":"a","tab_id":"t1","pane_id":"p2","terminal_id":"term2"},{"workspace_id":"a","tab_id":"t2","pane_id":"p3","terminal_id":"term3"},{"workspace_id":"b","tab_id":"t3","pane_id":"p4","terminal_id":"term4"}],"layouts":[],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	model := app.New()
	model.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: snapshot, TargetID: 1, Epoch: 1})
	model.SelectPane("p1")
	w := New(model, nil)
	b, err := input.Resolve(nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	w.ConfigureBindings(b, nil)
	return w
}
func TestIndexedNavigationAndLastPane(t *testing.T) {
	w := navigationWindow(t)
	w.DispatchIndex("switch_tab", 1)
	if w.currentView().PaneID != "p3" {
		t.Fatal(w.currentView())
	}
	w.Dispatch("last_pane")
	if w.currentView().PaneID != "p1" {
		t.Fatal(w.currentView())
	}
	w.Dispatch("cycle_pane_next")
	if w.currentView().PaneID != "p2" {
		t.Fatal(w.currentView())
	}
	w.DispatchIndex("switch_workspace", 1)
	if w.currentView().WorkspaceID != "b" {
		t.Fatal(w.currentView())
	}
	w.DispatchIndex("switch_workspace", 8)
	if w.currentView().WorkspaceID != "b" {
		t.Fatal("out of range changed selection")
	}
}
func TestNavigationPreviewCancelAndImplicitBindings(t *testing.T) {
	w := navigationWindow(t)
	w.Dispatch("workspace_picker")
	if !w.navigateKey("down") || w.controller.shortcut.navigateWorkspace != "b" || w.currentView().WorkspaceID != "a" {
		t.Fatal("navigation did not preview")
	}
	w.navigateKey("esc")
	if w.currentView().WorkspaceID != "a" {
		t.Fatal("cancel committed preview")
	}
	w.Dispatch("workspace_picker")
	w.navigateKey("down")
	w.navigateKey("enter")
	if w.currentView().WorkspaceID != "b" {
		t.Fatal("enter did not commit")
	}
	w.Dispatch("workspace_picker")
	if !w.navigateKey("f12") {
		t.Fatal("unknown navigate key leaked")
	}
	// p is normally a prefix action. In navigation it must be consumed as one.
	w.DispatchIndex("switch_workspace", 0)
	w.DispatchIndex("switch_tab", 1)
	w.Dispatch("workspace_picker")
	w.navigateKey("p")
	if w.currentView().TabID != "t1" {
		t.Fatal("implicit prefix action did not run")
	}
}
func TestAllConfiguredActionsHaveDispatchEntry(t *testing.T) {
	bindings, err := input.Resolve(nil, "defaults")
	if err != nil {
		t.Fatal(err)
	}
	catalog := map[string]bool{}
	for _, a := range Actions {
		catalog[a.ID] = true
	}
	for key, action := range bindings.Actions {
		if !catalog[action] {
			t.Fatalf("%s resolves %s with no dispatcher entry", key, action)
		}
	}
	for _, action := range Actions {
		if strings.HasPrefix(action.ID, "resize_pane_") && !extendedAction(action.ID) {
			t.Fatal("direct resize routed to mode resize")
		}
	}
}
func TestCopySelectionMotionsAndSearch(t *testing.T) {
	s := newCopyState("alpha beta\n你好 world\n\nlast line")
	s.key("g")
	s.key("v")
	s.key("e")
	if s.selectedText() != "alpha" {
		t.Fatal(s.selectedText())
	}
	s.key("esc")
	s.key("j")
	s.key("0")
	s.key("v")
	s.key("l")
	if s.selectedText() != "你好" {
		t.Fatal(s.selectedText())
	}
	s.key("esc")
	s.query = "line"
	s.search(false)
	if s.row != 3 || s.col != 5 {
		t.Fatal(s)
	}
	s.key("shift+v")
	if s.selectedText() != "last line" {
		t.Fatal(s.selectedText())
	}
	exit, copy := s.key("y")
	if !exit || !copy {
		t.Fatal("copy did not exit")
	}
	s.key("esc")
	if exit, _ := s.key("esc"); !exit {
		t.Fatal("second escape did not exit")
	}
}
func TestCopyPanelCapturesKeysAndShowsCursor(t *testing.T) {
	w := navigationWindow(t)
	w.controller.shortcut.copy = newCopyState("alpha beta\nsecond")
	tester := native.NewTester(w.View, 1000, 800)
	if !tester.HasText("Copy mode · line 2/2 · v select · y copy · / search · q close") {
		t.Fatal(tester.Texts())
	}
}

func TestEmptyWindowRetainsPrefixBindings(t *testing.T) {
	w := New(app.New(), nil)
	bindings, _ := input.Resolve(nil, "defaults")
	w.ConfigureBindings(bindings, nil)
	opened := false
	w.SetActionHooks(ActionHooks{Settings: func() error { opened = true; return nil }})
	tester := native.NewTester(w.View, 1000, 800)
	tester.Key(native.Ctrl, native.KeyB)
	tester.Key(0, native.KeyS)
	if !opened {
		t.Fatal("empty-window prefix could not open settings")
	}
}
func TestCopySearchCanCopyCurrentSmartCaseMatch(t *testing.T) {
	s := newCopyState("Alpha beta\nlast")
	s.query = "alpha"
	s.search(false)
	exit, copy := s.key("y")
	if !exit || !copy || s.selectedText() != "Alpha" {
		t.Fatal(exit, copy, s.selectedText())
	}
}
func TestImplicitGroupCloseRequiresNewConfirmation(t *testing.T) {
	w := navigationWindow(t)
	op := session.Operation{Kind: session.ClosePane, WorkspaceID: "a", PaneID: "p1"}
	if w.offerGroupClose(op, session.OperationResult{Status: session.OutcomeUnknown, Error: "confirmation_required"}) {
		t.Fatal("uncertain close offered retry")
	}
	if !w.offerGroupClose(op, session.OperationResult{Status: session.Rejected, Error: "Herdr confirmation_required: closing this pane would close a worktree group"}) || w.controller.shortcut.prompt == nil || !w.controller.shortcut.prompt.confirm {
		t.Fatal("group confirmation missing")
	}
}

func TestPromptEnterAndNavigatorEnter(t *testing.T) {
	w := navigationWindow(t)
	submitted := ""
	w.controller.shortcut.prompt = &shortcutPrompt{title: "Rename", value: "new name", focus: true, submit: func(value string) { submitted = value }}
	tester := native.NewTester(w.View, 1000, 800)
	tester.Key(0, native.KeyEnter)
	if submitted != "new name" || w.controller.shortcut.prompt != nil {
		t.Fatal("Enter did not submit text prompt", submitted)
	}
	w.Dispatch("goto")
	w.controller.shortcut.navigatorQuery = "Beta"
	tester = native.NewTester(w.View, 1000, 800)
	tester.Key(0, native.KeyEnter)
	if w.currentView().WorkspaceID != "b" || w.controller.shortcut.navigator {
		t.Fatal("navigator Enter did not select match")
	}
}

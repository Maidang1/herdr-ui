package ui

import (
	"slices"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestInsertIndexCountsBeforeTheMove(t *testing.T) {
	// Moving down lands after the target; moving up lands before it, as
	// move_tab_next's index+2 and move_tab_previous's index-1 do.
	if insertIndex(0, 2) != 3 || insertIndex(2, 0) != 0 || insertIndex(1, 2) != 3 || insertIndex(3, 1) != 1 {
		t.Fatal("insert index")
	}
}

func TestAgentViewFiltersSidebarAgents(t *testing.T) {
	w := New(app.New(), nil)
	agents := []app.AgentRow{
		{WorkspaceID: "a", PaneID: "1", Status: "working"},
		{WorkspaceID: "a", PaneID: "2", Status: "blocked"},
		{WorkspaceID: "b", PaneID: "3", Status: "done"},
	}
	count := func(view string) int {
		w.agentView = view
		return len(w.filterAgentView(agents, "a"))
	}
	if count("all") != 3 || count("attention") != 2 || count("working") != 1 || count("workspace") != 2 {
		t.Fatal("agent view filter")
	}
	var shared string
	w.SetExtraHooks(ExtraHooks{AgentView: func(id string, done func(error)) { shared = id; done(nil) }})
	w.chooseAgentView("working")
	if shared != "working" || w.agentView != "working" {
		t.Fatalf("shared %q", shared)
	}
	w.chooseAgentView("bogus")
	if w.agentView != "all" {
		t.Fatal("unknown view not reset to all")
	}
}

func TestHerdrMissingOffersInstallAndPlainTerminal(t *testing.T) {
	a := app.New()
	a.Update(session.PublishedSnapshot{State: session.Incompatible, Stale: true, Error: herdr.ErrNotInstalled.Error()})
	w := New(a, func() {})
	opened := 0
	var mode string
	w.SetExtraHooks(ExtraHooks{PlainTerminal: func() error { opened++; return nil }, StartupMode: func(m string) { mode = m }})
	tester := native.NewTester(w.View, 1200, 800)
	tester.Frame()
	if !tester.HasText("Herdr is not installed") || !tester.HasText("Installation Guide") {
		t.Fatal("missing install guidance")
	}
	if _, ok := tester.Find("Retry connection"); ok {
		t.Fatal("connection error bar shown for a missing installation")
	}
	if err := tester.Click("Open Plain Terminal"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if opened != 1 {
		t.Fatal("plain terminal not opened")
	}
	if err := tester.Click("Open as a plain terminal next time"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if mode != "plain" {
		t.Fatalf("startup mode %q", mode)
	}
}

func TestExtraActionsAreRegisteredAndGated(t *testing.T) {
	w := populatedChromeWindow(t)
	w.SetExtraHooks(ExtraHooks{AgentPrompt: func(session.Operation, string, func(error)) {}})
	for _, action := range []string{"agent_prompt", "agent_wait", "agent_start", "move_pane_new_tab", "move_pane_new_workspace", "save_layout", "apply_layout", "move_workspace_up"} {
		if !slices.ContainsFunc(Actions, func(a Action) bool { return a.ID == action }) {
			t.Fatalf("action %s not registered", action)
		}
	}
	if w.Can("apply_layout") {
		t.Fatal("apply layout enabled without saved layouts")
	}
	w.SetExtraHooks(ExtraHooks{ApplyLayout: func(session.Operation, string, func(error)) {}, LayoutNames: func() []string { return []string{"Review"} }})
	if !w.Can("apply_layout") {
		t.Fatal("apply layout disabled with a saved layout")
	}
}

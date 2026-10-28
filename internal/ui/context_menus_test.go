package ui

import (
	"runtime"
	"slices"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestWorkspaceContextMenuCopiesPathAndOpensRow(t *testing.T) {
	w := workspaceSidebarFixture(t)
	var copied string
	w.SetActionHooks(ActionHooks{WriteClipboard: func(s string) error { copied = s; return nil }})
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if err := tester.RightClick("Repo"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	menu := tester.Menu()
	for _, item := range []string{"Open", "New Tab", "Rename Workspace…", "New Worktree…", "Copy Path", revealPathLabel(runtime.GOOS), "Close Workspace…"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("workspace menu lacks %q: %v", item, menu)
		}
	}
	if err := tester.ChooseMenuItem("Copy Path"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if copied != "/repo" {
		t.Fatalf("Copy Path wrote %q", copied)
	}
	if err := tester.RightClick("Shell only"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if slices.Contains(tester.Menu(), "New Worktree…") {
		t.Fatal("worktree commands offered for a directory outside Git")
	}
	if err := tester.ChooseMenuItem("Open"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if w.currentView().WorkspaceID != "other" {
		t.Fatal("Open did not select the row's workspace")
	}
}

func TestTabContextMenuOffersTabCommands(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	if err := tester.RightClick("Tab Server"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	menu := tester.Menu()
	for _, item := range []string{"Rename Tab…", "Move Tab Left", "Move Tab Right", "Split Right", "Close Tab"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("tab menu lacks %q: %v", item, menu)
		}
	}
}

func TestAttentionButtonJumpsToWaitingAgent(t *testing.T) {
	snap, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Repo","agent_status":"blocked"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"shell","terminal_id":"tshell","workspace_id":"w","tab_id":"t","title":"shell"},{"pane_id":"agent","terminal_id":"tagent","workspace_id":"w","tab_id":"t","title":"prompt","agent":"claude","agent_status":"blocked"}],"layouts":[],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{TargetID: 1, State: session.Ready, Snapshot: snap})
	w := New(a, nil)
	w.ConfigureBindings(referenceWindow(t).controller.machine.Bindings, nil)
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if !tester.HasText("1 waiting") {
		t.Fatalf("waiting agent not surfaced: %v", tester.Texts())
	}
	if err := tester.Click("1 waiting"); err != nil {
		t.Fatal(err)
	}
	if w.currentView().PaneID != "agent" {
		t.Fatal("attention button did not select the waiting agent")
	}
}

func TestKeyGlyphsUseMacSymbols(t *testing.T) {
	for in, want := range map[string]string{"ctrl+b → shift+x": "⌃B  ⇧X", "ctrl+b → [": "⌃B  [", "ctrl+b → ctrl+left": "⌃B  ⌃←", "": ""} {
		if got := keyGlyphs(in); got != want {
			t.Errorf("keyGlyphs(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMenuBarCarriesEveryMenuCommandOnce(t *testing.T) {
	seen := map[string]int{}
	for _, section := range MenuBar {
		for _, group := range section.Groups {
			for _, id := range group {
				seen[id]++
			}
		}
	}
	keyboardOnly := map[string]bool{"switch_tab": true, "switch_workspace": true, "focus_agent": true, "settings": true, "help": true}
	for _, action := range Actions {
		if keyboardOnly[action.ID] {
			continue
		}
		if seen[action.ID] != 1 {
			t.Errorf("%s appears %d times in the menu bar", action.ID, seen[action.ID])
		}
	}
	for id := range seen {
		known := false
		for _, action := range Actions {
			known = known || action.ID == id
		}
		if !known {
			t.Errorf("menu bar names unknown action %s", id)
		}
	}
	w := referenceWindow(t)
	if got := w.MenuTitle("attention"); got != "Jump to Agent Needing You" {
		t.Errorf("title case: %q", got)
	}
	if got := w.MenuTitle("rename_tab"); got != "Rename Tab…" {
		t.Errorf("prompting command lacks its ellipsis: %q", got)
	}
}

func TestRevealPathLabelMatchesPlatform(t *testing.T) {
	for platform, want := range map[string]string{"darwin": "Reveal in Finder", "windows": "Reveal in File Manager", "linux": "Reveal in File Manager"} {
		if got := revealPathLabel(platform); got != want {
			t.Fatalf("%s: %s", platform, got)
		}
	}
}

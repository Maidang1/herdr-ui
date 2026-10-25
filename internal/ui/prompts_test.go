package ui

import (
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	native "github.com/egoist/mygo/ui"
)

func TestPromptsNameWhatTheyActOn(t *testing.T) {
	v := app.View{WorkspaceName: "herdr-ui", TabID: "t", Tabs: []app.TabRow{{ID: "t", Name: "Dev"}}, Panes: []app.PaneRow{{ID: "a"}, {ID: "b"}}}
	closeTab := promptFor("close_tab", v)
	if closeTab.title != "Close tab “Dev”?" || closeTab.message != "Its 2 panes close, and the programs running in them end." || closeTab.action != "Close Tab" || !closeTab.destructive || !closeTab.confirm {
		t.Fatalf("close tab = %+v", closeTab)
	}
	if got := promptFor("close_tab", app.View{}).title; got != "Close this tab?" {
		t.Fatalf("unnamed tab = %q", got)
	}
	if rename := promptFor("rename_tab", v); rename.value != "Dev" || rename.confirm || rename.action != "Rename" {
		t.Fatalf("rename tab = %+v", rename)
	}
}

func TestRequiredPromptWaitsForText(t *testing.T) {
	w := referenceWindow(t)
	tester := native.NewTester(w.View, 820, 560)
	var got []string
	prompt := promptFor("new_worktree", app.View{WorkspaceName: "herdr-ui"})
	prompt.focus, prompt.submit = true, func(text string) { got = append(got, text) }
	w.controller.shortcut.prompt = &prompt
	tester.Frame()
	if err := tester.Click("Create"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || w.controller.shortcut.prompt == nil {
		t.Fatal("an empty branch name was submitted")
	}
	tester.Type("feat/x")
	if err := tester.Click("Create"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "feat/x" {
		t.Fatalf("submitted %q", got)
	}
}

func TestReferenceListsEveryActionOnce(t *testing.T) {
	seen := map[string]int{}
	for _, group := range referenceGroups() {
		for _, id := range group.actions {
			seen[id]++
		}
	}
	for _, action := range Actions {
		if seen[action.ID] != 1 {
			t.Errorf("%s listed %d times", action.ID, seen[action.ID])
		}
	}
}

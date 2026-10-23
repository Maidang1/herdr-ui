package ui

import (
	"testing"

	native "github.com/egoist/mygo/ui"
)

func TestTabBarListsWorkspaceTabsAndSelectsThem(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	for _, label := range []string{"Tab bar", "Tab Development with a deliberately long name", "Tab Server", "New tab", "Close tab Server"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing tab bar control %q", label)
		}
	}
	if err := tester.Click("Tab Server"); err != nil {
		t.Fatal(err)
	}
	if w.app.View().TabID != "other" {
		t.Fatal("tab pill click did not switch tabs")
	}
	if w.app.View().Tabs[0].Selected {
		t.Fatal("previous tab remains selected after switching")
	}
}

func TestTabBarHidesWithoutTabs(t *testing.T) {
	w := referenceWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	if _, ok := tester.Find("Tab bar"); ok {
		t.Fatal("tab bar rendered without a session")
	}
}

func TestTabBarActionsMenuListsTabCommands(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	if err := tester.Click("Tab actions"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, label := range []string{"New tab", "Rename tab", "Split right", "Split down", "Close tab"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("tab actions menu missing %q", label)
		}
	}
	tester.ClickAt(10, 10)
	tester.Frame()
	if _, ok := tester.Find("Rename tab"); ok {
		t.Fatal("tab actions menu stayed open after clicking outside")
	}
}

func TestTabBarShowsWorkspaceContextWithoutSidebar(t *testing.T) {
	w := populatedChromeWindow(t)
	w.hidden = true
	tester := native.NewTester(w.View, 1200, 800)
	if _, ok := tester.Find("Very long workspace name for narrow headers"); !ok {
		t.Fatal("tab bar lost the workspace context without the sidebar")
	}
}

func TestTabBarShowsPrefixKeyTagWhileChordAwaited(t *testing.T) {
	w := populatedChromeWindow(t)
	w.controller.machine.Handle(w.controller.machine.Bindings.Prefix, false)
	if !w.controller.machine.Awaiting {
		t.Fatal("prefix chord did not engage")
	}
	tester := native.NewTester(w.View, 1200, 800)
	if _, ok := tester.Find("Ctrl + B"); !ok {
		t.Fatalf("prefix key tag missing: %v", tester.Texts())
	}
	if tester.HasText("Prefix") && !tester.HasText("Prefix Ctrl + B") {
		t.Fatal("plain Prefix hint returned")
	}
}

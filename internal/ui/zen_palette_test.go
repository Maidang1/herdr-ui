package ui

import (
	"testing"

	native "github.com/egoist/mygo/ui"
)

func TestZenHidesChromeAndRestoresIt(t *testing.T) {
	w := populatedChromeWindow(t)
	w.Dispatch("zen")
	tester := native.NewTester(w.View, 1200, 800)
	if _, ok := tester.Find("Workspace sidebar"); ok {
		t.Fatal("zen left the sidebar visible")
	}
	if _, ok := tester.Find("Tab bar"); ok {
		t.Fatal("zen left the tab strip visible")
	}
	w.Dispatch("zen")
	tester.Frame()
	if _, ok := tester.Find("Workspace sidebar"); !ok {
		t.Fatal("leaving zen did not restore the sidebar")
	}
}

func TestNavigatorListsCommandsAndDestinationsAndDispatches(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	// Destination half: the workspace row is reachable through the palette.
	w.Dispatch("goto")
	tester.Frame()
	if !tester.HasText("Go to anything, or run a command") {
		t.Fatalf("palette header missing: %v", tester.Texts())
	}
	if !tester.HasText("Very long workspace name for narrow headers") {
		t.Fatal("palette lost session destinations")
	}
	w.controller.shortcut.navigator = false
	w.BlurInput()
	// Command half: typing finds an action and choosing it dispatches.
	w.Dispatch("goto")
	tester.Frame()
	tester.Type("zen")
	tester.Frame()
	if !tester.HasText("Toggle zen mode") {
		t.Fatalf("palette did not list the command: %v", tester.Texts())
	}
	if err := tester.Click("Toggle zen mode"); err != nil {
		t.Fatal(err)
	}
	if !w.zen {
		t.Fatal("palette command did not dispatch")
	}
	// The chosen command climbs the next ranking.
	w.Dispatch("zen")
	entries := w.navigatorEntries("")
	found := false
	for _, entry := range entries[:min(3, len(entries))] {
		if entry.action == "zen" {
			found = true
		}
	}
	if !found {
		t.Fatal("chosen command did not rank at the top of the next palette")
	}
}

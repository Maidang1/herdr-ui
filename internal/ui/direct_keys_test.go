package ui

import (
	"testing"

	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestSelectTabNumberCountsFromOneAndNineIsLast(t *testing.T) {
	w := populatedChromeWindow(t)
	for _, step := range []struct {
		n    int
		want string
	}{{2, "other"}, {1, "t"}, {9, "other"}, {5, "other"}} {
		w.SelectTabNumber(step.n)
		if got := w.currentView().TabID; got != step.want {
			t.Fatalf("⌘%d selected %q, want %q", step.n, got, step.want)
		}
	}
}

func TestClosePaneKeyFindsTheFocusedAgent(t *testing.T) {
	w := referenceWindow(t)
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Work","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t","number":1,"label":"Dev"}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","title":"Shell"}],"agents":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","agent":"claude","agent_status":"working"}],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w.app.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: snapshot})
	if got := focusedAgent(w.currentView()); got != "claude" {
		t.Fatalf("focused agent = %q", got)
	}
	if got := focusedAgent(populatedChromeWindow(t).currentView()); got != "" {
		t.Fatalf("a plain shell reported agent %q", got)
	}
}

func TestDirectKeyLabelsUseMenuOrder(t *testing.T) {
	for id, want := range map[string]string{"split_horizontal": "⇧⌘D", "new_tab": "⌘T", "focus_pane_left": "⌥⌘←", "next_tab": "⇧⌘]", "zen": "⌃⌘Z", "rename_tab": ""} {
		if got := directKeyLabel(id); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestCopyConfirmationShowsThenFades(t *testing.T) {
	w := populatedChromeWindow(t)
	tester := native.NewTester(w.View, 1000, 700)
	if _, ok := tester.Find("Copied to clipboard"); ok {
		t.Fatal("confirmation showed before a copy")
	}
	w.ShowCopied()
	tester.Frame()
	if _, ok := tester.Find("Copied to clipboard"); !ok {
		t.Fatal("copy was not confirmed")
	}
	tester.Advance(copyShown)
	if _, ok := tester.Find("Copied to clipboard"); ok {
		t.Fatal("confirmation outstayed its time")
	}
}

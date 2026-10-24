package main

import (
	"encoding/json"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/session"
)

// fake collects the platform actions a test replaces, and answers clicks.
type fake struct {
	active   bool
	disabled bool
	shown    []shownNotification
	badges   []int
	cleared  int
	jumps    [][2]string
}

type shownNotification struct {
	group, title, body string
	click              func()
}

func (f *fake) notifier() *notifier {
	n := newNotifier(func() bool { return f.active }, func(workspaceID, paneID string) { f.jumps = append(f.jumps, [2]string{workspaceID, paneID}) })
	n.supported = func() bool { return !f.disabled }
	n.show = func(group, title, body string, click func()) {
		f.shown = append(f.shown, shownNotification{group, title, body, click})
	}
	n.badge = func(count int) { f.badges = append(f.badges, count) }
	n.clearAll = func() { f.cleared++ }
	return n
}

func agentSnapshot(t *testing.T, panes string) session.PublishedSnapshot {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(panes), &value); err != nil {
		t.Fatal(err)
	}
	wire := map[string]any{"version": "test", "protocol": 1, "workspaces": []any{map[string]any{"workspace_id": "w1", "label": "Website"}}, "tabs": []any{map[string]any{"tab_id": "a", "workspace_id": "w1"}}, "panes": value, "layouts": []any{}, "agents": []any{}}
	data, err := json.Marshal(map[string]any{"type": "session_snapshot", "snapshot": wire})
	if err != nil {
		t.Fatal(err)
	}
	s, err := session.DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	return session.PublishedSnapshot{Snapshot: s}
}

func agentPanes(status string) string {
	return `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"codex","agent_status":"` + status + `"}]`
}

func TestFirstSightAndUnchangedStatusStayQuiet(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("working")))
	if len(f.shown) != 0 {
		t.Fatal("first sight or an unchanged status notified", f.shown)
	}
	if len(f.badges) != 2 || f.badges[1] != 0 {
		t.Fatal("badge did not follow the blocked count", f.badges)
	}
}

func TestBlockedNeedsTheUser(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 1 || f.shown[0].group != "p1" || f.shown[0].title != "codex needs you" || f.shown[0].body != "Website" {
		t.Fatal("wrong needs-you notification", f.shown)
	}
	f.shown[0].click()
	if len(f.jumps) != 1 || f.jumps[0] != [2]string{"w1", "p1"} {
		t.Fatal("a click did not jump to the agent", f.jumps)
	}
	if len(f.badges) != 2 || f.badges[1] != 1 {
		t.Fatal("badge did not count the blocked agent", f.badges)
	}
}

func TestDoneAfterWorkingFinishes(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("done")))
	if len(f.shown) != 1 || f.shown[0].title != "codex finished" {
		t.Fatal("working to done did not finish", f.shown)
	}
	if len(f.badges) != 2 || f.badges[1] != 0 {
		t.Fatal("done left a badge", f.badges)
	}
}

func TestDoneWithoutWorkingStaysQuiet(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("idle")))
	n.Step(agentSnapshot(t, agentPanes("done")))
	if len(f.shown) != 0 {
		t.Fatal("done without working notified", f.shown)
	}
}

func TestBlockedToDoneStaysQuiet(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	n.Step(agentSnapshot(t, agentPanes("done")))
	if len(f.shown) != 0 {
		t.Fatal("blocked to done notified", f.shown)
	}
}

func TestQuietWhileActiveStillBadges(t *testing.T) {
	f := &fake{active: true}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 0 {
		t.Fatal("an active window was notified", f.shown)
	}
	if len(f.badges) != 2 || f.badges[1] != 1 {
		t.Fatal("an active window lost the badge", f.badges)
	}
}

func TestUnsupportedDesktopKeepsOnlyTheBadge(t *testing.T) {
	f := &fake{disabled: true}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 0 {
		t.Fatal("an unsupported desktop was notified", f.shown)
	}
	if len(f.badges) != 2 || f.badges[1] != 1 {
		t.Fatal("an unsupported desktop lost the badge", f.badges)
	}
}

func TestVanishedAgentIsForgotten(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, `[]`))
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 0 {
		t.Fatal("a returning agent was not a first sight again", f.shown)
	}
	if f.badges[len(f.badges)-1] != 1 {
		t.Fatal("a returning blocked agent did not badge", f.badges)
	}
}

func TestPublicationWithoutSnapshotForgetsAgents(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(session.PublishedSnapshot{})
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 0 {
		t.Fatal("an agent behind a snapshotless publication notified", f.shown)
	}
}

func TestClearRecountsTheBadge(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"codex","agent_status":"blocked"},{"pane_id":"p2","terminal_id":"t2","workspace_id":"w1","tab_id":"a","agent":"claude","agent_status":"blocked"}]`))
	n.Clear()
	if f.cleared != 1 {
		t.Fatal("clear did not remove delivered notifications")
	}
	if f.badges[len(f.badges)-1] != 2 {
		t.Fatal("clear zeroed a badge that still counts", f.badges)
	}
}

func TestStatusCaseDoesNotMatter(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("Working")))
	n.Step(agentSnapshot(t, agentPanes("DONE")))
	if len(f.shown) != 1 || f.shown[0].title != "codex finished" {
		t.Fatal("a case change hid the finish", f.shown)
	}
	n.Step(agentSnapshot(t, agentPanes("Blocked")))
	if len(f.shown) != 2 || f.shown[1].title != "codex needs you" {
		t.Fatal("a case change hid the need", f.shown)
	}
	if f.badges[len(f.badges)-1] != 1 {
		t.Fatal("a case change hid the badge", f.badges)
	}
}

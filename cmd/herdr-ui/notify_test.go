package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/session"
)

// fake collects the platform actions a test replaces, and answers clicks.
type fake struct {
	disabled bool
	shown    []shownNotification
	badges   []int
	cleared  int
	opened   []app.Notice
}

type shownNotification struct {
	group, title, body string
	click              func()
}

func (f *fake) notifier() *notifier {
	n := newNotifier(func(notice app.Notice) { f.opened = append(f.opened, notice) })
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

func TestBadgeCountsBlockedAgents(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("working")))
	n.Step(agentSnapshot(t, agentPanes("Blocked")))
	n.Step(session.PublishedSnapshot{})
	if len(f.badges) != 3 || f.badges[0] != 0 || f.badges[1] != 1 || f.badges[2] != 0 {
		t.Fatal("badge did not follow the blocked count", f.badges)
	}
	if len(f.shown) != 0 {
		t.Fatal("a status change posted on its own; posting is the queue's call", f.shown)
	}
}

func TestPostShowsOneSlotPerPaneAndOpensItsTarget(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	notice := app.Notice{Kind: "NeedsAttention", Title: "codex needs attention", Body: "Website · 1", WorkspaceID: "w1", PaneID: "w1:p1", Received: time.Unix(1, 0)}
	n.Post(notice)
	n.Post(app.Notice{Kind: "Custom", Title: "hello", Received: time.Unix(2, 0)})
	if len(f.shown) != 2 || f.shown[0].group != "w1:p1" || f.shown[0].title != "codex needs attention" || f.shown[0].body != "Website · 1" || f.shown[1].group != "notice/2000000000" {
		t.Fatal("wrong system notifications", f.shown)
	}
	f.shown[0].click()
	if len(f.opened) != 1 || f.opened[0] != notice {
		t.Fatal("a click did not open the notice's target", f.opened)
	}
}

func TestOffMutesPostsAndBadge(t *testing.T) {
	f := &fake{}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	n.SetEnabled(false)
	n.Post(app.Notice{Title: "muted"})
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	if len(f.shown) != 0 || f.cleared != 1 || f.badges[len(f.badges)-1] != 0 || len(f.badges) != 2 {
		t.Fatal("off did not mute everything", f.shown, f.cleared, f.badges)
	}
	n.SetEnabled(true)
	if f.badges[len(f.badges)-1] != 1 {
		t.Fatal("turning back on did not restore the badge", f.badges)
	}
}

func TestUnsupportedDesktopKeepsOnlyTheBadge(t *testing.T) {
	f := &fake{disabled: true}
	n := f.notifier()
	n.Step(agentSnapshot(t, agentPanes("blocked")))
	n.Post(app.Notice{Title: "nowhere"})
	if len(f.shown) != 0 || f.badges[len(f.badges)-1] != 1 {
		t.Fatal("an unsupported desktop was notified or lost the badge", f.shown, f.badges)
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

package app

import (
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/session"
)

func toastSnapshot(t *testing.T, status string) *session.Snapshot {
	return snapshot(t, `[{"workspace_id":"w1"}]`, `[{"tab_id":"a","workspace_id":"w1"},{"tab_id":"b","workspace_id":"w1"}]`,
		`[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"codex","agent_status":"`+status+`"},{"pane_id":"p2","terminal_id":"t2","workspace_id":"w1","tab_id":"a","agent":"claude","agent_status":"blocked"}]`)
}

func toastContext(s *session.Snapshot) ToastContext {
	return ToastContext{Snapshot: s, SelectedTabID: "b", Active: true, Delivery: DeliveryInApp}
}

func TestToastTimeStartsWhenShown(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	q.Receive(Notice{Kind: "NeedsAttention", Title: "first", Received: now})
	q.Receive(Notice{Kind: "Custom", Title: "second", Received: now})
	shown, ok, _, wake := q.Tick(now, ctx)
	if !ok || shown.Title != "first" || wake != 8*time.Second {
		t.Fatal("first toast not shown for its full time", shown, wake)
	}
	shown, _, _, _ = q.Tick(now.Add(8*time.Second), ctx)
	if shown.Title != "second" || shown.Remaining != 5*time.Second {
		t.Fatal("queued toast lost time while waiting", shown)
	}
	if _, ok, _, wake := q.Tick(now.Add(13*time.Second), ctx); ok || wake != 0 {
		t.Fatal("expired toast is still shown", wake)
	}
}

func TestToastPausesWhileWindowInactive(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	q.Receive(Notice{Kind: "Custom", Title: "hello", Received: now})
	q.Tick(now, ctx)
	ctx.Active = false
	q.Tick(now.Add(2*time.Second), ctx)
	q.Tick(now.Add(time.Minute), ctx)
	ctx.Active = true
	shown, ok, _, _ := q.Tick(now.Add(time.Minute+time.Second), ctx)
	if !ok || shown.Remaining != 3*time.Second {
		t.Fatal("background time was spent", shown)
	}
}

func TestNewerNoticeReplacesSamePane(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "done"))
	q.Receive(Notice{Kind: "NeedsAttention", Title: "codex needs attention", PaneID: "p1", Received: now})
	q.Tick(now, ctx)
	q.Receive(Notice{Kind: "Finished", Title: "codex finished", PaneID: "p1", Received: now})
	shown, ok, _, _ := q.Tick(now, ctx)
	if !ok || shown.Title != "codex finished" {
		t.Fatal("older notice for the pane was kept", shown)
	}
}

func TestFinishedWaitsForSnapshotThenExpires(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	q.Receive(Notice{Kind: "Finished", Title: "codex finished", PaneID: "p1", Received: now})
	if _, ok, _, wake := q.Tick(now, ctx); ok || wake != toastRecheck {
		t.Fatal("unconfirmed completion shown or not rechecked", wake)
	}
	ctx.Snapshot = toastSnapshot(t, "done")
	if shown, ok, _, _ := q.Tick(now.Add(toastRecheck), ctx); !ok || shown.Title != "codex finished" {
		t.Fatal("confirmed completion not shown")
	}

	q = Toasts{}
	ctx.Snapshot = toastSnapshot(t, "working")
	q.Receive(Notice{Kind: "Finished", Title: "late", PaneID: "p1", Received: now})
	q.Tick(now, ctx)
	if _, ok, _, wake := q.Tick(now.Add(toastGrace), ctx); ok || wake != 0 {
		t.Fatal("unconfirmed completion outlived its grace")
	}
}

func TestStaleOrTargetlessFinishedIsDropped(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "idle"))
	q.Receive(Notice{Kind: "Finished", Title: "idle", PaneID: "p1", Received: now})
	q.Receive(Notice{Kind: "Finished", Title: "nowhere", Received: now})
	if _, ok, _, wake := q.Tick(now, ctx); ok || wake != 0 {
		t.Fatal("stale completion shown")
	}
}

func TestNoticeForSelectedTabIsDropped(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	ctx.SelectedTabID = "a"
	q.Receive(Notice{Kind: "NeedsAttention", Title: "here", PaneID: "p2", Received: now})
	if _, ok, _, _ := q.Tick(now, ctx); ok {
		t.Fatal("notice for the tab on screen was shown")
	}
	ctx.SelectedTabID = "b"
	q.Receive(Notice{Kind: "NeedsAttention", Title: "there", PaneID: "p2", Received: now})
	if _, ok, _, _ := q.Tick(now, ctx); !ok {
		t.Fatal("notice for another tab was not shown")
	}
	ctx.SelectedTabID = "a"
	if _, ok, _, _ := q.Tick(now, ctx); ok {
		t.Fatal("toast stayed after its tab was selected")
	}
}

func TestDisabledClearsEverything(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	q.Receive(Notice{Kind: "Custom", Title: "a", Received: now})
	q.Receive(Notice{Kind: "Custom", Title: "b", Received: now})
	q.Tick(now, ctx)
	ctx.Delivery = DeliveryOff
	q.Tick(now, ctx)
	ctx.Delivery = DeliveryInApp
	if _, ok, _, _ := q.Tick(now, ctx); ok {
		t.Fatal("muted notices came back")
	}
}

func TestOpenAndDismissAdvanceTheQueue(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	for _, title := range []string{"a", "b", "c"} {
		q.Receive(Notice{Kind: "Custom", Title: title, Received: now})
	}
	q.Tick(now, ctx)
	if n, ok := q.Open(); !ok || n.Title != "a" {
		t.Fatal("open did not return the visible toast", n)
	}
	if shown, _, _, _ := q.Tick(now, ctx); shown.Title != "b" {
		t.Fatal("queue did not advance after open", shown)
	}
	q.Dismiss()
	if shown, _, _, _ := q.Tick(now, ctx); shown.Title != "c" {
		t.Fatal("queue did not advance after dismiss", shown)
	}
	q.Dismiss()
	if _, ok := q.Open(); ok {
		t.Fatal("open without a toast returned one")
	}
}

func TestQueueIsBounded(t *testing.T) {
	now := time.Now()
	var q Toasts
	for i := 0; i < 20; i++ {
		q.Receive(Notice{Kind: "Custom", Title: string(rune('a' + i)), Received: now})
	}
	shown, _, _, _ := q.Tick(now, toastContext(nil))
	if len(q.pending) != toastQueueLimit-1 || shown.Title != "m" {
		t.Fatal("queue is not bounded to the newest notices", len(q.pending), shown.Title)
	}
}

func TestSystemDeliveryHandsOutReadyNotices(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "done"))
	ctx.Delivery = DeliverySystem
	q.Receive(Notice{Kind: "NeedsAttention", Title: "claude needs attention", PaneID: "p2", Received: now})
	q.Receive(Notice{Kind: "Finished", Title: "codex finished", PaneID: "p1", Received: now})
	_, ok, system, wake := q.Tick(now, ctx)
	if ok || len(system) != 2 || system[0].Title != "claude needs attention" || wake != 0 {
		t.Fatal("system delivery did not hand out every ready notice", ok, system, wake)
	}
	if _, _, again, _ := q.Tick(now, ctx); len(again) != 0 {
		t.Fatal("a system notice was handed out twice", again)
	}
}

func TestSystemDeliveryReachesTheTabOnScreenOnlyWhileAway(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	ctx.Delivery, ctx.SelectedTabID = DeliverySystem, "a"
	q.Receive(Notice{Kind: "NeedsAttention", Title: "watched", PaneID: "p2", Received: now})
	if _, _, system, _ := q.Tick(now, ctx); len(system) != 0 {
		t.Fatal("a notice for the watched tab reached the system", system)
	}
	ctx.Active = false
	q.Receive(Notice{Kind: "NeedsAttention", Title: "away", PaneID: "p2", Received: now})
	if _, _, system, _ := q.Tick(now, ctx); len(system) != 1 {
		t.Fatal("a notice for the tab on screen was lost while away", system)
	}
}

func TestDelayHoldsAgentNoticesAndDropsStaleOnes(t *testing.T) {
	now := time.Now()
	var q Toasts
	ctx := toastContext(toastSnapshot(t, "working"))
	ctx.Delay = time.Second
	q.Receive(Notice{Kind: "NeedsAttention", Title: "still blocked", PaneID: "p2", Received: now})
	q.Receive(Notice{Kind: "NeedsAttention", Title: "answered", PaneID: "p1", Received: now})
	q.Receive(Notice{Kind: "Custom", Title: "custom", Received: now})
	shown, ok, _, wake := q.Tick(now, ctx)
	if !ok || shown.Title != "custom" || wake != time.Second {
		t.Fatal("custom notice waited or agent notice did not", shown, wake)
	}
	q.Dismiss()
	shown, ok, _, _ = q.Tick(now.Add(time.Second), ctx)
	if !ok || shown.Title != "still blocked" {
		t.Fatal("a still-current notice was not shown after the delay", shown)
	}
	q.Dismiss()
	if _, ok, _, wake := q.Tick(now.Add(time.Second), ctx); ok || wake != 0 {
		t.Fatal("a stale notice outlived the delay")
	}
}

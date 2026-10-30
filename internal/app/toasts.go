package app

import (
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/session"
)

// Notice is one notification Herdr sent to this client, as the in-app toast
// shows it.
type Notice struct {
	Kind, Title, Body, WorkspaceID, TabID, PaneID, Position string
	Received                                                time.Time
}

// Toast is the notice on screen and how long it has left there.
type Toast struct {
	Notice
	Remaining time.Duration
}

// Notification delivery, named as Herdr's [ui.toast] delivery values.
const (
	DeliveryOff    = "off"
	DeliveryInApp  = "herdr"
	DeliverySystem = "system"
)

// ToastContext is what the queue checks notices against on each tick.
type ToastContext struct {
	Snapshot *session.Snapshot
	// SelectedTabID is the tab the window shows; its own notices are
	// redundant while the user can see it.
	SelectedTabID string
	// Active reports whether the window has the focus. It pauses an in-app
	// toast's countdown, so a toast is not spent before anyone could read
	// it, and lets a system notification through for the tab on screen.
	Active bool
	// Delivery is one of the Delivery values.
	Delivery string
	// Delay holds agent notices back, then shows them only if the agent
	// is still in the state they report, as Herdr's delay_seconds does.
	Delay time.Duration
}

const (
	toastQueueLimit = 8
	// toastGrace bounds how long a notice may wait for the session snapshot
	// to confirm the status it reports, as the two travel separately.
	toastGrace   = time.Second
	toastRecheck = 50 * time.Millisecond
)

func toastDuration(kind string) time.Duration {
	switch kind {
	case "NeedsAttention":
		return 8 * time.Second
	case "UpdateInstalled":
		return 3 * time.Second
	}
	return 5 * time.Second
}

// Toasts delivers Herdr's notifications the way Herdr's own client does: a
// newer notice for a pane replaces an older one, agent notices wait out the
// delay and are dropped once stale, a completion is shown only once the
// snapshot agrees the agent is done, and the tab on screen gets none. In-app
// toasts show one at a time, in arrival order, their time starting when they
// appear; system ones are handed out as soon as they are ready.
type Toasts struct {
	pending   []Notice
	visible   *Notice
	remaining time.Duration
	// counted is the last tick that spent visible time; zero while paused.
	counted time.Time
}

// Receive queues a notice, replacing whatever is queued or shown for the
// same pane.
func (t *Toasts) Receive(n Notice) {
	if n.PaneID != "" {
		kept := t.pending[:0]
		for _, old := range t.pending {
			if old.PaneID != n.PaneID {
				kept = append(kept, old)
			}
		}
		t.pending = kept
		if t.visible != nil && t.visible.PaneID == n.PaneID {
			t.visible = nil
		}
	}
	if len(t.pending) == toastQueueLimit {
		t.pending = t.pending[1:]
	}
	t.pending = append(t.pending, n)
}

// Tick advances the queue to now. It returns the in-app toast to show, if
// any, the notices ready for the system, and how soon the queue needs
// another tick (zero when nothing is waiting).
func (t *Toasts) Tick(now time.Time, ctx ToastContext) (Toast, bool, []Notice, time.Duration) {
	if ctx.Delivery != DeliveryInApp && ctx.Delivery != DeliverySystem {
		t.pending, t.visible = nil, nil
		return Toast{}, false, nil, 0
	}
	if ctx.Delivery != DeliveryInApp {
		t.visible = nil
	}
	if t.visible != nil {
		if t.onScreen(*t.visible, ctx) {
			t.visible = nil
		} else {
			// Time since the last tick counts only if the window was
			// active through it; a paused tick resumes the count afresh.
			// Ticks come from frames and from outside them, whose clocks
			// may disagree slightly; time never runs backwards here.
			if !t.counted.IsZero() && now.After(t.counted) {
				t.remaining -= now.Sub(t.counted)
			}
			if !ctx.Active {
				t.counted = time.Time{}
			} else if t.counted.IsZero() || now.After(t.counted) {
				t.counted = now
			}
			if t.remaining <= 0 {
				t.visible = nil
			}
		}
	}
	var system []Notice
	wake := time.Duration(0)
	kept := t.pending[:0]
	for _, n := range t.pending {
		ready, keep, retry := t.check(n, now, ctx)
		switch {
		case !keep:
		case ready && ctx.Delivery == DeliverySystem:
			system = append(system, n)
		case ready && t.visible == nil:
			shown := n
			t.visible = &shown
			t.remaining = toastDuration(n.Kind)
			t.counted = time.Time{}
			if ctx.Active {
				t.counted = now
			}
		default:
			kept = append(kept, n)
			if !ready {
				wake = soonest(wake, retry)
			}
		}
	}
	t.pending = kept
	if t.visible == nil {
		return Toast{}, false, system, wake
	}
	if ctx.Active {
		wake = soonest(wake, t.remaining)
	}
	return Toast{Notice: *t.visible, Remaining: t.remaining}, true, system, wake
}

// Dismiss removes the toast on screen; the next queued one follows.
func (t *Toasts) Dismiss() { t.visible = nil }

// Open removes the toast on screen and returns it, so its target can be
// selected.
func (t *Toasts) Open() (Notice, bool) {
	if t.visible == nil {
		return Notice{}, false
	}
	n := *t.visible
	t.visible = nil
	return n, true
}

// check reports whether a queued notice may be delivered now, whether it is
// still worth keeping at all, and if it must wait, for how long.
func (t *Toasts) check(n Notice, now time.Time, ctx ToastContext) (ready, keep bool, retry time.Duration) {
	agent := n.Kind == "NeedsAttention" || n.Kind == "Finished"
	if agent {
		if wait := n.Received.Add(ctx.Delay).Sub(now); wait > 0 {
			return false, true, wait
		}
	}
	// Agent notices name a pane whose state may have moved on; a completion
	// carries no proof of its own, so it is always checked.
	if agent && (ctx.Delay > 0 || n.Kind == "Finished") {
		if n.PaneID == "" {
			if n.Kind == "Finished" {
				return false, false, 0
			}
		} else {
			want := map[string]string{"NeedsAttention": "blocked", "Finished": "done"}[n.Kind]
			switch status := t.status(n.PaneID, ctx); {
			case status == want:
			case status == "" || (n.Kind == "Finished" && status == "working"):
				// The snapshot travels apart from the notice; give it a moment.
				return false, now.Sub(n.Received) < toastGrace, toastRecheck
			default:
				return false, false, 0
			}
		}
	}
	if t.onScreen(n, ctx) {
		return false, false, 0
	}
	return true, true, 0
}

// onScreen reports whether the user already sees the notice's tab: for an
// in-app toast whenever the tab is selected, for a system notification only
// while the window has the focus too.
func (t *Toasts) onScreen(n Notice, ctx ToastContext) bool {
	if n.Kind == "OutputMatched" {
		return false
	}
	if ctx.Delivery == DeliverySystem && !ctx.Active {
		return false
	}
	target := t.target(n, ctx)
	return target != "" && target == ctx.SelectedTabID
}

func (t *Toasts) status(paneID string, ctx ToastContext) string {
	if ctx.Snapshot == nil {
		return ""
	}
	pane, ok := ctx.Snapshot.Pane(paneID)
	if !ok {
		return ""
	}
	return strings.ToLower(pane.Status)
}

// target is the tab a notice belongs to, from the notice or its pane.
func (t *Toasts) target(n Notice, ctx ToastContext) string {
	if n.TabID != "" {
		return n.TabID
	}
	if ctx.Snapshot != nil && n.PaneID != "" {
		if pane, ok := ctx.Snapshot.Pane(n.PaneID); ok {
			return pane.TabID
		}
	}
	return ""
}

func soonest(a, b time.Duration) time.Duration {
	if a == 0 || b < a {
		return b
	}
	return a
}

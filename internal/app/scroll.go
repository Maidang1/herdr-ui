package app

import "github.com/Maidang1/herdr-ui/internal/session"

// ScrollTracker is UI-thread owned. It accumulates against the latest desired
// absolute offset, not a slow snapshot's baseline, without storing wheel history.
type ScrollTracker struct {
	target, epoch, revision uint64
	pane, terminal          string
	desired                 uint64
	pending                 bool
	ticket                  uint64
}

func (t *ScrollTracker) Wheel(p session.PublishedSnapshot, pane session.Pane, rows int) (session.Operation, uint64, bool) {
	if p.State != session.Ready || p.Stale || !pane.Scroll.Known || rows == 0 {
		return session.Operation{}, 0, false
	}
	if t.target != p.TargetID || t.epoch != p.Epoch || t.pane != pane.ID || t.terminal != pane.TerminalID {
		t.target, t.epoch, t.pane, t.terminal = p.TargetID, p.Epoch, pane.ID, pane.TerminalID
		t.desired = pane.Scroll.Offset
		t.pending = false
		t.ticket++
	} else if !t.pending && t.revision != p.Revision {
		t.desired = pane.Scroll.Offset
	}
	t.revision = p.Revision
	t.desired = min(t.desired, pane.Scroll.MaxOffset)
	before := t.desired
	if rows < 0 {
		amount := uint64(-(rows + 1)) + 1
		t.desired += min(amount, pane.Scroll.MaxOffset-t.desired)
	} else {
		t.desired -= min(uint64(rows), t.desired)
	}
	if t.desired == before {
		return session.Operation{}, 0, false
	}
	t.ticket++
	t.pending = true
	return session.Operation{Kind: session.Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: pane.ID, TerminalID: pane.TerminalID, Offset: t.desired}, t.ticket, true
}

func (t *ScrollTracker) Complete(ticket uint64, result session.OperationResult, p session.PublishedSnapshot, pane session.Pane) {
	if ticket != t.ticket || t.target != p.TargetID || t.epoch != p.Epoch || pane.ID != t.pane || pane.TerminalID != t.terminal {
		return
	}
	t.pending = false
	t.revision = p.Revision
	if result.Status == session.Succeeded && result.Scroll.Known {
		t.desired = result.Scroll.Offset
	} else {
		t.desired = pane.Scroll.Offset
	}
}

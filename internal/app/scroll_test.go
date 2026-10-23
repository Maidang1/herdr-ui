package app

import (
	"github.com/Maidang1/herdr-ui/internal/session"
	"testing"
)

func TestScrollAccumulatesBeyondSlowSnapshots(t *testing.T) {
	tracker := ScrollTracker{}
	p := session.PublishedSnapshot{TargetID: 1, Epoch: 1, Revision: 3, State: session.Ready}
	pane := session.Pane{ID: "p", TerminalID: "t", Scroll: session.ScrollInfo{Offset: 10, MaxOffset: 100, Known: true}}
	first, ticket1, ok := tracker.Wheel(p, pane, -4)
	if !ok || first.Offset != 14 {
		t.Fatal(first)
	}
	second, ticket2, ok := tracker.Wheel(p, pane, -4)
	if !ok || second.Offset != 18 {
		t.Fatal(second)
	}
	tracker.Complete(ticket1, session.OperationResult{Status: session.Superseded}, p, pane)
	third, _, _ := tracker.Wheel(p, pane, 2)
	if third.Offset != 16 {
		t.Fatal("old completion reset pending offset")
	}
	tracker.Complete(ticket2, session.OperationResult{Status: session.Succeeded, Scroll: session.ScrollInfo{Offset: 18, MaxOffset: 100, Known: true}}, p, pane)
	fourth, _, _ := tracker.Wheel(p, pane, -3)
	if fourth.Offset != 19 {
		t.Fatal(fourth)
	}
	p.Epoch = 2
	reset, _, _ := tracker.Wheel(p, pane, -1)
	if reset.Offset != 11 {
		t.Fatal("epoch reused speculative offset")
	}
}

func TestScrollFailureRevertsAndClamps(t *testing.T) {
	tracker := ScrollTracker{}
	p := session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready}
	pane := session.Pane{ID: "p", TerminalID: "t", Scroll: session.ScrollInfo{Offset: 3, MaxOffset: 10, Known: true}}
	op, ticket, _ := tracker.Wheel(p, pane, -100)
	if op.Offset != 10 {
		t.Fatal(op)
	}
	tracker.Complete(ticket, session.OperationResult{Status: session.Rejected}, p, pane)
	op, _, _ = tracker.Wheel(p, pane, 1)
	if op.Offset != 2 {
		t.Fatal(op)
	}
	p.Stale = true
	if _, _, ok := tracker.Wheel(p, pane, -1); ok {
		t.Fatal("stale state accepted wheel")
	}
}

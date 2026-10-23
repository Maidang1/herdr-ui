package app

import (
	"github.com/bytedance/herdr-ui/internal/session"
	"testing"
)

func TestLateSplitCannotStealNewTabSelection(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 1, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")})
	op := session.Operation{Kind: session.Split, TargetID: 1, Epoch: 1, WorkspaceID: "w1", TabID: "a", PaneID: "p1", TerminalID: "t1", Direction: "right"}
	intent, ok := a.BeginOperation(op)
	if !ok {
		t.Fatal("operation rejected")
	}
	if _, ok := a.BeginOperation(op); ok {
		t.Fatal("duplicate split accepted")
	}
	a.SelectTab("b")
	a.CompleteOperation(intent, session.OperationResult{Status: session.Succeeded, PaneID: "p2"})
	if a.View().TabID != "b" {
		t.Fatal("late response stole focus")
	}
}

func TestSplitIntentWaitsForAuthoritativePane(t *testing.T) {
	a := New()
	base := snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")
	a.Update(session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 1, Snapshot: base})
	op := session.Operation{Kind: session.Split, TargetID: 1, Epoch: 1, WorkspaceID: "w1", TabID: "a", PaneID: "p1", TerminalID: "t1", Direction: "right"}
	intent, _ := a.BeginOperation(op)
	a.CompleteOperation(intent, session.OperationResult{Status: session.Succeeded, PaneID: "new"})
	if a.View().PaneID != "p1" || !a.Pending(session.Split, "p1") {
		t.Fatal("selected pane before snapshot")
	}
	fresh := snapshot(t, workspaces, tabs, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a"},{"pane_id":"new","terminal_id":"newterm","workspace_id":"w1","tab_id":"a"}]`, "w1", "a", "new")
	a.Update(session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 2, Snapshot: fresh})
	if a.View().PaneID != "new" || a.Pending(session.Split, "p1") {
		t.Fatal("pending intent not applied exactly once")
	}
}

func TestTabCycleStaysInsideWorkspace(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")})
	a.CycleTab(-1)
	if a.View().TabID != "b" {
		t.Fatal(a.View())
	}
	a.CycleTab(1)
	if a.View().TabID != "a" {
		t.Fatal(a.View())
	}
}

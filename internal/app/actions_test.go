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

func TestSnapshotBeforeSplitResponseStillAppliesIntent(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 1, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")})
	op := session.Operation{Kind: session.Split, TargetID: 1, Epoch: 1, WorkspaceID: "w1", TabID: "a", PaneID: "p1", TerminalID: "t1"}
	intent, _ := a.BeginOperation(op)
	fresh := snapshot(t, workspaces, tabs, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a"},{"pane_id":"new","terminal_id":"newterm","workspace_id":"w1","tab_id":"a"}]`, "w1", "a", "new")
	a.Update(session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 2, Snapshot: fresh})
	if a.View().PaneID != "p1" {
		t.Fatal("remote focus stole local selection")
	}
	a.CompleteOperation(intent, session.OperationResult{Status: session.Succeeded, PaneID: "new"})
	if a.View().PaneID != "new" || a.Pending(session.Split, "p1") {
		t.Fatal("response did not confirm existing pane")
	}
}

func TestUnknownActionMessageRemainsAccurateAfterRefresh(t *testing.T) {
	a := New()
	p := session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready, Revision: 1, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")}
	a.Update(p)
	intent, _ := a.BeginOperation(session.Operation{Kind: session.Split, TargetID: 1, Epoch: 1, WorkspaceID: "w1", TabID: "a", PaneID: "p1", TerminalID: "t1"})
	a.CompleteOperation(intent, session.OperationResult{Status: session.OutcomeUnknown})
	p.Revision++
	a.Update(p)
	if a.Pending(session.Split, "p1") {
		t.Fatal("unknown action pending after refresh")
	}
	if a.View().ActionMessage == "" {
		t.Fatal("lost unknown result warning")
	}
}

func TestRemoteZoomCannotLeaveSelectedPaneHidden(t *testing.T) {
	a := New()
	ps := `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a"},{"pane_id":"pnew","terminal_id":"tn","workspace_id":"w1","tab_id":"a"}]`
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, ps, "w1", "a", "p1")})
	a.SelectPane("p1")
	layouts := `[{"workspace_id":"w1","tab_id":"a","zoomed":true,"focused_pane_id":"pnew","area":{"x":0,"y":0,"width":80,"height":24},"panes":[{"pane_id":"p1","rect":{"x":0,"y":0,"width":40,"height":24}},{"pane_id":"pnew","rect":{"x":40,"y":0,"width":40,"height":24}}],"splits":[]}]`
	a.Update(session.PublishedSnapshot{Snapshot: snapshotWithLayouts(t, workspaces, tabs, ps, layouts, "w1", "a", "pnew")})
	if a.View().PaneID != "pnew" {
		t.Fatal("zoom left hidden selection")
	}
	a.SelectPane("p1")
	if a.View().PaneID != "pnew" {
		t.Fatal("sidebar selected hidden zoom pane")
	}
}

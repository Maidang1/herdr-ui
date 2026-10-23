package app

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/session"
)

func snapshot(t *testing.T, workspaces, tabs, panes string, focus ...string) *session.Snapshot {
	return snapshotWithLayouts(t, workspaces, tabs, panes, `[]`, focus...)
}

func snapshotWithLayouts(t *testing.T, workspaces, tabs, panes, layouts string, focus ...string) *session.Snapshot {
	t.Helper()
	w := map[string]any{"version": "test", "protocol": 1, "layouts": []any{}, "agents": []any{}}
	for k, v := range map[string]string{"workspaces": workspaces, "tabs": tabs, "panes": panes, "layouts": layouts} {
		var value any
		if err := json.Unmarshal([]byte(v), &value); err != nil {
			t.Fatal(err)
		}
		w[k] = value
	}
	for i, key := range []string{"focused_workspace_id", "focused_tab_id", "focused_pane_id"} {
		if i < len(focus) {
			w[key] = focus[i]
		}
	}
	data, _ := json.Marshal(map[string]any{"type": "session_snapshot", "snapshot": w})
	s, err := session.DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestUnvisitedScopesUseCurrentServerFocus(t *testing.T) {
	ws1 := `[{"workspace_id":"w1"},{"workspace_id":"w2","active_tab_id":"c"}]`
	ws2 := `[{"workspace_id":"w1"},{"workspace_id":"w2","active_tab_id":"d"}]`
	ts := `[{"tab_id":"a","workspace_id":"w1"},{"tab_id":"b","workspace_id":"w1"},{"tab_id":"c","workspace_id":"w2"},{"tab_id":"d","workspace_id":"w2"}]`
	ps1 := `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a"},{"pane_id":"p2","terminal_id":"t2","workspace_id":"w1","tab_id":"b","focused":true},{"pane_id":"p3","terminal_id":"t3","workspace_id":"w1","tab_id":"b"}]`
	ps2 := `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a"},{"pane_id":"p2","terminal_id":"t2","workspace_id":"w1","tab_id":"b"},{"pane_id":"p3","terminal_id":"t3","workspace_id":"w1","tab_id":"b","focused":true}]`
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, ws1, ts, ps1, "w1", "a", "p1")})
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, ws2, ts, ps2, "w1", "a", "p1")})
	a.SelectTab("b")
	if a.View().PaneID != "p3" {
		t.Fatal("unvisited tab retained obsolete automatic pane fallback")
	}
	a.SelectWorkspace("w2")
	if a.View().TabID != "d" {
		t.Fatal("unvisited workspace retained obsolete automatic tab fallback")
	}
}

func TestInactiveTabUsesLayoutFocus(t *testing.T) {
	layouts := `[{"workspace_id":"w1","tab_id":"a","focused_pane_id":"p2","area":{"x":0,"y":0,"width":80,"height":24},"panes":[],"splits":[]}]`
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshotWithLayouts(t, workspaces, tabs, panes, layouts, "w2", "c", "p4")})
	a.SelectWorkspace("w1")
	if a.View().PaneID != "p2" {
		t.Fatal("inactive tab must restore per-layout focus")
	}
}

const workspaces = `[{"workspace_id":"w1","label":"One"},{"workspace_id":"w2","label":"Two"}]`
const tabs = `[{"tab_id":"a","workspace_id":"w1","number":1},{"tab_id":"b","workspace_id":"w1","number":2},{"tab_id":"c","workspace_id":"w2","label":"Server","number":1}]`
const panes = `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","title":"First","agent":"Codex","agent_status":"Working","cwd":"/real/one"},{"pane_id":"p2","terminal_id":"t2","workspace_id":"w1","tab_id":"a","title":"Second","agent":"Codex","agent_status":"Idle"},{"pane_id":"p3","terminal_id":"t3","workspace_id":"w1","tab_id":"b"},{"pane_id":"p4","terminal_id":"t4","workspace_id":"w2","tab_id":"c"}]`

func TestSelectionsPreserveLocalFocusAndRememberScopes(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1"), State: session.Ready})
	a.SelectPane("p2")
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes, "w2", "c", "p4"), State: session.Ready})
	if v := a.View(); v.WorkspaceID != "w1" || v.TabID != "a" || v.PaneID != "p2" {
		t.Fatalf("server stole focus: %+v", v)
	}
	a.SelectTab("b")
	a.SelectWorkspace("w2")
	a.SelectWorkspace("w1")
	if v := a.View(); v.TabID != "b" || v.PaneID != "p3" {
		t.Fatalf("workspace memory: %+v", v)
	}
	a.SelectTab("a")
	if v := a.View(); v.PaneID != "p2" || v.Rows[0].Status != "Idle" {
		t.Fatalf("pane memory/status: %+v", v)
	}
	a.SelectPane("p4")
	if v := a.View(); v.WorkspaceID != "w2" || v.TabID != "c" {
		t.Fatalf("pane must select its scope: %+v", v)
	}
}

func TestClosedTabSelectsAdjacent(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, `[{"tab_id":"a","workspace_id":"w1"},{"tab_id":"b","workspace_id":"w1"},{"tab_id":"d","workspace_id":"w1"}]`, `[]`)})
	a.SelectTab("b")
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, `[{"tab_id":"a","workspace_id":"w1","focused":true},{"tab_id":"d","workspace_id":"w1"}]`, `[]`)})
	if a.View().TabID != "d" {
		t.Fatal("closed middle tab should choose next")
	}
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, `[{"tab_id":"a","workspace_id":"w1"}]`, `[]`)})
	if a.View().TabID != "a" {
		t.Fatal("closed last tab should choose previous")
	}
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, `[]`, `[]`)})
	if v := a.View(); v.TabID != "" || v.PaneID != "" {
		t.Fatalf("closed all: %+v", v)
	}
}

func TestRowsKeepPaneIdentityAndRealPathsAndCopies(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes)})
	v := a.View()
	if len(v.Rows[0].Panes) != 3 || v.Rows[0].Panes[0].Status != "Working" || v.Rows[0].Panes[1].Status != "Idle" {
		t.Fatalf("same-name agents conflated: %+v", v.Rows[0])
	}
	if v.Path != "/real/one" || v.Rows[1].Path != "" || v.Tabs[0].Name != "1" || v.Tabs[1].PaneCount != 1 {
		t.Fatalf("projection: %+v", v)
	}
	v.Rows[0].Panes[0].Title = "corrupt"
	v.Tabs[0].Name = "corrupt"
	v.Panes[0].Title = "corrupt"
	if fresh := a.View(); fresh.Rows[0].Panes[0].Title != "First" || fresh.Tabs[0].Name != "1" || fresh.Panes[0].Title != "First" {
		t.Fatal("View aliases returned data")
	}
	a.SelectPane("p2")
	if a.View().Path != "" {
		t.Fatal("missing selected pane cwd must remain empty")
	}
}

func TestGroupingRequiresUniqueExplicitEvidence(t *testing.T) {
	ws := `[{"workspace_id":"child","worktree":{"checkout_path":"/elsewhere","repo_root":"/repo","repo_key":"key","is_linked_worktree":true}},{"workspace_id":"root","worktree":{"checkout_path":"/repo","repo_key":"key"}},{"workspace_id":"guess","worktree":{"checkout_path":"/repo/child","is_linked_worktree":true}},{"workspace_id":"wrong","worktree":{"repo_root":"/repo","repo_key":"other","is_linked_worktree":true}}]`
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, ws, `[]`, `[]`)})
	r := a.View().Rows
	if len(r) != 4 || r[0].ID != "root" || r[1].ID != "child" || r[1].Depth != 1 || r[2].Depth != 0 || r[3].Depth != 0 || r[1].Path != "/elsewhere" {
		t.Fatalf("grouping: %+v", r)
	}
	ws = `[{"workspace_id":"one","worktree":{"checkout_path":"/repo"}},{"workspace_id":"two","worktree":{"repo_root":"/repo"}},{"workspace_id":"child","worktree":{"repo_root":"/repo","is_linked_worktree":true}}]`
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, ws, `[]`, `[]`)})
	if a.View().Rows[2].Depth != 0 {
		t.Fatal("ambiguous parent must stay root")
	}
}

func TestStalePublicationPreservesDataAndStatus(t *testing.T) {
	a := New()
	updated := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes), State: session.Ready, Revision: 4, Updated: updated})
	a.Update(session.PublishedSnapshot{State: session.Reconnecting, Stale: true, Error: "disconnected", Revision: 4, Updated: updated})
	v := a.View()
	if len(v.Rows) != 2 || !v.Stale || v.State != session.Reconnecting || v.Error != "disconnected" || v.Revision != 4 || v.Rows[0].Status != "Working" {
		t.Fatalf("stale view: %+v", v)
	}
	if !v.Updated.Equal(updated) {
		t.Fatalf("last sync timestamp: %v", v.Updated)
	}
}

func TestScopedFocusFallbackAndInvalidSelections(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes, "w2", "c", "p4")})
	if v := a.View(); v.WorkspaceID != "w2" || v.TabID != "c" || v.PaneID != "p4" {
		t.Fatalf("initial authoritative focus: %+v", v)
	}
	a.SelectWorkspace("w1")
	if v := a.View(); v.TabID != "a" || v.PaneID != "p1" {
		t.Fatalf("other workspace must use scoped fallback: %+v", v)
	}
	a.SelectWorkspace("missing")
	a.SelectTab("c")
	a.SelectPane("missing")
	if v := a.View(); v.WorkspaceID != "w1" || v.TabID != "a" || v.PaneID != "p1" {
		t.Fatalf("invalid selections must be ignored: %+v", v)
	}
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, `[]`, `[]`, `[]`)})
	if v := a.View(); v.WorkspaceID != "" || v.TabID != "" || v.PaneID != "" || len(v.Rows) != 0 {
		t.Fatalf("removed selections must clear: %+v", v)
	}
}

func TestExplicitTargetChangeDropsOldData(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{TargetID: 1, Snapshot: snapshot(t, workspaces, tabs, panes)})
	a.Update(session.PublishedSnapshot{TargetID: 2, State: session.Connecting, Stale: true})
	if v := a.View(); len(v.Rows) != 0 || v.WorkspaceID != "" || v.TabID != "" || v.PaneID != "" {
		t.Fatalf("new target retained old target data: %+v", v)
	}
}

func TestTabRowsCarryAggregatedAgentStatus(t *testing.T) {
	a := New()
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1"), State: session.Ready})
	v := a.View()
	if v.Tabs[0].Status != "Working" || v.Tabs[1].Status != "" {
		t.Fatalf("tab agent status: %+v", v.Tabs)
	}
}

func TestAgentStatusBlockedOutranksWorking(t *testing.T) {
	panes := []session.Pane{{Agent: "a", Status: "Working"}, {Agent: "b", Status: "Blocked"}, {Agent: "c", Status: "unknown"}}
	if got := agentStatus(panes); got != "Blocked" {
		t.Fatalf("blocked must outrank working: %q", got)
	}
	if got := agentStatus([]session.Pane{{Agent: "a", Status: "Future"}}); got != "" {
		t.Fatalf("unknown statuses must not show a state: %q", got)
	}
}

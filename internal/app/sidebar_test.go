package app

import (
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/Maidang1/herdr-ui/internal/sidebarinfo"
	"testing"
)

func TestSidebarSeparatesRealAgentsAndWorkspaceDetails(t *testing.T) {
	a := New()
	p := session.PublishedSnapshot{TargetID: 8, State: session.Ready, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")}
	a.Update(p)
	source := sidebarinfo.BuildSource(p)
	revision := a.ViewRevision()
	a.UpdateSidebar(sidebarinfo.Publication{TargetID: 8, Entries: map[string]sidebarinfo.Entry{"w1": {Fingerprint: sidebarinfo.Fingerprint(source.Workspaces[0]), Info: sidebarinfo.Info{Path: "/real/one", Branch: "feat/sidebar", Remote: "github", Ahead: 13, Behind: 11, GitKnown: true, PortsKnown: true, Ports: []int{8080, 8081}}}}})
	if a.ViewRevision() != revision {
		t.Fatal("metadata refresh changed selection intent revision")
	}
	v := a.View()
	if v.Rows[0].Branch != "feat/sidebar" || v.Rows[0].Ahead != 13 || len(v.Rows[0].Ports) != 2 {
		t.Fatal("metadata lost", v.Rows[0])
	}
	if len(v.Agents) != 2 || v.Agents[0].PaneID != "p1" || v.Agents[1].PaneID != "p2" {
		t.Fatal("shells included or same-name agents merged", v.Agents)
	}
	v.Rows[0].Ports[0] = 1
	if a.View().Rows[0].Ports[0] != 8080 {
		t.Fatal("view leaked metadata slices")
	}
	a.SelectPane(v.Agents[1].PaneID)
	if a.View().PaneID != "p2" {
		t.Fatal("agent identity selection failed")
	}
}

func TestSidebarRejectsLateMetadataForReplacedTerminalAndTarget(t *testing.T) {
	a := New()
	p := session.PublishedSnapshot{TargetID: 1, State: session.Ready, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")}
	a.Update(p)
	old := sidebarinfo.BuildSource(p)
	publication := sidebarinfo.Publication{TargetID: 1, Entries: map[string]sidebarinfo.Entry{"w1": {Fingerprint: sidebarinfo.Fingerprint(old.Workspaces[0]), Info: sidebarinfo.Info{Branch: "old", GitKnown: true}}}}
	a.UpdateSidebar(publication)
	p.Snapshot = snapshot(t, workspaces, tabs, `[{"pane_id":"p1","terminal_id":"replacement","workspace_id":"w1","tab_id":"a","cwd":"/real/one"}]`, "w1", "a", "p1")
	a.Update(p)
	a.UpdateSidebar(publication)
	if a.View().Rows[0].Branch != "" {
		t.Fatal("old terminal metadata survived replacement")
	}
	p.TargetID = 2
	a.Update(p)
	a.UpdateSidebar(publication)
	if a.View().Rows[0].Branch != "" {
		t.Fatal("old target metadata applied")
	}
}

func TestWorkspaceStatusIsAggregateAndDoesNotFollowLocalShell(t *testing.T) {
	a := New()
	ws := `[{"workspace_id":"w1","label":"One","agent_status":"working"}]`
	ps := `[{"pane_id":"shell","terminal_id":"term-shell","workspace_id":"w1","tab_id":"a","agent_status":"unknown"},{"pane_id":"agent","terminal_id":"term-agent","workspace_id":"w1","tab_id":"a","agent":"codex","agent_status":"working"}]`
	a.Update(session.PublishedSnapshot{Snapshot: snapshot(t, ws, `[{"workspace_id":"w1","tab_id":"a"}]`, ps, "w1", "a", "shell")})
	if a.View().Rows[0].Status != "working" || len(a.View().Agents) != 1 {
		t.Fatal("selected shell hid workspace agent state")
	}
}

func TestSidebarTargetChangeClearsExistingMetadataEvenWithSameWorkspaceIDs(t *testing.T) {
	a := New()
	p := session.PublishedSnapshot{TargetID: 1, State: session.Ready, Snapshot: snapshot(t, workspaces, tabs, panes, "w1", "a", "p1")}
	a.Update(p)
	src := sidebarinfo.BuildSource(p).Workspaces[0]
	a.UpdateSidebar(sidebarinfo.Publication{TargetID: 1, Entries: map[string]sidebarinfo.Entry{"w1": {Fingerprint: sidebarinfo.Fingerprint(src), Info: sidebarinfo.Info{Branch: "old-target", GitKnown: true}}}})
	if a.View().Rows[0].Branch == "" {
		t.Fatal("fixture metadata not installed")
	}
	p.TargetID = 2
	a.Update(p)
	if a.View().Rows[0].Branch != "" {
		t.Fatal("metadata crossed target identity")
	}
}

package ui

import (
	"testing"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

func TestThePaletteKeepsTheCommandsThatMatch(t *testing.T) {
	snapshot := &herdr.Snapshot{
		Commands: []herdr.Command{
			{CommandID: "cmd-1", Description: "hunk: review changes", BindingLabel: "prefix+shift+h"},
			{CommandID: "cmd-2", Description: "workspace: new"},
		},
	}
	all := filterCommands(snapshot, "")
	if len(all) != 2 {
		t.Fatalf("the palette kept %d commands", len(all))
	}
	matched := filterCommands(snapshot, "review")
	if len(matched) != 1 || matched[0].CommandID != "cmd-1" {
		t.Fatalf("the palette kept %+v", matched)
	}
	if len(filterCommands(snapshot, "nothing here")) != 0 {
		t.Fatal("the palette kept commands that do not match")
	}
}

func TestThePaneUnderThePointer(t *testing.T) {
	frame := &herdr.SurfaceFrame{
		Panes: []herdr.PaneSurface{
			{
				PaneID:    "w1:p1",
				Rect:      herdr.SurfaceRect{X: 0, Y: 0, Width: 80, Height: 24},
				InnerRect: herdr.SurfaceRect{X: 0, Y: 1, Width: 80, Height: 23},
			},
			{
				PaneID:    "w1:p2",
				Rect:      herdr.SurfaceRect{X: 80, Y: 0, Width: 80, Height: 24},
				InnerRect: herdr.SurfaceRect{X: 80, Y: 1, Width: 80, Height: 23},
			},
		},
	}
	cellWidth := float32(8)
	cellHeight := float32(18)

	// The content of the first pane.
	pane, ok := paneAt(frame, 4*cellWidth, 5*cellHeight, cellWidth, 18)
	if !ok || pane != "w1:p1" {
		t.Fatalf("the pointer found %q", pane)
	}
	// The content of the second pane.
	pane, ok = paneAt(frame, 100*cellWidth, 5*cellHeight, cellWidth, 18)
	if !ok || pane != "w1:p2" {
		t.Fatalf("the pointer found %q", pane)
	}
	// The strip above a pane holds no content.
	if _, ok := paneAt(frame, 4*cellWidth, 0, cellWidth, 18); ok {
		t.Fatal("the pointer found a pane in the title strip")
	}
	// Beyond the panes there is nothing.
	if _, ok := paneAt(frame, 500*cellWidth, 5*cellHeight, cellWidth, 18); ok {
		t.Fatal("the pointer found a pane that is not there")
	}
}

func TestTheTitleOfAPaneNamesItsAgent(t *testing.T) {
	snapshot := &herdr.Snapshot{
		Agents: []herdr.Agent{
			{PaneID: "w1:p1", Agent: stringPtr("grok"), TerminalTitle: stringPtr("herdr — index"), AgentStatus: "idle"},
		},
		Panes: []herdr.Pane{
			{PaneID: "w1:p2", CWD: stringPtr("/tmp/work")},
		},
	}
	app := &App{}
	title := app.paneTitle("w1:p1", snapshot)
	if title != "grok · herdr — index" {
		t.Fatalf("the title came out as %q", title)
	}
	title = app.paneTitle("w1:p2", snapshot)
	if title != "/tmp/work" {
		t.Fatalf("the title came out as %q", title)
	}
}

func TestTheAgentsFollowTheOrderOfTheDaemon(t *testing.T) {
	snapshot := &herdr.Snapshot{
		AgentOrder: []string{"w1:p2", "w1:p1"},
		Agents: []herdr.Agent{
			{PaneID: "w1:p1", Agent: stringPtr("grok")},
			{PaneID: "w1:p2", Agent: stringPtr("claude")},
		},
	}
	agents := agentRows(snapshot)
	if len(agents) != 2 {
		t.Fatalf("the agents came back as %+v", agents)
	}
	if agents[0].PaneID != "w1:p2" || agents[1].PaneID != "w1:p1" {
		t.Fatalf("the agents came back as %+v", agents)
	}
}

func TestTheWindowShowsWhatTheDaemonAnnounces(t *testing.T) {
	app := testApp()
	app.snapshot.ProductAnnouncement = &herdr.Announcement{
		ID:    "a-1",
		Title: "Herdr 0.9.5",
		Body:  "The hotfix is here.",
	}
	tester := ui.NewTester(app.view, 1100, 700)
	// The announcement no longer takes the top of the window: the mark at
	// the top right says that there is something to read.
	if !tester.HasText("What the daemon has to say") {
		t.Errorf("the window does not offer the announcement; it shows %q", tester.Texts())
	}
	if tester.HasText("The hotfix is here.") {
		t.Errorf("the window shows the announcement without being asked; it shows %q", tester.Texts())
	}
	if err := tester.Click("What the daemon has to say"); err != nil {
		t.Fatalf("the mark of the session: %v", err)
	}
	tester.Frame()
	if !tester.HasText("The hotfix is here.") {
		t.Errorf("the window does not show the announcement; it shows %q", tester.Texts())
	}
}

func TestTheWindowShowsTheAgents(t *testing.T) {
	app := testApp()
	app.snapshot.Agents = []herdr.Agent{
		{PaneID: "w1:p1", Agent: stringPtr("grok"), AgentStatus: "working"},
	}
	tester := ui.NewTester(app.view, 1100, 700)
	if !tester.HasText("grok (working)") {
		t.Errorf("the window does not show the agent; it shows %q", tester.Texts())
	}
}

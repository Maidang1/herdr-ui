package ui

import (
	"testing"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The window is drawn without a screen here: the tester lays the view out
// in memory, so the test says what the user would see.
func testApp() *App {
	app := &App{}
	app.snapshot = &herdr.Snapshot{
		BootID:             "boot-1",
		FocusedWorkspaceID: stringPtr("w1"),
		FocusedTabID:       stringPtr("w1:t1"),
		FocusedPaneID:      stringPtr("w1:p1"),
		Workspaces: []herdr.Workspace{
			{WorkspaceID: "w1", Number: 1, Label: "dsh", ActiveTabID: "w1:t1"},
			{WorkspaceID: "w2", Number: 2, Label: "coze", ActiveTabID: "w2:t1"},
		},
		Tabs: []herdr.Tab{
			{TabID: "w1:t1", WorkspaceID: "w1", Number: 1, Label: "1"},
			{TabID: "w1:t2", WorkspaceID: "w1", Number: 2, Label: "2"},
		},
		Panes: []herdr.Pane{
			{PaneID: "w1:p1", WorkspaceID: "w1", TabID: "w1:t1", CWD: stringPtr("/tmp/work"), Focused: true},
		},
		Agents: []herdr.Agent{
			{PaneID: "w1:p1", WorkspaceID: "w1", TabID: "w1:t1", Agent: stringPtr("grok"), AgentStatus: "idle", TerminalTitle: stringPtr("grok — herdr")},
		},
	}
	app.workspace = "w1"
	app.tab = "w1:t1"
	return app
}

func stringPtr(value string) *string { return &value }

func TestTheWindowShowsTheSession(t *testing.T) {
	app := testApp()
	tester := ui.NewTester(app.view, 1100, 700)

	for _, want := range []string{"dsh", "coze", "/tmp/work"} {
		if !tester.HasText(want) {
			t.Errorf("the window does not show %q; it shows %q", want, tester.Texts())
		}
	}
}

func TestTheWindowShowsTheTabsOfTheFocusedWorkspace(t *testing.T) {
	app := testApp()
	tester := ui.NewTester(app.view, 1100, 700)

	if !tester.HasText("1") || !tester.HasText("2") {
		t.Errorf("the window does not show the tabs; it shows %q", tester.Texts())
	}
}

func TestTheWindowWaitsForTheDaemon(t *testing.T) {
	app := &App{}
	tester := ui.NewTester(app.view, 800, 600)
	if !tester.HasText("Waiting for the Herdr daemon…") {
		t.Errorf("the window does not wait for the daemon; it shows %q", tester.Texts())
	}
}

func TestTheControlStatusFollowsTheSnapshot(t *testing.T) {
	app := testApp()
	status := app.ControlStatus()
	if status.BootID != "boot-1" {
		t.Fatalf("the boot id came back as %q", status.BootID)
	}
	if len(status.Workspaces) != 2 || status.Workspaces[0].Label != "dsh" {
		t.Fatalf("the workspaces came back as %+v", status.Workspaces)
	}
	if len(status.Tabs) != 2 {
		t.Fatalf("the tabs came back as %+v", status.Tabs)
	}
	if len(status.Panes) != 1 || status.Panes[0].CWD != "/tmp/work" {
		t.Fatalf("the panes came back as %+v", status.Panes)
	}
}

func TestTheUpdatePanelShowsTheReleaseNotes(t *testing.T) {
	app := testApp()
	app.snapshot.ReleaseNotes = &herdr.ReleaseNotes{
		Version: "9.9.9",
		Body:    "the window paints the terminals\n",
	}
	app.snapshot.UpdateInstallCommand = "herdr update"
	app.openUpdatePanel()

	tester := ui.NewTester(app.view, 1100, 700)
	for _, want := range []string{"Herdr 9.9.9", "the window paints the terminals", "herdr update"} {
		if !tester.HasText(want) {
			t.Errorf("the panel does not show %q; it shows %q", want, tester.Texts())
		}
	}
}

func TestTheWindowOfARemoteHostKeepsItsOwnSocket(t *testing.T) {
	socket := remoteSocketPath("/tmp/state/herdr-ui/control.sock", "user@host")
	if socket != "/tmp/state/herdr-ui/control-user-host.sock" {
		t.Fatalf("the socket came out as %q", socket)
	}
}

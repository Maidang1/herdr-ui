package ui

import (
	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The bar at the top of the window.
//
// It is thin, and it holds the one thing that has to be seen: the round
// mark at the right that says the daemon has something to say. The Rust
// client draws it the same way.

// drawTopBar shows the thin bar above the terminal.
func (a *App) drawTopBar(c *ui.Context, colors chrome, snapshot *herdr.Snapshot) {
	fontSize := a.uiFontSizePx()
	ui.Row(c).Height(30).Padding(0, 10).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Grow(1)

		// The mark of the session: it says whether there is something new.
		mark := a.sessionMark(snapshot)
		button := ui.Box(c).Width(28).Height(28).Radius(14).AlignItems(ui.Center).Justify(ui.Center).
			Label("What the daemon has to say").Role(ui.RoleButton)
		button.Children(func() {
			if mark == nil {
				return
			}
			ui.Icon(c, mark).FontSize(fontSize + 1).TextColor(colors.accent)
		})
		button.OnClick(func() {
			a.openUpdatePanel()
		})
	})
}

// sessionMark returns the mark of the session.
//
// The bell means the daemon has something to say, and the mark of the agent
// means it has nothing.
func (a *App) sessionMark(snapshot *herdr.Snapshot) *ui.SVG {
	if snapshot == nil {
		return nil
	}
	if snapshot.ProductAnnouncement != nil ||
		(snapshot.UpdateAvailable != nil && *snapshot.UpdateAvailable != "") ||
		snapshot.LatestReleaseNotes {
		return icon("bell")
	}
	if pane, ok := a.targetPane(snapshot); ok {
		if agent, ok := snapshot.AgentOf(pane.PaneID); ok {
			return agentIcon(agentName(agent))
		}
	}
	return icon("user")
}

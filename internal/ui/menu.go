package ui

import (
	"github.com/egoist/mygo/ui"
)

// The right-click menu of a pane holds what a pane can be asked to do,
// the way the multiplexer offers it.

// paneMenu builds the menu of the pane the user last aimed at.
func (a *App) paneMenu(m *ui.Menu) {
	paneID := a.selectedPaneID()
	if m.Item("Split right").Chosen() {
		a.splitPane(paneID, "right")
	}
	if m.Item("Split down").Chosen() {
		a.splitPane(paneID, "down")
	}
	if m.Item("Zoom").Chosen() {
		a.zoomPane(paneID)
	}
	m.Separator()
	if m.Item("Scroll up").Chosen() {
		a.scrollPane(paneID, 1)
	}
	if m.Item("Scroll down").Chosen() {
		a.scrollPane(paneID, -1)
	}
	m.Separator()
	if m.Item("Copy mode").Chosen() {
		a.focusPane(paneID)
		a.enterCopyMode()
	}
	if m.Item("Focus").Chosen() {
		a.focusPane(paneID)
	}
	if m.Item("Close pane").Chosen() {
		a.closePane(paneID)
	}
}

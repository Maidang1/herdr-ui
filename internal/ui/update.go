package ui

import (
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The panel that tells what the daemon has to say about the version: the
// notes of the release, and the command that installs the update when one
// is offered.
//
// The daemon owns the update; the window shows what it says and the command
// it gives, and leaves the installing to it.

const (
	// updatePanelWidth is how wide the panel is.
	updatePanelWidth = 620
	// updatePanelHeight is how tall the panel is.
	updatePanelHeight = 420
)

// drawUpdatePanel shows the notes of the release, and the command that
// installs the update.
func (a *App) drawUpdatePanel(c *ui.Context, snapshot *herdr.Snapshot) {
	a.mu.Lock()
	open := a.updatePanel
	a.mu.Unlock()
	if !open || snapshot == nil {
		return
	}
	theme := c.Theme()

	version := ""
	notes := ""
	command := snapshot.UpdateInstallCommand
	switch {
	case snapshot.ProductAnnouncement != nil:
		// The announcement of the daemon comes before the notes of the
		// release: it is the newer word.
		version = snapshot.ProductAnnouncement.Title
		notes = snapshot.ProductAnnouncement.Body
	case snapshot.ReleaseNotes != nil:
		version = snapshot.ReleaseNotes.Version
		notes = snapshot.ReleaseNotes.Body
	}
	update := ""
	if snapshot.UpdateAvailable != nil {
		update = *snapshot.UpdateAvailable
	}

	ui.Modal(c, &a.updatePanel, func() {
		ui.Column(c).Width(updatePanelWidth).MaxWidth(updatePanelWidth).
			MaxHeight(updatePanelHeight).Padding(16).Gap(10).
			Background(theme.Surface).Border(1, theme.Border).Radius(10).Children(func() {
			title := "Herdr"
			if version != "" {
				title = "Herdr " + version
			}
			ui.Text(c, title).Bold()
			if update != "" {
				ui.Text(c, "An update is available: "+update).FontSize(12).TextColor(theme.Accent)
			}
			if notes != "" {
				ui.Column(c).MaxHeight(260).Gap(4).Children(func() {
					for _, line := range strings.Split(notes, "\n") {
						if strings.TrimSpace(line) == "" {
							continue
						}
						ui.Text(c, line).FontSize(12).TextColor(theme.TextMuted)
					}
				})
			} else {
				ui.Text(c, "There are no release notes to show.").FontSize(12).TextColor(theme.TextMuted)
			}
			if command != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, command).FontSize(12)
					if ui.Button(c, "Copy").Label("Copy the update command").Clicked() {
						a.copyText(command)
					}
				})
			}
			ui.Row(c).Grow(1)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "Close").Clicked() {
					a.closeUpdatePanel()
				}
			})
		})
	})
}

// openUpdatePanel opens the panel that tells about the version.
func (a *App) openUpdatePanel() {
	a.mu.Lock()
	a.updatePanel = true
	a.mu.Unlock()
	a.invalidate()
}

// closeUpdatePanel closes the panel.
func (a *App) closeUpdatePanel() {
	a.mu.Lock()
	a.updatePanel = false
	a.mu.Unlock()
	a.invalidate()
}

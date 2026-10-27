package ui

import (
	"time"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

// UpdateSettings is read from the updater so its window and Settings always
// reflect the same persisted preferences and last successful check.
type UpdateSettings struct {
	Version                             string
	Enabled                             bool
	AutomaticChecks, AutomaticDownloads bool
	LastCheck                           time.Time
}

func (w *Window) settingsUpdates(c *native.Context, p design.Palette) {
	var s UpdateSettings
	if w.hooks.UpdateSettings != nil {
		s = w.hooks.UpdateSettings()
	}
	w.settingsSection(c, "settings-updates", "Software updates", "Updates are checked against a signature before installation. Your Herdr sessions keep running while Herdr UI restarts.", p, func(g *settingsGroup) {
		version := s.Version
		if version == "" {
			version = "Development build"
		}
		g.row("Herdr UI", version, func() {
			if native.Button(c, "Check for Updates…").Label("Check for Updates").Clicked() && w.hooks.CheckForUpdates != nil {
				w.hooks.CheckForUpdates()
			}
		})
		if !s.Enabled {
			g.row("Automatic updates unavailable", "Install a release build in a writable Applications folder to enable automatic updates.", func() {})
			return
		}
		if g.toggle("settings-update-checks", "Automatically check for updates", "Checks after launch and once a day while Herdr UI is running.", &s.AutomaticChecks).Changed() && w.hooks.AutomaticUpdateChecks != nil {
			w.hooks.AutomaticUpdateChecks(s.AutomaticChecks)
		}
		if g.toggle("settings-update-downloads", "Automatically download and install", "Updates found by automatic checks take effect the next time you open Herdr UI.", &s.AutomaticDownloads).Changed() && w.hooks.AutomaticUpdateDownloads != nil {
			w.hooks.AutomaticUpdateDownloads(s.AutomaticDownloads)
		}
		last := "Not checked yet"
		if !s.LastCheck.IsZero() {
			last = s.LastCheck.Local().Format("2006-01-02 15:04")
		}
		g.row("Last successful check", last, func() {})
	})
}

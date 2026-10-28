package ui

import (
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	native "github.com/egoist/mygo/ui"
)

type integrationSettings struct {
	entries             []herdr.Integration
	loaded, busy        bool
	fresh, refreshAgain bool
	error, result       string
}

// RefreshIntegrations invalidates the cached list when Settings is reopened.
// All state and completion callbacks belong to the UI thread.
func (w *Window) RefreshIntegrations() {
	s := &w.settings.integrations
	s.result = ""
	if s.busy {
		s.refreshAgain = true
	} else {
		s.loaded, s.fresh = false, false
	}
}

func (w *Window) loadIntegrations() {
	s := &w.settings.integrations
	if s.busy {
		return
	}
	s.loaded, s.busy, s.error = true, true, ""
	s.fresh, s.refreshAgain = false, false
	if w.hooks.Integrations == nil {
		s.busy, s.error = false, "Connect to Herdr to manage agent integrations."
		return
	}
	w.hooks.Integrations(func(entries []herdr.Integration, err error) {
		s.busy = false
		if s.refreshAgain {
			w.loadIntegrations()
			return
		}
		if err != nil {
			s.error = "Could not refresh integrations. " + err.Error()
			return
		}
		s.entries = append([]herdr.Integration(nil), entries...)
		s.fresh = true
	})
}

func (w *Window) changeIntegration(entry herdr.Integration, action string) {
	s := &w.settings.integrations
	if s.busy || !s.fresh || w.hooks.IntegrationChange == nil {
		return
	}
	s.busy, s.error, s.result = true, "", ""
	s.fresh = false
	w.hooks.IntegrationChange(action, entry.Target, func(messages []string, err error) {
		s.busy = false
		if err != nil {
			s.error = "Could not change " + entry.Label + " integration. " + err.Error()
			return
		}
		s.result = strings.Join(messages, "\n")
		if s.result == "" {
			s.result = entry.Label + " integration installed."
			if action == "uninstall" {
				s.result = entry.Label + " integration removed."
			}
		}
		w.loadIntegrations()
	})
}

func (w *Window) settingsIntegrations(c *native.Context, p design.Palette) {
	s := &w.settings.integrations
	if !s.loaded {
		w.loadIntegrations()
	}
	w.settingsSection(c, "settings-integrations", "Agent integrations", "Enable session restore and, where supported, direct agent status updates. Installations are managed by Herdr for the agents listed below.", p, func(g *settingsGroup) {
		g.row("Integration status", "Refresh after installing or updating an agent.", func() {
			if native.Button(c, "Refresh").Label("Refresh integrations").Disabled(s.busy).Clicked() {
				s.result = ""
				w.loadIntegrations()
			}
		})
		for _, entry := range s.entries {
			detail, action, label := integrationPresentation(entry)
			if !s.fresh && !s.busy {
				detail += " · Refresh to verify"
			}
			g.row(entry.Label, detail, func() {
				if action == "" {
					return
				}
				if native.Button(c.Key("integration:"+entry.Target), label).Label(label + " " + entry.Label + " integration").Disabled(s.busy || !s.fresh || w.hooks.IntegrationChange == nil).Clicked() {
					w.changeIntegration(entry, action)
				}
				if entry.State == "outdated" {
					if native.Button(c.Key("integration-remove:"+entry.Target), "Remove").Label("Remove " + entry.Label + " integration").Disabled(s.busy || !s.fresh || w.hooks.IntegrationChange == nil).Clicked() {
						w.changeIntegration(entry, "uninstall")
					}
				}
			})
		}
	})
	if s.busy {
		native.Text(c, "Updating integrations…").FontSize(12).TextColor(p.Secondary)
	} else if len(s.entries) == 0 && s.error == "" {
		native.Text(c, "Herdr did not report any agent integrations.").FontSize(12).TextColor(p.Secondary)
	}
	if s.error != "" {
		native.Text(c, s.error).FontSize(12).TextColor(p.Danger).FillWidth()
	}
	if s.result != "" {
		native.Text(c, s.result).FontSize(12).TextColor(p.Secondary).FillWidth()
	}
}

func integrationPresentation(entry herdr.Integration) (detail, action, label string) {
	switch entry.State {
	case "current":
		detail, action, label = "Installed · Up to date", "uninstall", "Remove"
	case "outdated":
		detail, action, label = "Update or repair available", "install", "Update"
	case "not_installed":
		detail = "Not installed"
		if entry.Available {
			action, label = "install", "Install"
		}
	default:
		detail = "Unknown integration status · Update Herdr and refresh"
	}
	if !entry.Available {
		detail += " · Agent command not found"
	}
	return
}

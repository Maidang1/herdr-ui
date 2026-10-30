package ui

import (
	"fmt"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

type pluginSettings struct {
	loaded, busy, fresh bool
	catalog             herdr.PluginCatalog
	message, source     string
	expanded            map[string]bool
}
type restoreSettingsUI struct {
	loaded, busy bool
	value        herdr.RestoreSettings
	message      string
}

func (w *Window) loadPlugins() {
	s := &w.settings.plugins
	if s.busy {
		return
	}
	s.loaded = true
	s.busy = true
	s.fresh = false
	s.message = ""
	if w.hooks.Plugins == nil {
		s.busy = false
		s.message = "Plugin management is unavailable."
		return
	}
	w.hooks.Plugins(func(v herdr.PluginCatalog, err error) {
		s.busy = false
		if err != nil {
			s.message = err.Error()
			return
		}
		s.catalog = v
		s.fresh = true
	})
}
func (w *Window) pluginChange(run func(func(error))) {
	s := &w.settings.plugins
	if s.busy || !s.fresh {
		return
	}
	s.busy = true
	s.fresh = false
	s.message = ""
	run(func(err error) {
		s.busy = false
		if err != nil {
			s.message = "The action may have completed. Refresh before repeating. " + err.Error()
			return
		}
		w.loadPlugins()
	})
}
func (w *Window) settingsPlugins(c *native.Context, p design.Palette) {
	s := &w.settings.plugins
	if !s.loaded {
		w.loadPlugins()
	}
	w.settingsSection(c, "settings-plugins", "Installed plugins", "Actions execute plugin commands. Enable only plugins you trust. Popup and overlay entries require Herdr's terminal client.", p, func(g *settingsGroup) {
		g.row("Plugin catalog", "Refresh to read current status, actions and the last 30 logs.", func() {
			if native.Button(c, "Refresh").Disabled(s.busy).Clicked() {
				w.loadPlugins()
			}
		})
		g.row("GitHub plugin", "Review the installation in Herdr before accepting.", func() {
			native.TextInput(c.Key("plugin-source"), &s.source).Label("Plugin repository").Placeholder("owner/repo[/subdir]").Width(200)
		})
		g.row("Install and update", "Herdr asks for confirmation in a command surface.", func() {
			if native.Button(c, "Install…").Disabled(w.hooks.PluginInstall == nil || strings.TrimSpace(s.source) == "").Clicked() {
				if err := w.hooks.PluginInstall(s.source, false); err != nil {
					s.message = err.Error()
				}
			}
			if native.Button(c, "Update…").Disabled(w.hooks.PluginInstall == nil).Clicked() {
				if err := w.hooks.PluginInstall("", true); err != nil {
					s.message = err.Error()
				}
			}
		})
		for _, plugin := range s.catalog.Plugins {
			entry := plugin
			g.row(entry.Name+" · "+entry.Version, entry.Description+"\n"+strings.Join(entry.Warnings, "\n"), func() {
				label := "Enable"
				if entry.Enabled {
					label = "Disable"
				}
				if native.Button(c.Key("plugin-toggle:"+entry.ID), label).Disabled(s.busy || !s.fresh || w.hooks.PluginEnabled == nil).Clicked() {
					w.pluginChange(func(done func(error)) { w.hooks.PluginEnabled(entry.ID, !entry.Enabled, done) })
				}
			})
			if !entry.Enabled {
				continue
			}
			for _, action := range s.catalog.Actions {
				if action.PluginID != entry.ID {
					continue
				}
				a := action
				g.row(a.Title, a.Description, func() {
					op, ok := w.operation(session.ExportLayout, "")
					supported := len(a.Contexts) == 0
					for _, scope := range a.Contexts {
						if scope != "selection" {
							supported = true
						}
					}
					if native.Button(c.Key("plugin-action:"+entry.ID+":"+a.ID), "Run").Disabled(!ok || !supported || s.busy || !s.fresh || w.hooks.PluginAction == nil).Clicked() {
						w.pluginChange(func(done func(error)) { w.hooks.PluginAction(op, entry.ID, a.ID, done) })
					}
				})
			}
			for _, pane := range entry.Panes {
				pp := pane
				if pp.Placement != "tab" && pp.Placement != "split" {
					continue
				}
				g.row(pp.Title, "Opens a plugin "+pp.Placement+".", func() {
					op, ok := w.operation(session.ExportLayout, "")
					if native.Button(c.Key("plugin-pane:"+entry.ID+":"+pp.ID), "Open").Disabled(!ok || s.busy || !s.fresh || w.hooks.PluginPane == nil).Clicked() {
						w.pluginChange(func(done func(error)) { w.hooks.PluginPane(op, entry.ID, pp.ID, pp.Placement, done) })
					}
				})
			}
		}
	})
	if s.busy {
		native.Text(c, "Reading plugins…").TextColor(p.Secondary)
	}
	if s.message != "" {
		native.Text(c, s.message).TextColor(p.Danger)
	}
	if s.fresh && len(s.catalog.Plugins) == 0 {
		native.Text(c, "No plugins installed.").TextColor(p.Secondary)
	}
	w.settingsSection(c, "settings-plugin-logs", "Recent plugin commands", "Output can contain private information. Logs are read only when you refresh.", p, func(g *settingsGroup) {
		if len(s.catalog.Logs) == 0 {
			g.row("No recent commands", "", func() {})
		}
		for _, log := range s.catalog.Logs {
			l := log
			detail := strings.Join(l.Command, " ")
			if l.ExitCode != nil {
				detail += fmt.Sprintf(" · Exit %d", *l.ExitCode)
			}
			g.row(l.PluginID+" · "+l.Status, detail, func() {
				if native.Button(c.Key("plugin-log:"+l.ID), "Output").Clicked() {
					if s.expanded == nil {
						s.expanded = map[string]bool{}
					}
					s.expanded[l.ID] = !s.expanded[l.ID]
				}
			})
			if output := strings.TrimSpace(l.Stdout + "\n" + l.Stderr + "\n" + l.Error); output != "" && s.expanded[l.ID] {
				g.block(func() { native.Text(c, output).FontSize(11).FillWidth() })
			}
		}
	})
}
func (w *Window) loadRestore() {
	s := &w.settings.restore
	if s.busy {
		return
	}
	s.loaded = false
	s.busy = true
	s.message = ""
	if w.hooks.RestoreSettings == nil {
		s.busy = false
		s.message = "Herdr configuration is unavailable."
		return
	}
	w.hooks.RestoreSettings(func(v herdr.RestoreSettings, err error) {
		s.busy = false
		if err != nil {
			s.loaded = false
			s.message = err.Error()
			return
		}
		s.value = v
		s.loaded = true
	})
}
func (w *Window) saveRestore() {
	s := &w.settings.restore
	if s.busy || w.hooks.SaveRestoreSettings == nil {
		return
	}
	s.busy = true
	w.hooks.SaveRestoreSettings(s.value, func(err error) {
		s.busy = false
		if err != nil {
			s.message = err.Error() + " Refresh before changing these settings again."
			s.loaded = false
			return
		}
		s.message = "Saved to Herdr configuration. Applies the next time Herdr starts."
	})
}
func (w *Window) settingsRestore(c *native.Context, p design.Palette) {
	s := &w.settings.restore
	if !s.loaded && !s.busy && s.message == "" {
		w.loadRestore()
	}
	w.settingsSection(c, "settings-restore", "Session restore", "These settings apply the next time Herdr starts. Hergo never restarts Herdr automatically. Session references in Details indicate available metadata, not a guarantee that an agent can resume.", p, func(g *settingsGroup) {
		g.row("Herdr configuration", "Settings are saved to Herdr's config.toml.", func() {
			if native.Button(c, "Refresh").Disabled(s.busy).Clicked() {
				w.loadRestore()
			}
		})
		if s.loaded && !s.busy {
			if g.toggle("restore-agents", "Resume agents after restart", "Resume supported agents from their saved session references.", &s.value.ResumeAgents).Changed() {
				w.saveRestore()
			}
			if g.toggle("restore-history", "Save terminal history", "Terminal output may contain passwords, tokens and other private information.", &s.value.PaneHistory).Changed() {
				w.saveRestore()
			}
		}
	})
	if s.busy {
		native.Text(c, "Reading or saving configuration…").TextColor(p.Secondary)
	}
	if s.message != "" {
		native.Text(c, s.message).FontSize(12).TextColor(p.Secondary)
	}
}

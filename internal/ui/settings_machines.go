package ui

import (
	"fmt"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	native "github.com/egoist/mygo/ui"
)

type machineSettings struct {
	loaded, busy                        bool
	catalog                             herdr.MachineCatalog
	id, target, label, session, message string
}

func (w *Window) loadMachines() {
	s := &w.settings.machines
	if s.busy {
		return
	}
	s.loaded = true
	s.busy = true
	s.message = ""
	if w.hooks.Machines == nil {
		s.busy = false
		s.message = "Machine management is unavailable."
		return
	}
	w.hooks.Machines(func(catalog herdr.MachineCatalog, err error) {
		s.busy = false
		if err != nil {
			s.message = err.Error()
			return
		}
		s.catalog = catalog
	})
}
func (w *Window) machineCommand(command herdr.MachineCommand) {
	s := &w.settings.machines
	if w.hooks.MachineCommand == nil {
		return
	}
	if err := w.hooks.MachineCommand(command); err != nil {
		s.message = err.Error()
		return
	}
	s.message = "Opened in the main window. Review Herdr's prompts there, then refresh this page."
}
func (w *Window) settingsMachines(c *native.Context, p design.Palette) {
	s := &w.settings.machines
	if !s.loaded {
		w.loadMachines()
	}
	w.settingsSection(c, "settings-machines", "Saved machines", "Remote sessions open Herdr's interactive terminal client in the main window. This does not add remote workspaces to the native sidebar. Refresh after changes.", p, func(g *settingsGroup) {
		g.row("Machines and status", "Reads local saved profiles. Check status connects through SSH without prompting for authentication.", func() {
			if native.Button(c, "Refresh").Disabled(s.busy).Clicked() {
				w.loadMachines()
			}
			if native.Button(c, "Check status…").Disabled(w.hooks.MachineCommand == nil).Clicked() {
				w.machineCommand(herdr.MachineCommand{Action: "status"})
			}
		})
		for _, entry := range s.catalog.Machines {
			machine := entry
			status := "Status unavailable"
			for _, item := range s.catalog.Status {
				if item.ID == machine.ID {
					status = item.Status
					if item.Error != "" {
						status += " · " + item.Error
					}
					break
				}
			}
			g.row(fallback(machine.Label, machine.ID), machine.Target+" · "+fallback(machine.Session, "default")+" · "+status, func() {
				if native.Button(c.Key("machine-open:"+machine.ID), "Open…").Disabled(w.hooks.MachineCommand == nil || !machine.Enabled).Clicked() {
					w.machineCommand(herdr.MachineCommand{Action: "open", Value: machine.Target, Session: machine.Session})
				}
				action, label := "enable", "Enable"
				if machine.Enabled {
					action, label = "disable", "Disable"
				}
				if native.Button(c.Key("machine-enable:"+machine.ID), label).Disabled(w.hooks.MachineCommand == nil).Clicked() {
					w.machineCommand(herdr.MachineCommand{Action: action, ID: machine.ID})
				}
			})
			g.row("Profile actions", machine.ID, func() {
				if native.Button(c.Key("machine-rename:"+machine.ID), "Rename…").Disabled(w.hooks.MachineCommand == nil).Clicked() {
					s.id = machine.ID
					s.label = machine.Label
				}
				if native.Button(c.Key("machine-reconnect:"+machine.ID), "Reconnect…").Disabled(w.hooks.MachineCommand == nil).Clicked() {
					w.machineCommand(herdr.MachineCommand{Action: "reconnect", ID: machine.ID})
				}
				if native.Button(c.Key("machine-remove:"+machine.ID), "Remove…").Disabled(w.hooks.MachineCommand == nil).Clicked() {
					w.machineCommand(herdr.MachineCommand{Action: "remove", ID: machine.ID})
				}
			})
		}
		if len(s.catalog.Machines) == 0 {
			g.row("No saved machines", "Add an SSH target below.", func() {})
		}
		if s.catalog.StatusError != "" {
			g.block(func() { native.Text(c, s.catalog.StatusError).TextColor(p.Secondary) })
		}
		g.row("Rename profile", "Select Rename on a profile, then edit its label here.", func() {
			native.TextInput(c.Key("machine-label"), &s.label).Label("New machine label").Width(140)
			if native.Button(c, "Save name").Disabled(s.id == "" || s.label == "" || w.hooks.MachineCommand == nil).Clicked() {
				w.machineCommand(herdr.MachineCommand{Action: "rename", ID: s.id, Value: s.label})
			}
		})
	})
	w.settingsSection(c, "settings-remote-open", "Connect through SSH", "Herdr handles installation, authentication and confirmation in its terminal client. Enter an SSH target or open a saved machine above.", p, func(g *settingsGroup) {
		g.row("SSH target", "For example user@host or an SSH config host.", func() { native.TextInput(c.Key("machine-target"), &s.target).Label("SSH target").Width(210) })
		g.row("Remote session", "Leave empty for the default session.", func() { native.TextInput(c.Key("machine-session"), &s.session).Label("Remote session name").Width(210) })
		g.row("Connection", "Review prompts in the main window.", func() {
			if native.Button(c, "Add machine…").Disabled(s.target == "" || w.hooks.MachineCommand == nil).Clicked() {
				w.machineCommand(herdr.MachineCommand{Action: "add", Value: s.target, Session: s.session})
			}
			if native.Button(c, "Open…").Disabled(s.target == "" || w.hooks.MachineCommand == nil).Clicked() {
				w.machineCommand(herdr.MachineCommand{Action: "open", Value: s.target, Session: s.session})
			}
		})
	})
	w.settingsSection(c, "settings-local-sessions", "Local sessions", "Opens Herdr's interactive client. Existing native terminal attachments remain owned by this window; close the command surface to return.", p, func(g *settingsGroup) {
		if len(s.catalog.Sessions) == 0 {
			g.row("No sessions reported", "", func() {})
		}
		for _, entry := range s.catalog.Sessions {
			e := entry
			g.row(e.Name, fmt.Sprintf("Running: %t · Default: %t", e.Running, e.Default), func() {
				if native.Button(c.Key("session-open:"+e.Name), "Open…").Disabled(w.hooks.MachineCommand == nil).Clicked() {
					w.machineCommand(herdr.MachineCommand{Action: "session", Value: e.Name})
				}
			})
		}
	})
	if s.busy {
		native.Text(c, "Reading machines and sessions…").TextColor(p.Secondary)
	}
	if s.message != "" {
		native.Text(c, s.message).FontSize(12).TextColor(p.Secondary)
	}
}

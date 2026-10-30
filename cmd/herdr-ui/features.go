package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
	view "github.com/Maidang1/herdr-ui/internal/ui"
)

func featureIdentity(s *session.Session, op session.Operation) bool {
	p := s.Latest()
	if p.State != session.Ready || p.Stale || p.TargetID != op.TargetID || p.Epoch != op.Epoch || p.Snapshot == nil {
		return false
	}
	pane, ok := p.Snapshot.Pane(op.PaneID)
	return ok && pane.TerminalID == op.TerminalID && pane.TabID == op.TabID && pane.WorkspaceID == op.WorkspaceID
}
func installFeatureHooks(h *view.ActionHooks, ctx context.Context, s *session.Session, target herdr.Target, screen *view.Window, post func(func()), binary string) {
	stale := func() error { return fmt.Errorf("the terminal or connection changed; select it again") }
	h.PaneDetails = func(op session.Operation, done func(herdr.PaneDetails, error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(herdr.PaneDetails{}, stale()) })
				return
			}
			data, err := herdr.ReadPaneDetails(ctx, target, op.PaneID)
			post(func() {
				if !featureIdentity(s, op) {
					done(herdr.PaneDetails{}, stale())
					return
				}
				done(data, err)
			})
		}()
	}
	h.RenameAgent = func(op session.Operation, name *string, done func(error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(stale()) })
				return
			}
			err := herdr.RenameAgentTerminal(ctx, target, op.PaneID, op.TerminalID, name, func() bool { return featureIdentity(s, op) })
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.Plugins = func(done func(herdr.PluginCatalog, error)) {
		go func() { data, err := herdr.ReadPlugins(ctx, target); post(func() { done(data, err) }) }()
	}
	h.PluginEnabled = func(id string, on bool, done func(error)) {
		go func() { err := herdr.SetPluginEnabled(ctx, target, id, on); post(func() { done(err) }) }()
	}
	h.PluginAction = func(op session.Operation, plugin, action string, done func(error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(stale()) })
				return
			}
			err := herdr.InvokePluginAction(ctx, target, plugin, action, herdr.PluginContext{PaneID: op.PaneID, TabID: op.TabID, WorkspaceID: op.WorkspaceID, Source: "hergo"})
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.PluginPane = func(op session.Operation, plugin, entry, placement string, done func(error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(stale()) })
				return
			}
			err := herdr.OpenPluginPane(ctx, target, plugin, entry, placement, op.PaneID)
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.WatchOutput = func(op session.Operation, watch herdr.OutputWatch, done func(herdr.OutputMatch, error)) func() {
		call, cancel := context.WithCancel(ctx)
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(herdr.OutputMatch{}, stale()) })
				return
			}
			match, err := herdr.WaitForOutput(call, target, op.PaneID, watch)
			post(func() {
				if !featureIdentity(s, op) {
					done(herdr.OutputMatch{}, stale())
					return
				}
				done(match, err)
			})
		}()
		return cancel
	}
	h.RestoreSettings = func(done func(herdr.RestoreSettings, error)) {
		go func() { value, err := herdr.ReadRestoreSettings(configPath()); post(func() { done(value, err) }) }()
	}
	h.SaveRestoreSettings = func(value herdr.RestoreSettings, done func(error)) {
		go func() { err := herdr.SaveRestoreSettings(configPath(), value); post(func() { done(err) }) }()
	}
	h.Machines = func(done func(herdr.MachineCatalog, error)) {
		go func() {
			call, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()
			catalog, err := herdr.ReadMachineCatalog(call, binary)
			post(func() { done(catalog, err) })
		}()
	}
	h.MachineCommand = func(command herdr.MachineCommand) error {
		args, err := herdr.MachineArguments(command)
		if err != nil {
			return err
		}
		return screen.OpenReviewCommandSurface("Herdr machines and sessions", append([]string{binary}, args...), herdr.CatalogEnvironment(os.Environ()), "")
	}
	h.PluginInstall = func(source string, update bool) error {
		if !update && (strings.TrimSpace(source) == "" || strings.HasPrefix(source, "-") || strings.ContainsAny(source, "\x00\r\n")) {
			return fmt.Errorf("enter a GitHub repository such as owner/repo")
		}
		_, env, err := target.Attach(binary, "plugin-manager", os.Environ())
		if err != nil {
			return err
		}
		if update {
			return screen.OpenReviewCommandSurface("Update Herdr plugins", []string{binary, "plugin", "update"}, env, "")
		}
		return screen.OpenReviewCommandSurface("Install Herdr plugin", []string{binary, "plugin", "install", source}, env, "")
	}
}

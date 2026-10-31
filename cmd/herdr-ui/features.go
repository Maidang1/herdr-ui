package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/prefs"
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

// installExtraHooks wires the agent, layout and server requests. Mutating
// requests are sent once; a request that fails may still have happened.
func installExtraHooks(ctx context.Context, s *session.Session, target herdr.Target, post func(func()), settings *prefs.Store, install func() herdr.Installation, binary string) view.ExtraHooks {
	stale := func() error { return fmt.Errorf("the terminal or connection changed; select it again") }
	persist := func(fn func(*prefs.Settings)) {
		go func() {
			if _, err := settings.Update(fn); err != nil {
				log.Printf("prefs: %v", err)
			}
		}()
	}
	var h view.ExtraHooks
	h.AgentPrompt = func(op session.Operation, text string, done func(error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(stale()) })
				return
			}
			_, err := herdr.AgentPrompt(ctx, target, op.PaneID, text)
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.AgentWait = func(op session.Operation, until []string, timeout time.Duration, done func(herdr.AgentDetails, error)) func() {
		call, cancel := context.WithCancel(ctx)
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(herdr.AgentDetails{}, stale()) })
				return
			}
			details, err := herdr.AgentWait(call, target, op.PaneID, until, timeout)
			post(func() { done(details, err) })
		}()
		return cancel
	}
	h.AgentStart = func(op session.Operation, kind string, done func(error)) {
		go func() {
			if !featureIdentity(s, op) {
				post(func() { done(stale()) })
				return
			}
			_, err := herdr.AgentStart(ctx, target, op.PaneID, kind, "", nil)
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.AgentView = func(preset string, done func(error)) {
		persist(func(v *prefs.Settings) { v.AgentView = preset })
		go func() {
			err := herdr.SetAgentView(ctx, target, herdr.AgentViewPresetByID(preset))
			post(func() { done(err) })
		}()
	}
	h.LayoutNames = func() []string {
		var names []string
		for _, t := range settings.LayoutTemplates() {
			names = append(names, t.Name)
		}
		return names
	}
	h.SaveLayout = func(op session.Operation, name string, done func(error)) {
		go func() {
			template, err := herdr.ExportLayoutTemplate(ctx, target, op.TabID, name)
			if err == nil {
				err = settings.SaveLayoutTemplate(prefs.LayoutTemplate{Name: name, Root: template.Root})
			}
			post(func() { done(err) })
		}()
	}
	h.ApplyLayout = func(op session.Operation, name string, done func(error)) {
		var template *herdr.LayoutTemplate
		for _, t := range settings.LayoutTemplates() {
			if t.Name == name {
				template = &herdr.LayoutTemplate{Name: t.Name, Root: t.Root}
				break
			}
		}
		if template == nil {
			done(fmt.Errorf("no saved layout named %q", name))
			return
		}
		go func() {
			err := herdr.ApplyLayoutTemplate(ctx, target, op.WorkspaceID, *template)
			post(func() { s.Refresh(); done(err) })
		}()
	}
	h.DeleteLayout = func(name string) {
		if err := settings.DeleteLayoutTemplate(name); err != nil {
			log.Printf("prefs: %v", err)
		}
	}
	h.ShareReminder = func(n app.Notice) {
		go func() {
			if err := herdr.ShowNotification(ctx, target, n.Title, n.Body, "done"); err != nil {
				log.Printf("notification.show: %v", err)
			}
		}()
	}
	h.ShareReminders = func(on bool) { persist(func(v *prefs.Settings) { v.ShareReminders = on }) }
	h.Manifests = func(reload bool, done func([]herdr.AgentManifest, error)) {
		go func() {
			read := herdr.ReadAgentManifests
			if reload {
				read = herdr.ReloadAgentManifests
			}
			entries, err := read(ctx, target)
			post(func() { done(entries, err) })
		}()
	}
	h.PluginLink = func(path string, done func(error)) {
		if strings.HasPrefix(path, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				path = filepath.Join(home, path[2:])
			}
		}
		go func() { _, err := herdr.LinkPlugin(ctx, target, path); post(func() { done(err) }) }()
	}
	h.PluginUnlink = func(id string, done func(error)) {
		go func() { err := herdr.UnlinkPlugin(ctx, target, id); post(func() { done(err) }) }()
	}
	h.LiveHandoff = func(done func(error)) {
		exe := install().Path
		go func() { err := herdr.LiveHandoff(ctx, target, exe); post(func() { done(err) }) }()
	}
	h.StartupMode = func(mode string) { persist(func(v *prefs.Settings) { v.StartupMode = mode }) }
	h.Installation = install
	return h
}

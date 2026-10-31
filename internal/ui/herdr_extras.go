package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
)

// ExtraHooks reach the Herdr requests that act on agents, layouts and the
// server beyond the session's own shortcut lane. Every callback completes
// on the UI thread.
type ExtraHooks struct {
	AgentPrompt  func(op session.Operation, text string, done func(error))
	AgentWait    func(op session.Operation, until []string, timeout time.Duration, done func(herdr.AgentDetails, error)) func()
	AgentStart   func(op session.Operation, kind string, done func(error))
	AgentView    func(preset string, done func(error))
	SaveLayout   func(op session.Operation, name string, done func(error))
	ApplyLayout  func(op session.Operation, name string, done func(error))
	DeleteLayout func(name string)
	// LayoutNames lists the saved templates.
	LayoutNames func() []string
	// ShareReminder repeats one of Hergo's reminders in Herdr's clients
	// (notification.show), when the user asked for it.
	ShareReminder  func(app.Notice)
	ShareReminders func(on bool)
	Manifests      func(reload bool, done func([]herdr.AgentManifest, error))
	PluginLink     func(path string, done func(error))
	PluginUnlink   func(id string, done func(error))
	LiveHandoff    func(done func(error))
	// StartupMode persists how the app opens next ("auto", "herdr" or
	// "plain"); PlainTerminal opens the plain terminal now.
	StartupMode   func(mode string)
	PlainTerminal func() error
	// Installation is what Hergo found of the Herdr executable.
	Installation func() herdr.Installation
	// WindowTitle sets the native window's title.
	WindowTitle func(string)
}

func (w *Window) SetExtraHooks(h ExtraHooks) { w.extra = h }

// ExtraState is the persisted state the extras start from.
type ExtraState struct {
	AgentView      string
	ShareReminders bool
	StartupMode    string
}

func (w *Window) SetExtraState(s ExtraState) {
	w.agentView = herdr.AgentViewPresetByID(s.AgentView).ID
	w.shareReminders = s.ShareReminders
	w.startupMode = s.StartupMode
	if w.startupMode == "" {
		w.startupMode = "auto"
	}
}

type agentWatchUI struct {
	cancel     func()
	op         session.Operation
	generation uint64
	status     string
}

// extraAction reports whether action is one of the extras, which need a
// connected, ready session as the other shortcuts do.
func extraAction(action string) bool {
	switch action {
	case "agent_prompt", "agent_wait", "agent_start", "move_pane_new_tab", "move_pane_new_workspace", "move_workspace_up", "move_workspace_down", "save_layout", "apply_layout", "agent_view_next":
		return true
	}
	return false
}

// canExtra adds what each extra needs beyond a ready pane.
func (w *Window) canExtra(action string, v app.View) bool {
	switch action {
	case "agent_prompt":
		return w.extra.AgentPrompt != nil && w.selectedAgent(v) != ""
	case "agent_wait":
		return w.extra.AgentWait != nil && (w.agentWatch.cancel != nil || w.selectedAgent(v) != "")
	case "agent_start":
		return w.extra.AgentStart != nil && w.selectedAgent(v) == ""
	case "move_pane_new_tab":
		return len(v.Panes) > 1
	case "move_workspace_up", "move_workspace_down":
		return len(v.Rows) > 1
	case "save_layout":
		return w.extra.SaveLayout != nil && len(v.Panes) > 0
	case "apply_layout":
		return w.extra.ApplyLayout != nil && w.extra.LayoutNames != nil && len(w.extra.LayoutNames()) > 0
	case "agent_view_next":
		return w.extra.AgentView != nil
	}
	return true
}

func (w *Window) selectedAgent(v app.View) string {
	for _, pane := range v.Panes {
		if pane.ID == v.PaneID {
			return pane.Agent
		}
	}
	return ""
}

// dispatchExtra runs an extra; false leaves action to the shortcut path.
func (w *Window) dispatchExtra(action string) bool {
	v := w.currentView()
	switch action {
	case "agent_prompt":
		op, ok := w.shortcutOperation(action)
		if !ok {
			return true
		}
		agent := w.selectedAgent(v)
		w.ask(shortcutPrompt{title: "Send a prompt to " + fallback(agent, "the agent"), placeholder: "What should the agent do next?", message: "Herdr types the prompt into the agent and submits it.", action: "Send", required: true}, func(text string) {
			if !w.featureCurrent(op) {
				return
			}
			w.extra.AgentPrompt(op, text, func(err error) {
				if err != nil {
					w.shortcutError(err)
					return
				}
				w.controller.message = "Prompt sent to " + fallback(agent, "the agent") + "."
			})
		})
		return true
	case "agent_wait":
		w.toggleAgentWatch()
		return true
	case "agent_start":
		op, ok := w.shortcutOperation(action)
		if !ok {
			return true
		}
		w.ask(shortcutPrompt{title: "Start an agent in this pane", value: "claude", placeholder: "Agent kind, as claude or codex", message: "Herdr launches the agent in the pane's shell and waits until it is ready.", action: "Start", required: true}, func(kind string) {
			if !w.featureCurrent(op) {
				return
			}
			w.controller.message = "Starting " + kind + "…"
			w.extra.AgentStart(op, kind, func(err error) {
				if err != nil {
					w.shortcutError(err)
					return
				}
				w.controller.message = ""
				if w.terminal != nil {
					w.terminal.session.Refresh()
				}
			})
		})
		return true
	case "move_workspace_up", "move_workspace_down":
		delta := -1
		if action == "move_workspace_down" {
			delta = 1
		}
		w.moveWorkspace(v.WorkspaceID, delta)
		return true
	case "save_layout":
		op, ok := w.shortcutOperation(action)
		if !ok {
			return true
		}
		w.ask(shortcutPrompt{title: "Save tab layout", value: tabName(v), placeholder: "Layout name", message: "Saves the splits, ratios, pane names and directories of this tab. Programs and environments are not saved.", action: "Save", required: true}, func(name string) {
			w.extra.SaveLayout(op, strings.TrimSpace(name), func(err error) {
				if err != nil {
					w.shortcutError(err)
					return
				}
				w.controller.message = "Saved layout " + strings.TrimSpace(name) + "."
			})
		})
		return true
	case "apply_layout":
		names := w.extra.LayoutNames()
		w.ask(shortcutPrompt{title: "New tab from layout", value: names[0], placeholder: "Layout name", message: "Saved layouts: " + strings.Join(names, ", "), action: "Open", required: true}, func(name string) {
			w.applyLayout(strings.TrimSpace(name))
		})
		return true
	case "agent_view_next":
		next := 0
		for i, preset := range herdr.AgentViewPresets {
			if preset.ID == w.agentView {
				next = (i + 1) % len(herdr.AgentViewPresets)
			}
		}
		w.chooseAgentView(herdr.AgentViewPresets[next].ID)
		return true
	}
	return false
}

// applyLayout opens a saved layout as a new tab of the selected workspace.
func (w *Window) applyLayout(name string) {
	op, ok := w.shortcutOperation("apply_layout")
	if !ok || w.extra.ApplyLayout == nil {
		return
	}
	w.extra.ApplyLayout(op, name, func(err error) {
		w.shortcutError(err)
		if w.terminal != nil {
			w.terminal.session.Refresh()
		}
	})
}

// chooseAgentView filters the sidebar's agents and shares the filter with
// Herdr's clients.
func (w *Window) chooseAgentView(id string) {
	preset := herdr.AgentViewPresetByID(id)
	w.agentView = preset.ID
	if w.extra.AgentView != nil {
		w.extra.AgentView(preset.ID, func(err error) {
			if err != nil && w.controller != nil {
				w.controller.message = "Herdr did not take the agent view: " + err.Error()
			}
		})
	}
}

// filterAgentView keeps the agents the chosen preset shows.
func (w *Window) filterAgentView(agents []app.AgentRow, workspace string) []app.AgentRow {
	preset := herdr.AgentViewPresetByID(w.agentView)
	if preset.ID == "all" {
		return agents
	}
	var kept []app.AgentRow
	for _, agent := range agents {
		if preset.CurrentWorkspace && agent.WorkspaceID != workspace {
			continue
		}
		if len(preset.Statuses) > 0 {
			match := false
			for _, status := range preset.Statuses {
				if strings.EqualFold(agent.Status, status) {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		kept = append(kept, agent)
	}
	return kept
}

func agentViewShortLabel(id string) string {
	switch id {
	case "attention":
		return "Attention"
	case "working":
		return "Working"
	case "workspace":
		return "Space"
	}
	return "All"
}

// toggleAgentWatch starts a reminder for the selected agent's next stop,
// or cancels the one running.
func (w *Window) toggleAgentWatch() {
	s := &w.agentWatch
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.generation++
		s.status = "Agent reminder cancelled."
		w.controller.message = s.status
		return
	}
	op, ok := w.shortcutOperation("agent_wait")
	if !ok || w.extra.AgentWait == nil {
		return
	}
	agent := w.selectedAgent(w.currentView())
	w.ask(shortcutPrompt{title: "Notify when " + fallback(agent, "the agent") + " stops", value: "30", placeholder: "Minutes, 1 to 1440", message: "Notifies once when the agent is done, waiting for you, or idle. Cancel from the Pane menu.", action: "Watch", required: true}, func(text string) {
		minutes, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || minutes < 1 || minutes > 1440 {
			minutes = 30
		}
		w.startAgentWatch(op, agent, time.Duration(minutes)*time.Minute)
	})
}

func (w *Window) startAgentWatch(op session.Operation, agent string, timeout time.Duration) {
	if !w.featureCurrent(op) {
		return
	}
	s := &w.agentWatch
	s.generation++
	gen := s.generation
	s.op = op
	s.status = "Waiting for " + fallback(agent, "the agent") + " to stop"
	w.controller.message = s.status + "."
	s.cancel = w.extra.AgentWait(op, []string{"done", "blocked", "idle"}, timeout, func(details herdr.AgentDetails, err error) {
		if gen != s.generation {
			return
		}
		s.cancel = nil
		if !w.featureCurrent(op) {
			s.status = "Agent reminder ended because the terminal changed."
			return
		}
		if err != nil {
			s.status = "Agent reminder ended: " + err.Error()
			return
		}
		state := statusLabel(details.Status)
		s.status = fallback(agent, "The agent") + ": " + state
		w.deliverOutputNotice(app.Notice{Kind: "AgentStopped", Title: fallback(details.Name, fallback(agent, "Agent")) + " · " + state, Body: fallback(details.Title, "The agent stopped working."), WorkspaceID: op.WorkspaceID, TabID: op.TabID, PaneID: op.PaneID, Received: time.Now()})
	})
}

// syncAgentWatch ends the reminder when its terminal or epoch ends.
func (w *Window) syncAgentWatch() {
	s := &w.agentWatch
	if s.cancel != nil && !w.featureCurrent(s.op) {
		s.cancel()
		s.cancel = nil
		s.generation++
		s.status = "Agent reminder ended because the terminal changed."
	}
}

// targetOperation is a shortcut operation aimed at a workspace or tab other
// than the selected one, through a pane inside it, as the server's request
// identity needs one.
func (w *Window) targetOperation(action, workspaceID, tabID string) (session.Operation, bool) {
	if w.terminal == nil || w.terminal.session == nil {
		return session.Operation{}, false
	}
	p := w.terminal.session.Latest()
	if p.State != session.Ready || p.Stale || p.Snapshot == nil {
		return session.Operation{}, false
	}
	if tabID == "" {
		tabs := p.Snapshot.Tabs(workspaceID)
		if len(tabs) == 0 {
			return session.Operation{}, false
		}
		tabID = tabs[0].ID
	}
	tab, ok := p.Snapshot.Tab(tabID)
	if !ok {
		return session.Operation{}, false
	}
	paneID := p.Snapshot.FocusedPane(tabID)
	pane, ok := p.Snapshot.Pane(paneID)
	if !ok {
		panes := p.Snapshot.Panes(tabID)
		if len(panes) == 0 {
			return session.Operation{}, false
		}
		pane = panes[0]
	}
	return session.Operation{Kind: session.Shortcut, Action: action, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: tab.WorkspaceID, TabID: tab.ID, PaneID: pane.ID, TerminalID: pane.TerminalID}, true
}

// workspaceOrder is the server's workspace order, which insert_index counts.
func (w *Window) workspaceOrder() []string {
	if w.terminal == nil || w.terminal.session == nil {
		return nil
	}
	p := w.terminal.session.Latest()
	if p.Snapshot == nil {
		return nil
	}
	var ids []string
	for _, ws := range p.Snapshot.Workspaces() {
		ids = append(ids, ws.ID)
	}
	return ids
}

// insertIndex is the insert_index that places the item at from just before
// (up) or after (down) the item at to, counted, as tab.move and
// workspace.move count it, in the order before the move.
func insertIndex(from, to int) int {
	if to > from {
		return to + 1
	}
	return to
}

func (w *Window) moveWorkspace(id string, delta int) {
	order := w.workspaceOrder()
	from := indexOf(order, id, -1)
	if from < 0 || len(order) < 2 {
		return
	}
	to := wrap(from+delta, len(order))
	w.reorderWorkspace(id, order[to])
}

// reorderWorkspace moves the workspace id to where onto is.
func (w *Window) reorderWorkspace(id, onto string) {
	order := w.workspaceOrder()
	from, to := indexOf(order, id, -1), indexOf(order, onto, -1)
	if from < 0 || to < 0 || from == to {
		return
	}
	op, ok := w.targetOperation("move_workspace", id, "")
	if !ok {
		return
	}
	op.Index = insertIndex(from, to)
	w.runShortcut(op, nil)
}

// reorderTab moves the tab id to where onto is in its workspace's strip.
func (w *Window) reorderTab(id, onto string) {
	v := w.currentView()
	from, to := -1, -1
	for i, tab := range v.Tabs {
		if tab.ID == id {
			from = i
		}
		if tab.ID == onto {
			to = i
		}
	}
	if from < 0 || to < 0 || from == to {
		return
	}
	op, ok := w.targetOperation("move_tab_to", "", id)
	if !ok {
		return
	}
	op.Index = insertIndex(from, to)
	w.runShortcut(op, nil)
}

// movePaneToTab moves the selected pane into another tab, beside its
// focused pane.
func (w *Window) movePaneToTab(paneID, tabID string) {
	w.app.SelectPane(paneID)
	op, ok := w.shortcutOperation("move_pane_to_tab")
	if !ok {
		return
	}
	op.Text, op.Direction = tabID, "right"
	w.runShortcut(op, nil)
}

// workspaceDrag and tabDrag are what the sidebar rows and the tab pills
// drag, so that each only takes its own kind.
type workspaceDrag struct{ ID string }
type tabDrag struct{ ID string }

// shareReminder repeats a reminder in Herdr's clients when asked to.
func (w *Window) shareReminder(n app.Notice) {
	if w.shareReminders && w.extra.ShareReminder != nil {
		w.extra.ShareReminder(n)
	}
}

// syncWindowTitle names the window after the selected workspace and tab,
// as a terminal names itself after its program.
func (w *Window) syncWindowTitle(v app.View) {
	if w.extra.WindowTitle == nil {
		return
	}
	title := "Hergo"
	if v.WorkspaceName != "" {
		title = v.WorkspaceName
		if name := tabName(v); name != "" && len(v.Tabs) > 1 {
			title += " · " + name
		}
		title += " — Hergo"
	}
	if title != w.title {
		w.title = title
		w.extra.WindowTitle(title)
	}
}

func layoutNamesLabel(names []string) string {
	if len(names) == 0 {
		return "No saved layouts"
	}
	return fmt.Sprintf("%d saved: %s", len(names), strings.Join(names, ", "))
}

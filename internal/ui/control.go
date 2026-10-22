package ui

import (
	"fmt"
	"os"

	"herdr-ui/internal/control"
	"herdr-ui/internal/herdr"
)

// The window answers control requests: the CLI asks it to open a page, to
// focus a tab, to read a page's text and so on.

// ControlStatus says what the window is showing.
func (a *App) ControlStatus() control.Response {
	a.mu.Lock()
	snapshot := a.snapshot
	status := a.status
	a.mu.Unlock()

	response := control.Response{PID: os.Getpid(), Message: status}
	if snapshot == nil {
		return response
	}
	response.BootID = snapshot.BootID
	for _, workspace := range snapshot.Workspaces {
		response.Workspaces = append(response.Workspaces, control.WorkspaceInfo{
			ID:          workspace.WorkspaceID,
			Label:       workspace.Label,
			Number:      workspace.Number,
			TabCount:    len(snapshot.TabsOf(workspace.WorkspaceID)),
			AgentStatus: workspace.AgentStatus,
			Focused:     workspace.Focused,
		})
	}
	for _, tab := range snapshot.Tabs {
		response.Tabs = append(response.Tabs, control.TabInfo{
			ID:          tab.TabID,
			WorkspaceID: tab.WorkspaceID,
			Label:       tab.Label,
			Number:      tab.Number,
			PaneCount:   len(snapshot.PanesOf(tab.TabID)),
			AgentStatus: tab.AgentStatus,
			Focused:     tab.Focused,
		})
	}
	for _, pane := range snapshot.Panes {
		info := control.PaneInfo{
			ID:          pane.PaneID,
			WorkspaceID: pane.WorkspaceID,
			TabID:       pane.TabID,
			Focused:     pane.Focused,
		}
		if pane.CWD != nil {
			info.CWD = *pane.CWD
		}
		if agent, ok := snapshot.AgentOf(pane.PaneID); ok {
			info.AgentStatus = agent.AgentStatus
			if agent.Agent != nil {
				info.Agent = *agent.Agent
			}
			if agent.TerminalTitle != nil {
				info.Title = *agent.TerminalTitle
			}
		}
		response.Panes = append(response.Panes, info)
	}
	for _, page := range a.Pages() {
		response.Pages = append(response.Pages, control.PageInfo{
			ID:    page.ID,
			Title: page.Title,
			URL:   page.URL,
			File:  page.File,
			Zoom:  page.Zoom,
		})
	}
	for _, worktree := range a.worktreeRows() {
		response.Worktrees = append(response.Worktrees, control.WorktreeInfo{
			Path:            worktree.Path,
			Label:           worktree.Label,
			Branch:          worktree.Branch,
			OpenWorkspaceID: worktree.OpenWorkspaceID,
		})
	}
	for _, integration := range a.integrationRows() {
		response.Integrations = append(response.Integrations, control.IntegrationInfo{
			Target:    integration.Target,
			Label:     integration.Label,
			Command:   integration.Command,
			State:     integration.State,
			Available: integration.Available,
		})
	}
	return response
}

// ControlOpenPage opens a page for a URL or a local file.
func (a *App) ControlOpenPage(target string) (control.PageInfo, error) {
	page, err := a.OpenPage(target)
	if err != nil {
		return control.PageInfo{}, err
	}
	return control.PageInfo{
		ID:    page.ID,
		Title: page.Title,
		URL:   page.URL,
		File:  page.File,
		Zoom:  page.Zoom,
	}, nil
}

// ControlPages lists the pages.
func (a *App) ControlPages() []control.PageInfo {
	pages := a.Pages()
	out := make([]control.PageInfo, 0, len(pages))
	for _, page := range pages {
		out = append(out, control.PageInfo{
			ID:    page.ID,
			Title: page.Title,
			URL:   page.URL,
			File:  page.File,
			Zoom:  page.Zoom,
		})
	}
	return out
}

// ControlPage returns one page.
func (a *App) ControlPage(id string) (control.PageInfo, bool) {
	page, ok := a.Page(id)
	if !ok {
		return control.PageInfo{}, false
	}
	return control.PageInfo{
		ID:    page.ID,
		Title: page.Title,
		URL:   page.URL,
		File:  page.File,
		Zoom:  page.Zoom,
	}, true
}

// ControlClosePage closes a page.
func (a *App) ControlClosePage(id string) error {
	a.mu.Lock()
	found := false
	for _, page := range a.pages {
		if page.ID == id {
			found = true
		}
	}
	a.mu.Unlock()
	if !found {
		return fmt.Errorf("no page %s", id)
	}
	a.closePage(id)
	return nil
}

// ControlReloadPage reloads a page.
func (a *App) ControlReloadPage(id string) error { return a.ReloadPage(id) }

// ControlNavigatePage loads another address in a page.
func (a *App) ControlNavigatePage(id, url string) error { return a.NavigatePage(id, url) }

// ControlPageBack goes back in a page.
func (a *App) ControlPageBack(id string) error { return a.PageBack(id) }

// ControlPageForward goes forward in a page.
func (a *App) ControlPageForward(id string) error { return a.PageForward(id) }

// ControlPageText reads the text of a page.
func (a *App) ControlPageText(id string) (string, error) { return a.PageText(id) }

// ControlPageZoom changes the zoom of a page.
func (a *App) ControlPageZoom(id, change string) (float64, error) { return a.PageZoom(id, change) }

// ControlSendNotes sends notes to the focused pane.
func (a *App) ControlSendNotes(text string) error {
	if text == "" {
		return fmt.Errorf("no notes to send")
	}
	a.mu.Lock()
	snapshot := a.snapshot
	a.mu.Unlock()
	if snapshot == nil {
		return fmt.Errorf("the daemon has not sent a snapshot yet")
	}
	pane, ok := snapshot.FocusedPane()
	if !ok {
		return fmt.Errorf("no pane is focused")
	}
	if err := a.endpoint.SendText(pane.PaneID, text); err != nil {
		return err
	}
	a.setStatus("sent %d characters to %s", len(text), pane.PaneID)
	return nil
}

// ControlFocusWorkspace focuses a workspace.
func (a *App) ControlFocusWorkspace(id string) error {
	a.focusWorkspace(id)
	return nil
}

// ControlFocusTab focuses a tab.
func (a *App) ControlFocusTab(id string) error {
	a.focusTab(id)
	return nil
}

// ControlCloseTab closes a tab.
func (a *App) ControlCloseTab(id string) error {
	a.mu.Lock()
	snapshot := a.snapshot
	a.mu.Unlock()
	if snapshot == nil {
		return fmt.Errorf("the daemon has not sent a snapshot yet")
	}
	if _, ok := snapshot.Tab(id); !ok {
		return fmt.Errorf("no tab %s", id)
	}
	return a.requestSync("tab.close", map[string]any{"tab_id": id})
}

// requestSync sends one API request and waits for its answer.
func (a *App) requestSync(method string, params map[string]any) error {
	ctx, cancel := contextWithTimeout()
	defer cancel()
	if err := a.endpoint.Request(ctx, method, params, nil); err != nil {
		return err
	}
	return nil
}

// snapshotOf returns the session the daemon last sent.
func (a *App) snapshotOf() *herdr.Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.snapshot
}

// ControlCommands lists the commands the daemon offers.
func (a *App) ControlCommands() []control.CommandInfo {
	snapshot := a.snapshotOf()
	if snapshot == nil {
		return nil
	}
	commands := make([]control.CommandInfo, 0, len(snapshot.Commands))
	for _, command := range snapshot.Commands {
		commands = append(commands, control.CommandInfo{
			ID:          command.CommandID,
			Description: command.Description,
			Action:      command.Action,
			Binding:     command.BindingLabel,
		})
	}
	return commands
}

// ControlInvokeCommand carries out a command.
func (a *App) ControlInvokeCommand(id string) error {
	if id == "" {
		return fmt.Errorf("no command to run")
	}
	ctx, cancel := contextWithTimeout()
	defer cancel()
	if err := a.endpoint.Request(ctx, "command.invoke", map[string]any{"command_id": id}, nil); err != nil {
		return err
	}
	return nil
}

// ControlAgents lists the agents of the session.
func (a *App) ControlAgents() []control.AgentInfo {
	snapshot := a.snapshotOf()
	if snapshot == nil {
		return nil
	}
	agents := make([]control.AgentInfo, 0, len(snapshot.Agents))
	for _, agent := range agentRows(snapshot) {
		info := control.AgentInfo{
			PaneID:      agent.PaneID,
			WorkspaceID: agent.WorkspaceID,
			TabID:       agent.TabID,
			Status:      agent.AgentStatus,
			Focused:     agent.Focused,
		}
		if agent.Agent != nil {
			info.Agent = *agent.Agent
		}
		if agent.TerminalTitle != nil {
			info.Title = *agent.TerminalTitle
		}
		agents = append(agents, info)
	}
	return agents
}

// ControlFind searches the output of the focused pane.
func (a *App) ControlFind(query, direction string) (int, int, error) {
	snapshot := a.snapshotOf()
	pane, ok := a.targetPane(snapshot)
	if !ok {
		return 0, 0, fmt.Errorf("no pane is focused")
	}
	a.find(pane.PaneID, query, direction)
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.finder.total, a.finder.current, nil
}

// ControlFindNext moves to the next match, or to the one before it.
func (a *App) ControlFindNext(step int) (int, int, error) {
	a.findNext(step)
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.finder.total, a.finder.current, nil
}

// ControlFindClose closes the find bar.
func (a *App) ControlFindClose() { a.closeFind() }

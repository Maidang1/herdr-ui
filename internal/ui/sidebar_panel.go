package ui

import (
	"fmt"
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The sidebar of the session.
//
// It shows the spaces of the session as a tree: a row for every workspace
// with its branch and its ports, and the agents of the workspace under it.
// Below that, the agents again, grouped by the workspace they work in.
//
// The Rust client draws the same tree, with the same measures.

// drawRail shows the column of views at the left of the window.
func (a *App) drawRail(c *ui.Context, colors chrome, view string) {
	view = a.currentRailView(view)
	ui.Column(c).Width(railWidth).Background(colors.panel).Children(func() {
		railButton(c, colors, "terminal", "Spaces", view == "spaces", func() {
			a.setRailView("spaces")
		})
		railButton(c, colors, "pulse", "Agents", view == "agents", func() {
			a.setRailView("agents")
		})
		railButton(c, colors, "git-branch", "Worktrees", view == "worktrees", func() {
			a.setRailView("worktrees")
		})
		ui.Row(c).Grow(1)
		railButton(c, colors, "settings", "Settings", false, func() {
			a.openSettings()
		})
	})
}

// railButton draws one button of the column of views.
//
// The buttons are the marks of the views: the one that is showing wears the
// accent, the others are quiet.
func railButton(c *ui.Context, colors chrome, name, title string, active bool, run func()) {
	mark := icon(name)
	button := ui.Box(c).Width(railWidth).Height(40).AlignItems(ui.Center).Justify(ui.Center).
		Label(title).Role(ui.RoleButton)
	button.Children(func() {
		if mark == nil {
			return
		}
		inner := ui.Box(c).Width(30).Height(30).Radius(7).AlignItems(ui.Center).Justify(ui.Center)
		if active {
			inner.Border(1, colors.muted)
		}
		inner.Children(func() {
			color := colors.muted
			if active {
				color = colors.text
			}
			ui.Icon(c, mark).FontSize(18).TextColor(color)
		})
	})
	button.OnClick(run)
}

// drawSidebarPanel shows the spaces of the session and their agents.
func (a *App) drawSidebarPanel(c *ui.Context, colors chrome, snapshot *herdr.Snapshot, selected string) {
	fontSize := a.uiFontSizePx()
	view := a.currentRailView("")
	layout := a.sidebarLayout()

	ui.Column(c).Width(sidebarWidth).Background(colors.sidebar).Children(func() {
		// The head of the sidebar names what it holds.
		ui.Row(c).Height(headerHeight).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "WORKSPACES").Bold().FontSize(fontSize - 3).TextColor(colors.text)
			ui.Row(c).Grow(1)
			if mark := icon("panel-left"); mark != nil {
				ui.Icon(c, mark).FontSize(fontSize).TextColor(colors.muted)
			}
		})

		switch view {
		case "agents":
			a.drawAgentGroups(c, colors, snapshot, fontSize)
		case "worktrees":
			a.drawWorktreeList(c, colors, fontSize)
		default:
			a.drawSpaces(c, colors, snapshot, selected, fontSize, layout)
		}
	})
}

// drawSpaces shows the workspaces of the session as a tree.
func (a *App) drawSpaces(c *ui.Context, colors chrome, snapshot *herdr.Snapshot, selected string, fontSize float32, layout Layout) {
	ui.Row(c).Height(sectionHeight).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "spaces").FontSize(fontSize).TextColor(colors.text)
		ui.Row(c).Grow(1)
		if mark := icon("search"); mark != nil {
			ui.Icon(c, mark).FontSize(fontSize - 1).TextColor(colors.muted)
		}
	})

	for _, workspace := range snapshot.Workspaces {
		workspace := workspace
		active := workspace.WorkspaceID == selected
		label := workspace.Label
		if label == "" {
			label = fmt.Sprintf("%d", workspace.Number)
		}

		// The row of the workspace: its mark, its icon and its name.
		row := ui.Row(c).Padding(0, a.sidebarPadding()).Gap(4).AlignItems(ui.Center).Children(func() {
			if mark := icon("workspace"); mark != nil {
				ui.Icon(c, mark).FontSize(statusWidth + 6).TextColor(colors.muted)
			}
			agent := agentIcon(a.agentNameOfWorkspace(snapshot, workspace.WorkspaceID))
			if agent != nil {
				ui.Icon(c, agent).FontSize(iconSize).TextColor(colors.muted)
			}
			ui.Row(c).Width(2)
			text := ui.Text(c, label).FontSize(fontSize)
			if active {
				text.Bold().TextColor(colors.text)
			} else {
				text.TextColor(colors.text)
			}
			text.OnClick(func() {
				a.focusWorkspace(workspace.WorkspaceID)
			})
		})
		if layout.Rounded() {
			// The rounded layout draws the rows as cards of their own.
			row.Radius(6).Background(colors.sidebar)
			if active {
				row.Background(colors.panel)
			}
		}

		// The branch it sits on.
		if workspace.Branch != nil && *workspace.Branch != "" {
			ui.Row(c).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Width(childIndent)
				ui.Text(c, *workspace.Branch).FontSize(fontSize).TextColor(colors.yellow).OnClick(func() {
					a.focusWorkspace(workspace.WorkspaceID)
				})
			})
		}

		// The agents that work in it.
		for _, agent := range a.agentsOfWorkspace(snapshot, workspace.WorkspaceID) {
			agent := agent
			ui.Row(c).Padding(0, a.sidebarPadding()).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "•").FontSize(fontSize).TextColor(colors.muted)
				mark := agentIcon(agentName(agent))
				if mark != nil {
					ui.Icon(c, mark).FontSize(iconSize).TextColor(colors.muted)
				}
				ui.Row(c).Width(2)
				ui.Text(c, agentLabel(agent)).FontSize(fontSize).TextColor(colors.text).OnClick(func() {
					a.focusPane(agent.PaneID)
				})
			})
		}

		// The ports the workspace listens on.
		if ports := a.portsOfWorkspace(workspace.WorkspaceID); len(ports) > 0 {
			ui.Row(c).Padding(0, a.sidebarPadding()).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Width(childIndent)
				mark := icon("globe")
				if mark != nil {
					ui.Icon(c, mark).FontSize(fontSize - 2).TextColor(colors.muted)
				}
				ui.Text(c, strings.Join(ports, " ")).FontSize(fontSize - 1).TextColor(colors.muted)
			})
		}
	}

	// The foot of the spaces: what the user may add, and the menu of the
	// sidebar.
	ui.Row(c).Height(sectionHeight).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "new").FontSize(fontSize).TextColor(colors.text).OnClick(func() {
			a.newWorkspace()
		})
		ui.Row(c).Grow(1)
		ui.Text(c, "menu").FontSize(fontSize).TextColor(colors.muted).OnClick(func() {
			a.setStatus("the menu of the sidebar is not here yet")
		})
	})

	// The agents of the session, grouped by the workspace they work in.
	a.drawAgentGroups(c, colors, snapshot, fontSize)
}

// drawAgentGroups shows the agents of the session, grouped by workspace.
func (a *App) drawAgentGroups(c *ui.Context, colors chrome, snapshot *herdr.Snapshot, fontSize float32) {
	ui.Row(c).Height(sectionHeight).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "agents").FontSize(fontSize).TextColor(colors.text)
		ui.Row(c).Grow(1)
		ui.Text(c, "grouped").FontSize(fontSize - 1).TextColor(colors.muted)
	})

	for _, group := range a.agentGroups(snapshot) {
		ui.Row(c).Padding(0, a.sidebarPadding()).Gap(4).AlignItems(ui.Center).Children(func() {
			if mark := icon("workspace"); mark != nil {
				ui.Icon(c, mark).FontSize(statusWidth + 6).TextColor(colors.muted)
			}
			ui.Text(c, group.Label).Bold().FontSize(fontSize).TextColor(colors.text).OnClick(func() {
				a.focusWorkspace(group.WorkspaceID)
			})
			ui.Row(c).Width(6)
			ui.Text(c, fmt.Sprintf("· %d", group.Count)).FontSize(fontSize).TextColor(colors.muted)
		})
		for _, agent := range group.Agents {
			agent := agent
			// The name of the agent stands on a line of its own, and its
			// state under it, the way the Rust client draws them.
			ui.Row(c).Padding(0, a.sidebarPadding()).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Width(childIndent)
				mark := agentIcon(agentName(agent))
				if mark != nil {
					ui.Icon(c, mark).FontSize(iconSize).TextColor(colors.muted)
				}
				ui.Row(c).Width(2)
				ui.Text(c, agentLabel(agent)).FontSize(fontSize).TextColor(colors.text).OnClick(func() {
					a.focusPane(agent.PaneID)
				})
			})
			if agent.AgentStatus != "" {
				ui.Row(c).Padding(0, a.sidebarPadding()).AlignItems(ui.Center).Children(func() {
					ui.Row(c).Width(childIndent + iconSize + 4)
					ui.Text(c, agent.AgentStatus).FontSize(fontSize).TextColor(colors.stateColor(agent.AgentStatus))
				})
			}
		}
	}
}

// drawWorktreeList shows the worktrees of the repository.
func (a *App) drawWorktreeList(c *ui.Context, colors chrome, fontSize float32) {
	ui.Row(c).Height(sectionHeight).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "worktrees").FontSize(fontSize).TextColor(colors.text)
		ui.Row(c).Grow(1)
	})
	for _, worktree := range a.worktreeRows() {
		worktree := worktree
		label := worktree.Label
		if worktree.Branch != "" {
			label = worktree.Label + "  " + worktree.Branch
		}
		ui.Row(c).Padding(0, rowPadding).AlignItems(ui.Center).Children(func() {
			mark := icon("git-branch")
			if mark != nil {
				ui.Icon(c, mark).FontSize(iconSize).TextColor(colors.muted)
			}
			ui.Text(c, label).FontSize(fontSize).TextColor(colors.text).OnClick(func() {
				a.openWorktree(worktree.Path)
			})
		})
	}
}

// agentGroup is the agents of one workspace.
type agentGroup struct {
	// WorkspaceID is the workspace the agents work in.
	WorkspaceID string
	// Label is the name of the workspace.
	Label string
	// Count is how many agents work in it.
	Count int
	// Status is the state of the workspace.
	Status string
	// Agents are the agents themselves.
	Agents []herdr.Agent
}

// agentGroups groups the agents of the session by their workspace.
func (a *App) agentGroups(snapshot *herdr.Snapshot) []agentGroup {
	if snapshot == nil {
		return nil
	}
	var groups []agentGroup
	index := map[string]int{}
	labels := map[string]string{}
	statuses := map[string]string{}
	for _, workspace := range snapshot.Workspaces {
		label := workspace.Label
		if label == "" {
			label = fmt.Sprintf("%d", workspace.Number)
		}
		labels[workspace.WorkspaceID] = label
		statuses[workspace.WorkspaceID] = workspace.AgentStatus
	}
	// The agents are grouped by the workspace they work in, and an agent
	// whose workspace is unknown stands in a group of its own, so that no
	// agent is lost.
	for _, agent := range agentRows(snapshot) {
		workspaceID := agent.WorkspaceID
		position, ok := index[workspaceID]
		if !ok {
			label := labels[workspaceID]
			if label == "" {
				label = "unknown"
			}
			index[workspaceID] = len(groups)
			groups = append(groups, agentGroup{
				WorkspaceID: workspaceID,
				Label:       label,
				Status:      statuses[workspaceID],
			})
			position = len(groups) - 1
		}
		groups[position].Agents = append(groups[position].Agents, agent)
		groups[position].Count = len(groups[position].Agents)
	}
	return groups
}

// agentsOfWorkspace returns the agents that work in one workspace.
func (a *App) agentsOfWorkspace(snapshot *herdr.Snapshot, workspaceID string) []herdr.Agent {
	if snapshot == nil {
		return nil
	}
	var agents []herdr.Agent
	for _, agent := range agentRows(snapshot) {
		if agent.WorkspaceID == workspaceID {
			agents = append(agents, agent)
		}
	}
	return agents
}

// agentName says the name of an agent.
func agentName(agent herdr.Agent) string {
	if agent.DisplayAgent != nil && *agent.DisplayAgent != "" {
		return *agent.DisplayAgent
	}
	if agent.Agent != nil && *agent.Agent != "" {
		return *agent.Agent
	}
	return ""
}

// currentRailView returns the view the column of views at the left shows.
//
// The spaces are the first view, the way the Rust client opens them.
func (a *App) currentRailView(view string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.railView != "" {
		return a.railView
	}
	if view != "" {
		return view
	}
	return "spaces"
}

// setRailView asks the column of views at the left for another view.
func (a *App) setRailView(view string) {
	a.mu.Lock()
	a.railView = view
	a.mu.Unlock()
	a.invalidate()
}

// closeTabByID asks the daemon to close a tab.
func (a *App) closeTabByID(id string) {
	if id == "" {
		return
	}
	a.request("tab.close", map[string]any{"tab_id": id})
}

// newWorkspace asks the daemon for another workspace.
func (a *App) newWorkspace() {
	a.request("workspace.create", map[string]any{})
	a.setStatus("asked for a new workspace")
}

// agentNameOfWorkspace returns the name of the agent that works in a
// workspace, the one the mark of the workspace wears.
func (a *App) agentNameOfWorkspace(snapshot *herdr.Snapshot, workspaceID string) string {
	for _, agent := range a.agentsOfWorkspace(snapshot, workspaceID) {
		if name := agentName(agent); name != "" {
			return name
		}
	}
	return ""
}

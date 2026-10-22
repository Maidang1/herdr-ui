package ui

import (
	"strings"

	"herdr-ui/internal/design"
	"herdr-ui/internal/protocol"
	"herdr-ui/internal/ui/widgets"

	"github.com/egoist/mygo/ui"
)

// sidebar draws the tree of the workspaces, the groups of the agents, and
// the foot of both.
//
// The design file's rule is that the sidebar stands on the gradient of the
// window with nothing behind it: only the row that is chosen has a face of
// its own.
func (a *App) sidebar(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Width(design.SidebarWidth).
		Children(func() {
			widgets.SidebarHeader(c, t, "Workspaces")
			a.spacesSection(c, t)
			a.spacesTree(c, t)
			a.sidebarFooter(c, t)
			ui.Row(c).Height(design.SidebarDividerHeight).Background(t.Separator)
			a.agentsSection(c, t)
			a.agentGroups(c, t)
		})
}

// spacesSection draws the head of the spaces.
func (a *App) spacesSection(c *ui.Context, t design.Tokens) {
	ui.Row(c).
		Height(design.SidebarSectionHeight).
		Padding(0, design.SpaceXL).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			widgets.Label(c, t, "Spaces", widgets.LabelOptions{
				Size:   design.FontUI,
				Color:  t.TextPrimary,
				Weight: design.WeightMedium,
			})
			ui.Row(c).Grow(1)
			widgets.Icon(c, t, "search", design.IconMedium, t.TextTertiary)
		})
}

// spacesTree draws the workspaces of the session, and what is in them.
func (a *App) spacesTree(c *ui.Context, t design.Tokens) {
	snapshot := a.live.Snapshot()
	if snapshot == nil {
		a.sampleSpaces(c, t)
		return
	}
	ui.Column(c).
		Gap(design.SpaceXS).
		Padding(design.SpaceXS, design.SpaceMD).
		Children(func() {
			for _, workspace := range snapshot.Workspaces {
				mark := widgets.MarkOutline
				if workspace.Focused {
					mark = widgets.MarkAccent
				}
				id := workspace.WorkspaceID
				widgets.TreeRow(c, t, widgets.RowOptions{
					Mark:     mark,
					Selected: workspace.Focused,
					Icon:     "bot",
					Name:     workspace.Label,
					OnClick: func() {
						a.live.Request("workspace.focus", map[string]any{"workspace_id": id})
					},
				})
				if workspace.Branch != "" {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "git-branch",
						Text:   workspace.Branch,
						Color:  t.YellowText,
						Indent: design.TreeIndent,
						Height: design.BranchRowHeight,
					})
				}
				for _, pane := range snapshot.Panes {
					if pane.WorkspaceID != id {
						continue
					}
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "globe",
						Text:   paneName(pane),
						Indent: design.TreeIndent,
						Height: design.PortsRowHeight,
					})
				}
			}
		})
}

// sampleSpaces draws the sample workspaces, when the daemon is not there.
func (a *App) sampleSpaces(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Gap(design.SpaceXS).
		Padding(design.SpaceXS, design.SpaceMD).
		Children(func() {
			for _, space := range a.session.Spaces {
				mark := widgets.MarkOutline
				if space.Selected {
					mark = widgets.MarkAccent
				}
				widgets.TreeRow(c, t, widgets.RowOptions{
					Mark:     mark,
					Selected: space.Selected,
					Icon:     "bot",
					Name:     space.Name,
				})
				if space.Branch != "" {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "git-branch",
						Text:   space.Branch,
						Color:  t.YellowText,
						Indent: design.TreeIndent,
						Height: design.BranchRowHeight,
					})
				}
				if space.Ports != "" {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "globe",
						Text:   space.Ports,
						Indent: design.TreeIndent,
						Height: design.PortsRowHeight,
					})
				}
				for _, agent := range space.Agents {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "bot",
						Text:   agent.Name,
						Color:  t.TextPrimary,
						Indent: design.TreeIndent,
						Height: design.AgentRowHeight,
						Size:   design.FontUI,
					})
				}
			}
		})
}

// sidebarFooter draws the foot of the sidebar.
func (a *App) sidebarFooter(c *ui.Context, t design.Tokens) {
	ui.Row(c).
		Height(design.SidebarFooterHeight).
		Padding(0, design.SpaceXL).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			widgets.Label(c, t, "New", widgets.LabelOptions{
				Size:  design.FontUI,
				Color: t.AccentText,
			})
			widgets.Label(c, t, "Menu", widgets.LabelOptions{
				Size:  design.FontUI,
				Color: t.TextTertiary,
			})
		})
}

// agentsSection draws the head of the groups of the agents.
func (a *App) agentsSection(c *ui.Context, t design.Tokens) {
	ui.Row(c).
		Height(design.SidebarSectionHeight).
		Padding(0, design.SpaceXL).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			widgets.Label(c, t, "Agents", widgets.LabelOptions{
				Size:   design.FontUI,
				Color:  t.TextPrimary,
				Weight: design.WeightMedium,
			})
			ui.Row(c).Grow(1)
			widgets.Label(c, t, "Grouped", widgets.LabelOptions{
				Size:  design.FontBody,
				Color: t.TextTertiary,
			})
		})
}

// agentGroups draws the groups of the agents.
func (a *App) agentGroups(c *ui.Context, t design.Tokens) {
	snapshot := a.live.Snapshot()
	if snapshot == nil {
		a.sampleAgentGroups(c, t)
		return
	}
	// The agents are grouped by the workspace they work in.
	groups := map[string][]protocol.Agent{}
	order := []string{}
	for _, agent := range snapshot.Agents {
		if _, seen := groups[agent.WorkspaceID]; !seen {
			order = append(order, agent.WorkspaceID)
		}
		groups[agent.WorkspaceID] = append(groups[agent.WorkspaceID], agent)
	}
	ui.Column(c).
		Gap(design.SpaceXS).
		Padding(design.SpaceXS, design.SpaceMD).
		Children(func() {
			for _, workspaceID := range order {
				agents := groups[workspaceID]
				name := workspaceID
				for _, workspace := range snapshot.Workspaces {
					if workspace.WorkspaceID == workspaceID {
						name = workspace.Label
						break
					}
				}
				widgets.TreeRow(c, t, widgets.RowOptions{
					Mark:     widgets.MarkOutline,
					Name:     name,
					Trailing: "· " + itoa(len(agents)),
					Height:   design.GroupRowHeight,
				})
				for _, agent := range agents {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "bot",
						Text:   agentName(agent),
						Color:  t.TextPrimary,
						Indent: design.TreeIndent,
						Height: design.AgentRowHeight,
						Size:   design.FontUI,
					})
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Text:   string(agent.AgentStatus),
						Color:  agentStateColor(t, agent.AgentStatus),
						Indent: design.AgentStateIndent,
						Height: design.StateRowHeight,
					})
				}
			}
		})
}

// sampleAgentGroups draws the sample groups, when the daemon is not there.
func (a *App) sampleAgentGroups(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Gap(design.SpaceXS).
		Padding(design.SpaceXS, design.SpaceMD).
		Children(func() {
			for _, group := range a.session.Groups {
				widgets.TreeRow(c, t, widgets.RowOptions{
					Mark:     widgets.MarkOutline,
					Name:     group.Name,
					Trailing: "· " + itoa(group.Count),
					Height:   design.GroupRowHeight,
				})
				for _, agent := range group.Agents {
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Icon:   "bot",
						Text:   agent.Name,
						Color:  t.TextPrimary,
						Indent: design.TreeIndent,
						Height: design.AgentRowHeight,
						Size:   design.FontUI,
					})
					widgets.ChildRow(c, t, widgets.ChildRowOptions{
						Text:   agent.State,
						Color:  t.GreenText,
						Indent: design.AgentStateIndent,
						Height: design.StateRowHeight,
					})
				}
			}
		})
}

// paneName returns the name to show of a pane.
//
// The place of a pane is a long path; the sidebar shows the last two
// pieces of it, for the name of the directory is what a person knows it by.
func paneName(pane protocol.Pane) string {
	if pane.Label != "" {
		return pane.Label
	}
	path := pane.Cwd
	if path == "" {
		return pane.PaneID
	}
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	return strings.Join(parts, "/")
}

// agentName returns the name to show of an agent.
func agentName(agent protocol.Agent) string {
	if agent.DisplayAgent != "" {
		return agent.DisplayAgent
	}
	if agent.Name != "" {
		return agent.Name
	}
	return agent.PaneID
}

// agentStateColor returns the color of the state of an agent.
func agentStateColor(t design.Tokens, status protocol.AgentStatus) ui.Color {
	switch status {
	case protocol.AgentWorking:
		return t.AccentText
	case protocol.AgentBlocked:
		return t.RedText
	case protocol.AgentDone:
		return t.GreenText
	}
	return t.TextTertiary
}

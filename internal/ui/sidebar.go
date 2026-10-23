package ui

import (
	"fmt"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
	"strconv"
	"strings"
)

func (w *Window) sidebar(c *native.Context, v app.View, p design.Palette) {
	native.Column(c.Key("workspace-sidebar")).Label("Workspace sidebar").Width(w.sidebarWidth).Shrink(0).Background(p.Sidebar).AlignItems(native.Stretch).Gap(6).Children(func() {
		native.Row(c).Height(36).Shrink(0).Padding(0, 8).AlignItems(native.Center).DragWindow().Children(func() {
			native.Box(c).Grow(1)
			if w.iconButton(c, "hide-sidebar", icons.PanelLeft, "Hide sidebar", p).Clicked() {
				w.Dispatch("toggle_sidebar")
			}
		})
		native.Scroll(c.Key("sidebar-information")).Grow(1).MinHeight(0).FillWidth().Label("Spaces and agents").Children(func() {
			native.Column(c).FillWidth().Gap(6).Children(func() {
				native.Text(c, "spaces").FontSize(12).TextColor(p.Secondary).PaddingX(12)
				native.Column(c).FillWidth().Gap(0).Children(func() {
					for _, row := range v.Rows {
						w.workspace(c, row, p)
					}
				})
				native.Row(c).FillWidth().Padding(10, 12, 2, 12).Children(func() {
					native.Text(c, "agents").FontSize(12).TextColor(p.Secondary)
					native.Box(c).Grow(1)
					native.Text(c, "grouped").FontSize(12).TextColor(p.Secondary)
				})
				if len(v.Agents) == 0 {
					native.Text(c, "No agents").FontSize(12).TextColor(p.Tertiary).Padding(4, 12)
				}
				for i := 0; i < len(v.Agents); {
					end := i + 1
					for end < len(v.Agents) && v.Agents[end].WorkspaceID == v.Agents[i].WorkspaceID {
						end++
					}
					w.agentGroup(c, v.Agents[i:end], p)
					i = end
				}
			})
		})
		native.Row(c).Height(28).Shrink(0).PaddingX(12).AlignItems(native.Center).Gap(7).Children(func() {
			icon, color, label := connection(v.State, c.Theme().Dark)
			native.Icon(c, icon).Size(14, 14).TextColor(color).Label(label).Tooltip(label)
			native.Text(c, "Local").FontSize(12).TextColor(p.Secondary)
			native.Box(c).Grow(1)
			if w.iconButton(c, "keyboard-reference-entry", icons.Keyboard, "Keyboard reference", p).Clicked() {
				w.Dispatch("help")
			}
		})
		w.sidebarResizeHandle(c)
	})
}
func portsLabel(ports []int) string {
	labels := make([]string, len(ports))
	for i, port := range ports {
		labels[i] = ":" + strconv.Itoa(port)
	}
	return strings.Join(labels, " ")
}
func workspaceDescription(row app.WorkspaceRow) string {
	label := row.Name
	if row.Branch != "" {
		label += " · " + row.Branch
	}
	if row.Ahead > 0 {
		label += fmt.Sprintf(" · ↑%d", row.Ahead)
	}
	if row.Behind > 0 {
		label += fmt.Sprintf(" · ↓%d", row.Behind)
	}
	return label + " · " + statusLabel(row.Status)
}
func repositoryIcon(row app.WorkspaceRow) *native.SVG {
	if row.Remote == "github" {
		return icons.RepositoryGithub
	}
	if row.Remote != "" && row.Remote != "none" {
		return icons.Globe
	}
	if row.GitKnown {
		return icons.GitBranch
	}
	return icons.Folder
}
func (w *Window) workspace(c *native.Context, row app.WorkspaceRow, p design.Palette) {
	native.Column(c.Key("workspace:"+row.ID)).FillWidth().Padding(0, 0, 0, float32(row.Depth*16)).Children(func() {
		button := native.ButtonBase(c.Key("select:"+row.ID)).FillWidth().MinWidth(0).Height(42).Radius(8).Padding(4, 8).Label(workspaceDescription(row)).Tooltip(row.Path).Selected(row.Selected).TextColor(p.Text)
		if row.Selected {
			button.Background(p.Selected)
		} else {
			w.controlState(button, p)
		}
		if button.FocusVisible() {
			button.Border(2, p.Focus)
		}
		button.Children(func() {
			native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
				native.Icon(c, repositoryIcon(row)).Size(16, 16).TextColor(p.Secondary)
				native.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					name := native.Text(c, row.Name).FontSize(13).SingleLine()
					if row.Selected {
						name.Bold()
					}
					if row.Branch != "" {
						native.Row(c).FillWidth().Gap(6).AlignItems(native.Center).Children(func() {
							color := p.Secondary
							if row.Selected {
								color = p.Accent
							}
							native.Text(c, row.Branch).FontSize(12).TextColor(color).SingleLine().Tooltip(row.Branch).Grow(1).MinWidth(0)
							if row.Ahead > 0 {
								native.Text(c, fmt.Sprintf("↑%d", row.Ahead)).FontSize(12).TextColor(p.Success)
							}
							if row.Behind > 0 {
								native.Text(c, fmt.Sprintf("↓%d", row.Behind)).FontSize(12).TextColor(p.Secondary)
							}
						})
					}
				})
				if row.Status != "" && strings.ToLower(row.Status) != "unknown" {
					_, color, label := status(row.Status, c.Theme().Dark)
					stateIcon := icons.Circle
					if strings.EqualFold(row.Status, "working") {
						stateIcon = icons.CircleDot
					}
					native.Icon(c, stateIcon).Size(9, 9).TextColor(color).Tooltip(label).Label(row.Name + " · " + label)
				}
			})
		})
		if button.Clicked() {
			w.app.SelectWorkspace(row.ID)
			w.BlurInput()
		}
		if len(row.Ports) > 0 {
			label := portsLabel(row.Ports)
			native.Row(c.Key("ports:"+row.ID)).Height(22).FillWidth().Padding(0, 8, 0, 32).Gap(6).AlignItems(native.Center).Label("Listening ports for " + row.Name + ": " + label).Tooltip(label).Children(func() {
				native.Icon(c, icons.Globe).Size(12, 12).TextColor(p.Secondary)
				native.Text(c, label).FontSize(12).SingleLine().MinWidth(0).Grow(1).TextColor(p.Secondary)
			})
		}
	})
}
func agentDescription(row app.AgentRow) string {
	return row.WorkspaceName + " · " + row.Agent + " · " + statusLabel(row.Status) + " · " + row.PaneID
}
func (w *Window) agentGroup(c *native.Context, rows []app.AgentRow, p design.Palette) {
	native.Column(c.Key("agent-group:" + rows[0].WorkspaceID)).FillWidth().Gap(0).PaddingY(4).Label("Agents in " + rows[0].WorkspaceName).Children(func() {
		native.Text(c, rows[0].WorkspaceName).FontSize(13).Bold().SingleLine().PaddingX(12)
		for _, row := range rows {
			button := native.ButtonBase(c.Key("agent:"+row.PaneID)).FillWidth().Height(42).Padding(2, 12).Radius(6).Label(agentDescription(row)).Tooltip(agentDescription(row)).Selected(row.Selected).TextColor(p.Text)
			if row.Selected {
				button.Background(p.Selected)
			} else {
				w.controlState(button, p)
			}
			if button.FocusVisible() {
				button.Border(2, p.Focus)
			}
			button.Children(func() {
				native.Column(c).FillWidth().MinWidth(0).Gap(0).Children(func() {
					native.Row(c).FillWidth().Gap(7).AlignItems(native.Center).Children(func() {
						native.Icon(c, icons.Agent(row.Agent)).Size(14, 14).TextColor(p.Secondary)
						native.Text(c, row.Agent).FontSize(12).SingleLine().Grow(1).MinWidth(0)
					})
					_, color, label := status(row.Status, c.Theme().Dark)
					native.Text(c, label).FontSize(12).TextColor(color).Padding(0, 0, 0, 21)
				})
			})
			if button.Clicked() {
				w.app.SelectPane(row.PaneID)
				w.BlurInput()
			}
		}
	})
}

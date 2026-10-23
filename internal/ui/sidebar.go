package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
)

// sidebarFollows Herdr's sidebar language: a small-caps header over a
// quiet filter field, workspace rows that carry their own git and port
// details, a new/menu row, and an agents section whose groups mirror the
// workspaces above.
func (w *Window) sidebar(c *native.Context, v app.View, p design.Palette) {
	native.Column(c.Key("workspace-sidebar")).Label("Workspace sidebar").Width(w.sidebarWidth).Shrink(0).Background(p.Sidebar).AlignItems(native.Stretch).Children(func() {
		bar := c.TitleBar()
		native.Row(c).Height(36).Shrink(0).Padding(0, 8).AlignItems(native.Center).DragWindow().Children(func() {
			// Keep the title clear of the native traffic lights, which sit
			// over the sidebar's top-left corner.
			native.Box(c).Width(bar.Left).Shrink(0)
			native.Text(c, "Workspaces").FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted)
			native.Box(c).Grow(1)
			if w.iconButton(c, "hide-sidebar", icons.PanelLeft, "Hide sidebar", p).Clicked() {
				w.Dispatch("toggle_sidebar")
			}
		})
		native.Row(c).Shrink(0).FillWidth().Padding(0, 8, 6, 8).Children(func() {
			w.sidebarFilterField(c, p)
		})
		native.Scroll(c.Key("sidebar-information")).Grow(1).MinHeight(0).FillWidth().Label("Spaces and agents").Children(func() {
			native.Column(c).FillWidth().Padding(2, 8, 10, 8).Gap(14).Children(func() {
				rows, agents := w.filteredSidebar(v)
				native.Column(c).FillWidth().Gap(2).Children(func() {
					w.sidebarSectionHeader(c, "Spaces")
					for _, row := range rows {
						w.workspace(c, row, p)
					}
					if len(rows) == 0 {
						native.Text(c, "No matching spaces").FontSize(12).TextColor(p.Tertiary).Padding(2, 8)
					}
					w.sidebarNewMenuRow(c, p)
				})
				native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
				native.Column(c).FillWidth().Gap(2).Children(func() {
					native.Row(c).FillWidth().AlignItems(native.Center).Padding(4, 8, 0, 8).Children(func() {
						w.sidebarSectionHeader(c, "Agents").Grow(1)
						w.agentSortToggle(c, p)
					})
					if len(agents) == 0 {
						native.Text(c, "No agents").FontSize(12).TextColor(p.Tertiary).Padding(2, 8)
					} else if w.agentsGrouped {
						for i := 0; i < len(agents); {
							end := i + 1
							for end < len(agents) && agents[end].WorkspaceID == agents[i].WorkspaceID {
								end++
							}
							w.agentGroup(c, agents[i:end], p)
							i = end
						}
					} else {
						for _, row := range agents {
							w.agentRow(c, row, p)
						}
					}
				})
			})
		})
		native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
		native.Row(c).Height(30).Shrink(0).PaddingX(12).AlignItems(native.Center).Gap(7).Children(func() {
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

func (w *Window) sidebarSectionHeader(c *native.Context, title string) native.Element {
	return native.Text(c, title).FontSize(12).FontWeight(600).TextColor(c.Theme().TextMuted).Padding(4, 8)
}

// sidebarFilterField is the quiet "spaces" row that becomes a live search
// field when clicked, as Herdr's does: magnifier at the left, a clear
// control at the right, Escape leaves the field.
func (w *Window) sidebarFilterField(c *native.Context, p design.Palette) {
	if !w.sidebarFilterOn {
		idle := native.ButtonBase(c.Key("sidebar-filter")).FillWidth().Radius(6).Padding(5, 8).Label("Filter spaces").Tooltip("Filter spaces")
		w.controlState(idle, p)
		idle.Children(func() {
			native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
				native.Text(c, "spaces").FontSize(12).TextColor(p.Tertiary).Grow(1)
				native.Icon(c, icons.Search).Size(13, 13).TextColor(p.Tertiary)
			})
		})
		if idle.Clicked() {
			w.sidebarFilterOn = true
			w.sidebarFilterFocus = true
		}
		return
	}
	field := native.Row(c.Key("sidebar-filter-field")).FillWidth().Radius(6).Padding(5, 8).Gap(7).AlignItems(native.Center).Background(p.Inset)
	field.Label("Filter spaces")
	field.Children(func() {
		native.Icon(c, icons.Search).Size(13, 13).TextColor(p.Tertiary)
		input := native.TextInput(c.Key("sidebar-filter-input"), &w.sidebarFilter).FillWidth().Placeholder("Filter spaces, branches, agents").FontSize(13).TextColor(p.Text)
		input.Label("Filter spaces, branches, agents")
		if w.sidebarFilterFocus {
			input.Focus()
			w.sidebarFilterFocus = false
		}
		if input.Submitted() {
			w.BlurInput()
		}
		clear := native.ButtonBase(c.Key("sidebar-filter-clear")).Size(18, 18).Shrink(0).Radius(5).Center().Label("Clear filter").Tooltip("Clear filter")
		w.controlState(clear, p)
		clear.Children(func() { native.Icon(c, icons.X).Size(11, 11).TextColor(p.Secondary) })
		if clear.Clicked() {
			w.sidebarFilter = ""
			w.sidebarFilterFocus = true
		}
		if field.FocusWithin() && field.Shortcut(0, native.KeyEscape) {
			w.closeSidebarFilter()
		}
	})
}

func (w *Window) closeSidebarFilter() {
	w.sidebarFilterOn = false
	w.sidebarFilter = ""
	w.BlurInput()
}

// filteredSidebar narrows the sidebar's rows and agents to the filter
// text, matching names, branches, paths and agent names. An empty filter
// returns the view unchanged.
func (w *Window) filteredSidebar(v app.View) ([]app.WorkspaceRow, []app.AgentRow) {
	query := strings.ToLower(strings.TrimSpace(w.sidebarFilter))
	if query == "" {
		return v.Rows, v.Agents
	}
	match := func(parts ...string) bool {
		for _, part := range parts {
			if strings.Contains(strings.ToLower(part), query) {
				return true
			}
		}
		return false
	}
	var rows []app.WorkspaceRow
	for _, row := range v.Rows {
		if match(row.Name, row.Branch, row.Path) {
			rows = append(rows, row)
		}
	}
	var agents []app.AgentRow
	for _, agent := range v.Agents {
		if match(agent.Agent, agent.WorkspaceName) {
			agents = append(agents, agent)
		}
	}
	return rows, agents
}

// sidebarNewMenuRow closes the spaces section with Herdr's quiet text
// actions: a workspace is created immediately, and the menu opens the
// window-level commands in a small panel.
func (w *Window) sidebarNewMenuRow(c *native.Context, p design.Palette) {
	native.Row(c).FillWidth().Padding(6, 8, 2, 8).AlignItems(native.Center).Children(func() {
		newButton := native.ButtonBase(c.Key("new-workspace")).Radius(6).Padding(3, 8).Label("New workspace").Tooltip("New workspace")
		if w.Can("new_workspace") {
			w.controlState(newButton, p)
		} else {
			newButton.Opacity(.35)
		}
		newButton.Children(func() { native.Text(c, "new").FontSize(12).TextColor(p.Secondary) })
		if newButton.Clicked() {
			w.Dispatch("new_workspace")
		}
		native.Box(c).Grow(1)
		menuButton := native.ButtonBase(c.Key("sidebar-menu")).Radius(6).Padding(3, 8).Label("Sidebar menu").Tooltip("Sidebar menu")
		w.controlState(menuButton, p)
		menuButton.Children(func() { native.Text(c, "menu").FontSize(12).TextColor(p.Secondary) })
		if menuButton.Clicked() {
			w.sidebarMenuOpen = !w.sidebarMenuOpen
		}
		native.PopoverBase(c, menuButton, &w.sidebarMenuOpen, func(panel native.Element) {
			panel.MinWidth(190).Background(p.Elevated).Border(1, p.Border).Radius(10).Shadow(0, 4, 16, 0, p.ContentShadow).Padding(4)
			panel.Children(func() {
				w.sidebarMenuItem(c, p, "settings", icons.Settings, "Settings", func() { w.Dispatch("settings") })
				w.sidebarMenuItem(c, p, "menu-help", icons.Keyboard, "Keyboard reference", func() { w.Dispatch("help") })
				w.sidebarMenuItem(c, p, "menu-reload", icons.RotateCw, "Reload configuration", func() { w.Dispatch("reload_config") })
				w.sidebarMenuItem(c, p, "menu-detach", icons.Unplug, "Detach window", func() { w.Dispatch("detach") })
			})
		})
	})
}

func (w *Window) sidebarMenuItem(c *native.Context, p design.Palette, key string, icon *native.SVG, label string, run func()) {
	item := native.ButtonBase(c.Key(key)).FillWidth().Radius(6).Padding(6, 8).Label(label)
	w.controlState(item, p)
	item.Children(func() {
		native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
			native.Icon(c, icon).Size(14, 14).TextColor(p.Secondary)
			native.Text(c, label).FontSize(12).TextColor(p.Text)
		})
	})
	if item.Clicked() {
		w.sidebarMenuOpen = false
		run()
	}
}

// agentSortToggle cycles Herdr's two agents layouts: grouped under their
// workspaces, or one flat list ordered by attention priority.
func (w *Window) agentSortToggle(c *native.Context, p design.Palette) {
	mode := "grouped"
	if !w.agentsGrouped {
		mode = "priority"
	}
	button := native.ButtonBase(c.Key("agent-sort")).Radius(6).Padding(3, 8).Label("Sort agents: " + mode).Tooltip("Sort agents: " + mode)
	w.controlState(button, p)
	button.Children(func() { native.Text(c, mode).FontSize(12).TextColor(p.Secondary) })
	if button.Clicked() {
		w.agentsGrouped = !w.agentsGrouped
	}
}

// sidebarRowStyle paints a sidebar row's selection and interaction states.
// Selection is a translucent light layer over the sidebar material, as Herdr
// and macOS unfocused sidebars draw it.
func (w *Window) sidebarRowStyle(row native.Element, selected bool, p design.Palette) {
	if selected {
		row.Background(p.Selected)
	} else {
		w.controlState(row, p)
	}
	if row.FocusVisible() {
		row.Border(2, p.Focus)
	}
}

// sidebarRowPalette carries a sidebar row's colors for its selection state.
// A selected row keeps a plain title and marks itself through the fill and
// weight, as Herdr's sidebars do.
type sidebarRowPalette struct {
	title, secondary, muted, icon native.Color
	selected                      bool
}

func (w *Window) sidebarRowPalette(selected bool, p design.Palette) sidebarRowPalette {
	if selected {
		return sidebarRowPalette{title: p.Text, secondary: p.Accent, muted: p.Tertiary, icon: p.Text, selected: true}
	}
	return sidebarRowPalette{title: p.Text, secondary: p.Secondary, muted: p.Tertiary, icon: p.Secondary}
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

// statusGlyph is the small agent-state mark that leads every workspace and
// agent group row: a quiet dot until an agent reports in, then the state
// in its own color, as Herdr's ring does.
func statusGlyph(c *native.Context, value string, dark bool, fallback native.Color) {
	lower := strings.ToLower(value)
	if lower == "" || lower == "unknown" {
		native.Box(c).Size(4, 4).Radius(2).Background(fallback).Label(statusLabel(value))
		return
	}
	_, color, label := status(value, dark)
	icon := icons.Circle
	if lower == "working" {
		icon = icons.CircleDot
	}
	native.Icon(c, icon).Size(10, 10).TextColor(color).Label(label).Tooltip(label)
}

func (w *Window) workspace(c *native.Context, row app.WorkspaceRow, p design.Palette) {
	native.Column(c.Key("workspace:"+row.ID)).FillWidth().Padding(0, 0, 0, float32(row.Depth*14)).Children(func() {
		button := native.ButtonBase(c.Key("select:"+row.ID)).FillWidth().MinWidth(0).Radius(6).Padding(6, 8).Label(workspaceDescription(row)).Tooltip(row.Path).Selected(row.Selected)
		w.sidebarRowStyle(button, row.Selected, p)
		button.Children(func() {
			colors := w.sidebarRowPalette(row.Selected, p)
			native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
				native.Box(c).Width(12).Shrink(0).Center().Children(func() { statusGlyph(c, row.Status, c.Theme().Dark, p.Tertiary) })
				native.Icon(c, repositoryIcon(row)).Size(16, 16).TextColor(colors.icon)
				native.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					name := native.Text(c, row.Name).FontSize(13).SingleLine().Grow(1).MinWidth(0).TextColor(colors.title)
					if colors.selected {
						name.FontWeight(600)
					}
					if row.Branch != "" {
						native.Row(c).FillWidth().Gap(6).AlignItems(native.Center).Children(func() {
							native.Text(c, row.Branch).FontSize(11).TextColor(colors.secondary).SingleLine().Tooltip(row.Branch).Grow(1).MinWidth(0)
							if row.Ahead > 0 {
								native.Text(c, fmt.Sprintf("↑%d", row.Ahead)).FontSize(11).TextColor(p.Accent)
							}
							if row.Behind > 0 {
								native.Text(c, fmt.Sprintf("↓%d", row.Behind)).FontSize(11).TextColor(colors.secondary)
							}
						})
					}
				})
			})
		})
		if button.Clicked() {
			w.app.SelectWorkspace(row.ID)
			w.BlurInput()
		}
		// Herdr keeps the ports on a full-width band of their own below the
		// row, outside the selection highlight, aligned under the branch.
		if len(row.Ports) > 0 {
			label := portsLabel(row.Ports)
			native.Row(c.Key("ports:"+row.ID)).FillWidth().Padding(2, 8, 2, 52).Radius(5).Background(p.Inset).Gap(5).AlignItems(native.Center).Label("Listening ports for " + row.Name + ": " + label).Children(func() {
				native.Icon(c, icons.Globe).Size(11, 11).TextColor(p.Tertiary)
				native.Text(c, label).FontSize(11).SingleLine().MinWidth(0).Grow(1).TextColor(p.Tertiary).Tooltip(label)
			})
		}
	})
}
func agentDescription(row app.AgentRow) string {
	return row.WorkspaceName + " · " + row.Agent + " · " + statusLabel(row.Status) + " · " + row.PaneID
}

// agentRank orders the agent lifecycles by how much they need attention,
// the ordering Herdr's priority layout uses.
func agentRank(status string) int {
	switch strings.ToLower(status) {
	case "blocked":
		return 0
	case "working":
		return 1
	case "done":
		return 2
	case "idle":
		return 3
	}
	return 4
}

func sortAgentsByPriority(rows []app.AgentRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if agentRank(rows[i].Status) != agentRank(rows[j].Status) {
			return agentRank(rows[i].Status) < agentRank(rows[j].Status)
		}
		if rows[i].WorkspaceName != rows[j].WorkspaceName {
			return rows[i].WorkspaceName < rows[j].WorkspaceName
		}
		return rows[i].Agent < rows[j].Agent
	})
}

func (w *Window) agentGroup(c *native.Context, rows []app.AgentRow, p design.Palette) {
	native.Column(c.Key("agent-group:"+rows[0].WorkspaceID)).FillWidth().Gap(2).Padding(6, 0, 0, 0).Label("Agents in " + rows[0].WorkspaceName).Children(func() {
		native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Padding(2, 8).Children(func() {
			native.Box(c).Width(12).Shrink(0).Center().Children(func() {
				best := rows[0].Status
				for _, row := range rows[1:] {
					if agentRank(row.Status) < agentRank(best) {
						best = row.Status
					}
				}
				statusGlyph(c, best, c.Theme().Dark, p.Tertiary)
			})
			native.Text(c, rows[0].WorkspaceName).FontSize(12).FontWeight(600).SingleLine().TextColor(c.Theme().TextMuted)
			native.Textf(c, "· %d", len(rows)).FontSize(12).TextColor(c.Theme().TextMuted)
		})
		for _, row := range rows {
			w.agentRow(c, row, p)
		}
	})
}

func (w *Window) agentRow(c *native.Context, row app.AgentRow, p design.Palette) {
	button := native.ButtonBase(c.Key("agent:"+row.PaneID)).FillWidth().Radius(6).Padding(6, 8).Label(agentDescription(row)).Tooltip(agentDescription(row)).Selected(row.Selected)
	w.sidebarRowStyle(button, row.Selected, p)
	button.Children(func() {
		colors := w.sidebarRowPalette(row.Selected, p)
		_, color, label := status(row.Status, c.Theme().Dark)
		native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.Agent(row.Agent)).Size(15, 15).TextColor(colors.icon)
			native.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				agent := native.Text(c, row.Agent).FontSize(13).SingleLine().TextColor(colors.title)
				if colors.selected {
					agent.FontWeight(600)
				}
				detail := native.Row(c).Gap(5).AlignItems(native.Center)
				if !w.agentsGrouped {
					detail.Children(func() {
						native.Text(c, row.WorkspaceName).FontSize(11).SingleLine().TextColor(colors.secondary)
						native.Text(c, "·").FontSize(11).TextColor(colors.muted)
					})
				}
				detail.Children(func() {
					native.Text(c, label).FontSize(11).TextColor(color)
				})
			})
		})
	})
	if button.Clicked() {
		w.app.SelectPane(row.PaneID)
		w.BlurInput()
	}
}

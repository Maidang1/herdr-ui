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

// Every sidebar row shares one 8pt inset, so workspace names, branches,
// group titles and section headers all start on the same column; an
// agent's mark sits on it with its name a gutter beyond.
const (
	sidebarRowInset float32 = 8
	sidebarGutter   float32 = 8
)

// sidebar groups workspace metadata and agents on a compact, shared grid.
// It draws no surface of its own: it sits on the window's background, as a
// Mac sidebar does, and only the content beside it is a panel.
func (w *Window) sidebar(c *native.Context, v app.View, p design.Palette) {
	native.Column(c.Key("workspace-sidebar")).Label("Workspace sidebar").FillHeight().Width(w.sidebarWidth).Shrink(0).AlignItems(native.Stretch).Children(func() {
		bar := c.TitleBar()
		native.Row(c).Height(38).Shrink(0).Padding(0, 8).AlignItems(native.Center).DragWindow().Children(func() {
			// The native traffic lights sit over the sidebar's top-left
			// corner; the strip leaves them their space and keeps only the
			// sidebar toggle, since the sections below name themselves.
			native.Box(c).Width(bar.Left).Shrink(0)
			native.Box(c).Grow(1)
			if w.iconButton(c, "hide-sidebar", icons.PanelLeft, "Hide sidebar", p).Clicked() {
				w.Dispatch("toggle_sidebar")
			}
		})
		native.Row(c).Shrink(0).FillWidth().Padding(2, 10, 6, 10).Children(func() {
			w.sidebarFilterField(c, p)
		})
		rows, agents := w.filteredSidebar(v)
		// Keep scrollbar tracks outside the clipped sections; wheel and
		// touchpad scrolling still use the full list area.
		native.Column(c.Key("sidebar-information")).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
			native.Column(c.Key("sidebar-spaces-section")).Grow(1).Basis(0).MinHeight(0).FillWidth().PaddingX(8).Clip().Children(func() {
				w.sidebarSectionHeader(c, "Spaces", len(rows), p, func() { w.newWorkspaceButton(c, p) })
				native.Scroll(c.Key("sidebar-spaces")).Grow(1).Basis(0).MinHeight(0).FillWidth().ScrollbarInsets(0, -w.sidebarWidth, 0, -w.sidebarWidth).Label("Spaces list").Children(func() {
					native.Column(c).FillWidth().Padding(0, 0, 8, 0).Gap(2).Children(func() {
						for _, row := range rows {
							w.workspace(c, row, p)
						}
						if len(rows) == 0 {
							w.sidebarEmpty(c, "No matching spaces", p)
						}
					})
				})
			})
			native.Column(c.Key("sidebar-agents-section")).Grow(1).Basis(0).MinHeight(0).FillWidth().PaddingX(8).Clip().Children(func() {
				w.sidebarSectionHeader(c, "Agents", len(agents), p, func() {
					w.attentionButton(c, v.Agents, p)
					w.agentSortToggle(c, p)
				})
				native.Scroll(c.Key("sidebar-agents")).Grow(1).Basis(0).MinHeight(0).FillWidth().ScrollbarInsets(0, -w.sidebarWidth, 0, -w.sidebarWidth).Label("Agents list").Children(func() {
					native.Column(c).FillWidth().Padding(0, 0, 12, 0).Gap(2).Children(func() {
						if len(agents) == 0 {
							w.sidebarEmpty(c, "No agents", p)
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
		})
		native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
		native.Row(c).Height(32).Shrink(0).Padding(0, 6, 0, 10).AlignItems(native.Center).Gap(2).Children(func() {
			w.sidebarUpdate(c, p)
			if w.iconButton(c, "keyboard-reference-entry", icons.Keyboard, "Keyboard reference", p).Clicked() {
				w.Dispatch("help")
			}
			w.sidebarMenu(c, p)
		})
		w.sidebarResizeHandle(c)
	})
}

// sidebarSectionHeader titles a section with its item count, and places the
// section's one action at the right edge of the same line.
func (w *Window) sidebarSectionHeader(c *native.Context, title string, count int, p design.Palette, action func()) {
	native.Row(c).FillWidth().Height(28).Shrink(0).Padding(0, 2, 0, sidebarRowInset).Gap(6).AlignItems(native.Center).Children(func() {
		native.Text(c, title).FontSize(11).FontWeight(600).TextColor(p.Secondary)
		if count > 0 {
			native.Textf(c, "%d", count).FontSize(11).TextColor(p.Tertiary.Alpha(.75))
		}
		native.Box(c).Grow(1)
		action()
	})
}

func (w *Window) sidebarEmpty(c *native.Context, text string, p design.Palette) {
	native.Text(c, text).FontSize(12).TextColor(p.Tertiary).Padding(4, sidebarRowInset)
}

// sidebarFilterField becomes a live search field when clicked; Escape leaves it.
func (w *Window) sidebarFilterField(c *native.Context, p design.Palette) {
	if !w.sidebarFilterOn {
		idle := native.ButtonBase(c.Key("sidebar-filter")).FillWidth().Height(28).Radius(14).Padding(0, 10).Background(p.Inset).Border(1, p.Border).Label("Filter spaces")
		w.controlState(idle, p)
		idle.Children(func() {
			native.Row(c).FillWidth().Gap(7).AlignItems(native.Center).Children(func() {
				native.Icon(c, icons.Search).Size(13, 13).TextColor(p.Tertiary)
				native.Text(c, "Search").FontSize(12).TextColor(p.Tertiary).Grow(1)
			})
		})
		if idle.Clicked() {
			w.sidebarFilterOn = true
			w.sidebarFilterFocus = true
		}
		return
	}
	field := native.Row(c.Key("sidebar-filter-field")).FillWidth().Height(28).Radius(14).Padding(0, 4, 0, 10).Gap(7).AlignItems(native.Center).Background(p.Inset).Border(1, p.Focus.Alpha(.5))
	field.Label("Filter spaces")
	field.Children(func() {
		native.Icon(c, icons.Search).Size(13, 13).TextColor(p.Tertiary)
		input := native.TextInputBase(c.Key("sidebar-filter-input"), &w.sidebarFilter).FillWidth().Placeholder("Spaces, branches, agents").FontSize(12).TextColor(p.Text)
		input.Label("Filter spaces, branches, agents")
		if w.sidebarFilterFocus {
			input.Focus()
			w.sidebarFilterFocus = false
		}
		if input.Submitted() {
			w.BlurInput()
		}
		clear := native.ButtonBase(c.Key("sidebar-filter-clear")).Size(20, 20).Shrink(0).Radius(10).Center().Label("Clear filter")
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

// newWorkspaceButton is the spaces header's action: a workspace is created
// immediately, as Herdr's quiet "+" does.
func (w *Window) newWorkspaceButton(c *native.Context, p design.Palette) {
	button := native.ButtonBase(c.Key("new-workspace")).Size(24, 24).Shrink(0).Radius(12).Center().Label("New workspace")
	can := w.Can("new_workspace")
	if can {
		w.controlState(button, p)
	} else {
		button.Opacity(.35)
	}
	button.Children(func() { native.Icon(c, icons.Plus).Size(14, 14).TextColor(p.Secondary) })
	if can && button.Clicked() {
		w.Dispatch("new_workspace")
	}
}

// sidebarUpdate offers Settings until a background check finds a newer
// version, then gives the update the leading position in the footer.
func (w *Window) sidebarUpdate(c *native.Context, p design.Palette) {
	var notice UpdateNotice
	if w.hooks.UpdateNotice != nil {
		notice = w.hooks.UpdateNotice()
	}
	if notice.Available == "" {
		button := native.ButtonBase(c.Key("sidebar-settings")).Height(28).Radius(6).Padding(0, 8).Label("Open settings")
		w.controlState(button, p)
		button.Children(func() {
			native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
				native.Icon(c, icons.Settings).Size(14, 14).TextColor(p.Secondary)
				native.Text(c, "Settings").FontSize(12).TextColor(p.Secondary)
			})
		})
		if button.Clicked() {
			w.Dispatch("settings")
		}
		native.Box(c).Grow(1).MinWidth(0)
		return
	}
	label := "Update to " + notice.Available
	button := native.ButtonBase(c.Key("sidebar-update")).Grow(1).MinWidth(0).Height(26).Radius(8).Padding(0, 8).Label(label)
	w.controlState(button, p)
	button.Background(p.Accent.Alpha(.14))
	button.Children(func() {
		native.Row(c).FillWidth().MinWidth(0).Gap(6).AlignItems(native.Center).Children(func() {
			native.Box(c).Size(7, 7).Shrink(0).Radius(3.5).Background(p.Accent)
			native.Text(c, label).FontSize(11).FontWeight(500).SingleLine().Ellipsis("…").MinWidth(0).TextColor(p.Text)
		})
	})
	if button.Clicked() && w.hooks.CheckForUpdates != nil {
		w.hooks.CheckForUpdates()
	}
}

// sidebarMenu opens the window-level commands in a small panel from the
// sidebar's footer.
func (w *Window) sidebarMenu(c *native.Context, p design.Palette) {
	menuButton := native.ButtonBase(c.Key("sidebar-menu")).Size(28, 28).Shrink(0).Radius(14).Center().Label("Sidebar menu")
	w.controlState(menuButton, p)
	menuButton.Children(func() { native.Icon(c, icons.Ellipsis).Size(16, 16).TextColor(p.Secondary) })
	if menuButton.Clicked() {
		w.sidebarMenuOpen = !w.sidebarMenuOpen
	}
	native.PopoverBase(c, menuButton, &w.sidebarMenuOpen, func(panel native.Element) {
		w.popoverPanel(c, panel, p)
		panel.Children(func() {
			w.sidebarMenuItem(c, p, "settings", icons.Settings, "Settings", func() { w.Dispatch("settings") })
			w.sidebarMenuItem(c, p, "menu-help", icons.Keyboard, "Keyboard reference", func() { w.Dispatch("help") })
			w.sidebarMenuItem(c, p, "menu-reload", icons.RotateCw, "Reload configuration", func() { w.Dispatch("reload_config") })
			w.sidebarMenuItem(c, p, "menu-detach", icons.Unplug, "Detach window", func() { w.Dispatch("detach") })
		})
	})
}

func (w *Window) sidebarMenuItem(c *native.Context, p design.Palette, key string, icon *native.SVG, label string, run func()) {
	item := native.ButtonBase(c.Key(key)).FillWidth().Radius(w.shape.MenuItem).Padding(6, 8).Label(label)
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

// attentionButton surfaces the agents waiting for a reply, which Herdr
// otherwise reaches only by its jump key: one click goes to the next.
func (w *Window) attentionButton(c *native.Context, agents []app.AgentRow, p design.Palette) {
	waiting := 0
	for _, row := range agents {
		if strings.EqualFold(row.Status, "blocked") {
			waiting++
		}
	}
	if waiting == 0 {
		return
	}
	label := fmt.Sprintf("%d waiting", waiting)
	hint := "Jump to agent needing you"
	if binding := w.BindingLabel("attention"); binding != "" {
		hint += " · " + keyGlyphs(binding)
	}
	button := native.ButtonBase(c.Key("attention-jump")).Height(22).Radius(11).Padding(0, 8).Background(p.Warning.Alpha(.14)).Label(label + " · " + hint)
	if button.Hovered() {
		button.Background(p.Warning.Alpha(.22))
	}
	if button.FocusVisible() {
		button.Border(2, p.Focus)
	}
	button.Children(func() {
		native.Row(c).Gap(5).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.MessageCircle).Size(11, 11).TextColor(p.Warning)
			native.Text(c, label).FontSize(11).FontWeight(500).TextColor(p.Warning)
		})
	})
	if button.Clicked() {
		w.Dispatch("attention")
	}
}

// agentSortToggle cycles Herdr's two agents layouts: grouped under their
// workspaces, or one flat list ordered by attention priority.
func (w *Window) agentSortToggle(c *native.Context, p design.Palette) {
	mode, title := "grouped", "Grouped"
	if !w.agentsGrouped {
		mode, title = "priority", "Priority"
	}
	button := native.ButtonBase(c.Key("agent-sort")).Height(22).Radius(11).Padding(0, 6, 0, 8).Label("Sort agents: " + mode)
	w.controlState(button, p)
	button.Children(func() {
		native.Row(c).Gap(3).AlignItems(native.Center).Children(func() {
			native.Text(c, title).FontSize(11).TextColor(p.Tertiary)
			native.Icon(c, icons.ChevronDown).Size(12, 12).TextColor(p.Tertiary)
		})
	})
	if button.Clicked() {
		w.agentsGrouped = !w.agentsGrouped
	}
}

// sidebarRowStyle paints a sidebar row's selection and interaction states:
// a borderless wash in the window's own lavender hue while the window is
// active, the neutral selection while it is not.
func (w *Window) sidebarRowStyle(row native.Element, selected bool, p design.Palette) {
	if selected {
		if w.appearance.active {
			row.Background(p.RowSelected)
		} else {
			row.Background(p.Selected)
		}
	} else {
		w.controlState(row, p)
	}
	if row.FocusVisible() {
		row.Border(2, p.Focus)
	}
}

// portsLabel lists every listening port, for the row's accessible label.
func portsLabel(ports []int) string {
	labels := make([]string, len(ports))
	for i, port := range ports {
		labels[i] = ":" + strconv.Itoa(port)
	}
	return strings.Join(labels, " ")
}

// portsSummary is the row's compact form: the first port and how many more.
func portsSummary(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	label := ":" + strconv.Itoa(ports[0])
	if len(ports) > 1 {
		label += fmt.Sprintf(" +%d", len(ports)-1)
	}
	return label
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

// workspaceStatus marks a workspace's agent state when it asks for
// attention (working, waiting, done); an idle or silent workspace stays
// unmarked.
func (w *Window) workspaceStatus(c *native.Context, row app.WorkspaceRow, p design.Palette) {
	if agentRank(row.Status) >= 3 {
		return
	}
	statusMark(c, "workspace:"+row.ID, row.Status, 10, p)
}

// workspace is one space on the shared two-line grid: name and agent state
// on top; branch, ahead/behind and listening ports beneath.
func (w *Window) workspace(c *native.Context, row app.WorkspaceRow, p design.Palette) {
	native.Column(c.Key("workspace:"+row.ID)).FillWidth().Padding(0, 0, 0, float32(row.Depth*14)).Children(func() {
		button := native.ButtonBase(c.Key("select:"+row.ID)).FillWidth().MinWidth(0).Radius(w.shape.Row).Padding(6, sidebarRowInset).Label(workspaceDescription(row)).Selected(row.Selected)
		w.sidebarRowStyle(button, row.Selected, p)
		button.Children(func() {
			native.Row(c).FillWidth().MinHeight(32).AlignItems(native.Center).Children(func() {
				native.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					native.Row(c).FillWidth().Gap(6).AlignItems(native.Center).Children(func() {
						native.Text(c, row.Name).FontSize(13).FontWeight(500).SingleLine().Grow(1).MinWidth(0).TextColor(p.Text)
						w.workspaceStatus(c, row, p)
					})
					if row.Branch == "" && len(row.Ports) == 0 {
						return
					}
					native.Row(c).FillWidth().Gap(6).AlignItems(native.Center).Children(func() {
						native.Text(c, row.Branch).FontSize(11).TextColor(p.Secondary).SingleLine().Grow(1).MinWidth(0)
						if row.Ahead > 0 {
							native.Text(c, fmt.Sprintf("↑%d", row.Ahead)).FontSize(11).TextColor(p.Success).Shrink(0)
						}
						if row.Behind > 0 {
							native.Text(c, fmt.Sprintf("↓%d", row.Behind)).FontSize(11).TextColor(p.Warning).Shrink(0)
						}
						if len(row.Ports) > 0 {
							label := portsLabel(row.Ports)
							native.Row(c.Key("ports:" + row.ID)).Shrink(0).Gap(3).AlignItems(native.Center).Label("Listening ports for " + row.Name + ": " + label).Children(func() {
								native.Icon(c, icons.Globe).Size(10, 10).TextColor(p.Success)
								native.Text(c, portsSummary(row.Ports)).FontSize(11).SingleLine().TextColor(p.Success)
							})
						}
					})
				})
			})
		})
		button.ContextMenu(func(m *native.Menu) { w.workspaceMenu(m, row) })
		if button.DoubleClicked() {
			w.app.SelectWorkspace(row.ID)
			w.Dispatch("rename_workspace")
		} else if button.Clicked() {
			w.app.SelectWorkspace(row.ID)
			w.BlurInput()
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

// agentGroup titles a workspace's agents with a quiet caption on the name
// column, so the rows below read as its children.
func (w *Window) agentGroup(c *native.Context, rows []app.AgentRow, p design.Palette) {
	native.Column(c.Key("agent-group:" + rows[0].WorkspaceID)).FillWidth().Gap(2).Label("Agents in " + rows[0].WorkspaceName).Children(func() {
		native.Row(c).FillWidth().Height(22).Padding(0, sidebarRowInset).Gap(6).AlignItems(native.Center).Children(func() {
			native.Text(c, rows[0].WorkspaceName).FontSize(11).SingleLine().Grow(1).MinWidth(0).TextColor(p.Tertiary)
			if len(rows) > 1 {
				native.Textf(c, "%d", len(rows)).FontSize(11).TextColor(p.Tertiary.Alpha(.75))
			}
		})
		for _, row := range rows {
			w.agentRow(c, row, p)
		}
	})
}

// agentRow is one agent on the sidebar grid with its state at the right:
// a colored dot and word, quiet while the agent idles.
func (w *Window) agentRow(c *native.Context, row app.AgentRow, p design.Palette) {
	button := native.ButtonBase(c.Key("agent:"+row.PaneID)).FillWidth().Radius(w.shape.Row).Padding(5, sidebarRowInset).Label(agentDescription(row)).Selected(row.Selected)
	w.sidebarRowStyle(button, row.Selected, p)
	button.Children(func() {
		_, color, label := statusFromPalette(row.Status, p)
		native.Row(c).FillWidth().Gap(sidebarGutter).AlignItems(native.Center).Children(func() {
			native.Box(c).Size(18, 18).Shrink(0).Center().Children(func() {
				agentMark(c, row.Agent, 15, p.Secondary)
			})
			native.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
				native.Text(c, row.Agent).FontSize(13).FontWeight(500).SingleLine().TextColor(p.Text)
				if !w.agentsGrouped {
					native.Text(c, row.WorkspaceName).FontSize(11).SingleLine().TextColor(p.Secondary)
				}
			})
			text := p.Tertiary
			if agentRank(row.Status) < 3 {
				text = color
			}
			native.Row(c).Shrink(0).Gap(5).AlignItems(native.Center).Children(func() {
				statusMark(c, "agent:"+row.PaneID, row.Status, 10, p)
				native.Text(c, label).FontSize(11).TextColor(text)
			})
		})
	})
	button.ContextMenu(func(m *native.Menu) { w.paneMenu(m, row.PaneID) })
	if button.Clicked() {
		w.app.SelectPane(row.PaneID)
		w.BlurInput()
	}
}

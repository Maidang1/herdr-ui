package ui

import (
	"strings"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
)

// tabBar is the selected workspace's tab strip at the top of the content
// area, level with the sidebar header: pills sit directly on the shell, each
// showing its agent state and close control, with a control to open one
// more and the tab-actions menu on the right. With the sidebar hidden the
// strip lives in the title line beside the traffic lights instead.
func (w *Window) tabBar(c *native.Context, v app.View, p design.Palette) {
	if !w.sidebarVisible {
		return
	}
	native.Row(c.Key("tab-bar")).Label("Tab bar").FillWidth().Height(36).Shrink(0).Padding(0, 4, 0, 4).AlignItems(native.Center).Gap(2).Children(func() {
		w.tabStripControls(c, v, p, false)
	})
}

// tabStripControls lays out the shared strip content: pills and a new-tab
// control on the left; on the right the workspace context (only in the
// title line, where no sidebar names the workspace), the prefix key tag
// while a chord is awaited, and the tab-actions menu.
func (w *Window) tabStripControls(c *native.Context, v app.View, p design.Palette, inTitleBar bool) {
	for i, tab := range v.Tabs {
		if i > 0 && !tab.Selected && !v.Tabs[i-1].Selected {
			native.Box(c).Width(1).Height(16).Shrink(0).MarginX(2).Background(p.Border)
		}
		w.tabPill(c, tab, p)
	}
	if len(v.Tabs) > 0 {
		native.Box(c).Width(1).Height(16).Shrink(0).MarginX(2).Background(p.Border)
	}
	w.tabOpenControl(c, p)
	native.Box(c).Grow(1).MinWidth(0).DragWindow()
	if inTitleBar {
		if context := tabStripContext(v); context != "" {
			name := native.Text(c, context).FontSize(12).TextColor(p.Secondary).SingleLine().MinWidth(0).MaxWidth(320).Shrink(1)
			if v.Path != "" {
				name.Tooltip(v.Path)
			}
		}
	}
	w.prefixChip(c, p)
	w.tabActionsMenu(c, p)
}

// prefixChip marks an awaited prefix chord with its own key, so the strip
// shows "Ctrl + B" rather than the word Prefix.
func (w *Window) prefixChip(c *native.Context, p design.Palette) {
	a := w.controller
	if a == nil || !a.machine.Awaiting {
		return
	}
	label := prefixKeyLabel(a.machine.Bindings.Prefix)
	chip := native.Row(c.Key("prefix-chip")).Shrink(0).Height(20).Radius(6).Padding(0, 7).Gap(5).AlignItems(native.Center).Background(p.Warning.Alpha(.18))
	chip.Label("Prefix " + label + " · press an action key").Tooltip("Prefix " + label + " · press an action key")
	chip.Children(func() {
		native.Icon(c, icons.Keyboard).Size(11, 11).TextColor(p.Warning)
		native.Text(c, label).FontSize(11).TextColor(p.Warning)
	})
}

// prefixKeyLabel renders a binding label such as "ctrl+b" as "Ctrl + B".
func prefixKeyLabel(prefix string) string {
	parts := strings.Split(prefix, "+")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " + ")
}

// tabStripContext names what the strip serves: the workspace, and the agent
// focused inside it when one is running.
func tabStripContext(v app.View) string {
	for _, pane := range v.Panes {
		if pane.Selected && pane.Agent != "" {
			return v.WorkspaceName + " · " + pane.Agent
		}
	}
	return v.WorkspaceName
}

// tabPill is one tab in the strip. The close control sits inside the pill;
// the innermost element takes the click.
func (w *Window) tabPill(c *native.Context, tab app.TabRow, p design.Palette) {
	label := "Tab " + tab.Name
	pill := native.ButtonBase(c.Key("tab:"+tab.ID)).Shrink(0).Height(26).Radius(7).Padding(0, 4, 0, 8).Label(label).Tooltip(label).Selected(tab.Selected)
	if tab.Selected {
		pill.Background(p.Selected)
	} else {
		w.controlState(pill, p)
	}
	pill.Children(func() {
		native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
			if tab.Status != "" {
				_, color, stateLabel := status(tab.Status, c.Theme().Dark)
				native.Icon(c, tabStatusIcon(tab.Status)).Size(10, 10).TextColor(color).Label(tab.Name + " · " + stateLabel).Tooltip(stateLabel)
			}
			name := native.Text(c, tab.Name).FontSize(13).SingleLine().MaxWidth(180).TextColor(p.Secondary)
			if tab.Selected {
				name.TextColor(p.Text).FontWeight(500)
			}
			close := native.ButtonBase(c.Key("close-tab:"+tab.ID)).Size(18, 18).Shrink(0).Radius(5).Center().Label("Close tab " + tab.Name).Tooltip("Close tab " + tab.Name)
			w.controlState(close, p)
			close.Children(func() { native.Icon(c, icons.X).Size(11, 11).TextColor(p.Secondary) })
			if close.Clicked() {
				if !tab.Selected {
					w.app.SelectTab(tab.ID)
				}
				w.Dispatch("close_tab")
			}
		})
	})
	if pill.Clicked() {
		w.app.SelectTab(tab.ID)
		w.BlurInput()
	}
}

// tabStatusIcon keeps the pill's mark a compact ring-like glyph, as Herdr's
// tab state does; only active work earns the filled ring.
func tabStatusIcon(value string) *native.SVG {
	if value == "working" {
		return icons.CircleDot
	}
	return icons.Circle
}

func (w *Window) tabOpenControl(c *native.Context, p design.Palette) {
	can := w.Can("new_tab")
	plus := native.ButtonBase(c.Key("new-tab")).Size(26, 26).Shrink(0).Radius(7).Center().Label("New tab").Tooltip("New tab")
	if can {
		w.controlState(plus, p)
	} else {
		plus.Opacity(.35)
	}
	plus.Children(func() { native.Icon(c, icons.Plus).Size(15, 15).TextColor(p.Secondary) })
	if can && plus.Clicked() {
		w.Dispatch("new_tab")
	}
}

// tabActionsMenu gathers the tab's commands behind Herdr's quiet "···"
// control at the strip's right edge.
func (w *Window) tabActionsMenu(c *native.Context, p design.Palette) {
	more := native.ButtonBase(c.Key("tab-actions")).Size(26, 26).Shrink(0).Radius(7).Center().Label("Tab actions").Tooltip("Tab actions")
	w.controlState(more, p)
	more.Children(func() { native.Icon(c, icons.Ellipsis).Size(15, 15).TextColor(p.Secondary) })
	if more.Clicked() {
		w.tabMenuOpen = !w.tabMenuOpen
	}
	native.PopoverBase(c, more, &w.tabMenuOpen, func(panel native.Element) {
		panel.MinWidth(190).Background(p.Elevated).Border(1, p.Border).Radius(10).Shadow(0, 4, 16, 0, p.ContentShadow).Padding(4)
		panel.Children(func() {
			w.tabMenuItem(c, p, "tab-menu-new", icons.Plus, "New tab", "new_tab")
			w.tabMenuItem(c, p, "tab-menu-rename", icons.GitBranch, "Rename tab", "rename_tab")
			w.tabMenuItem(c, p, "tab-menu-split-right", icons.Columns2, "Split right", "split_vertical")
			w.tabMenuItem(c, p, "tab-menu-split-down", icons.Rows2, "Split down", "split_horizontal")
			w.tabMenuItem(c, p, "tab-menu-close", icons.X, "Close tab", "close_tab")
		})
	})
}

func (w *Window) tabMenuItem(c *native.Context, p design.Palette, key string, icon *native.SVG, label, action string) {
	can := w.Can(action)
	item := native.ButtonBase(c.Key(key)).FillWidth().Radius(6).Padding(6, 8).Label(label)
	if can {
		w.controlState(item, p)
	} else {
		item.Opacity(.35)
	}
	item.Children(func() {
		native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
			native.Icon(c, icon).Size(14, 14).TextColor(p.Secondary)
			native.Text(c, label).FontSize(12).TextColor(p.Text)
		})
	})
	if can && item.Clicked() {
		w.tabMenuOpen = false
		w.Dispatch(action)
	}
}

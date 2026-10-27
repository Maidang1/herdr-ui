package ui

import (
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
)

// toolbarHeight keeps the toolbar's controls centered on the traffic
// lights' own line (measured center y=26) when the toolbar holds them.
const toolbarHeight float32 = 36

// toolbar is the top bar of the content panel, on the panel's own white:
// the selected workspace's tabs and a control to open one more on the
// left; on the right the single pane's caption and layout controls, the
// prefix key tag while a chord is awaited, and the tab-actions menu. With
// the sidebar hidden it also makes room for the traffic lights and offers
// the sidebar back, naming the workspace the sidebar no longer shows. Zen
// mode draws no toolbar: the lights float over the terminal, as Herdr's
// zen does.
func (w *Window) toolbar(c *native.Context, v app.View, p design.Palette) {
	if w.zen {
		return
	}
	hidden := !w.sidebarVisible
	if len(v.Tabs) == 0 && !hidden {
		return
	}
	bar := native.Row(c.Key("tab-bar")).FillWidth().Height(toolbarHeight).Shrink(0).Padding(0, 6, 0, 10).Gap(2).AlignItems(native.Center).DragWindow()
	if len(v.Tabs) > 0 {
		bar.Label("Tab bar")
	}
	bar.Children(func() {
		if hidden {
			// The panel starts one gap in from the window's edge, where
			// the title bar's inset is measured from.
			native.Box(c).Width(max(0, c.TitleBar().Left-w.shape.Gap-10)).Shrink(0)
			if w.iconButton(c, "show-sidebar", icons.PanelLeft, "Show sidebar", p).Clicked() {
				w.Dispatch("toggle_sidebar")
			}
			native.Box(c).Width(4).Shrink(0)
		}
		for i, tab := range v.Tabs {
			if i > 0 && !tab.Selected && !v.Tabs[i-1].Selected {
				native.Box(c).Width(1).Height(14).Shrink(0).MarginX(2).Background(p.Border)
			}
			w.tabPill(c, tab, p)
		}
		if len(v.Tabs) > 0 {
			w.tabOpenControl(c, p)
		}
		native.Box(c).Grow(1).MinWidth(0).DragWindow()
		if hidden {
			if context := tabStripContext(v); context != "" {
				native.Text(c, context).FontSize(12).TextColor(p.Secondary).SingleLine().MinWidth(0).MaxWidth(320).Shrink(1).Padding(0, 6)
			}
		}
		w.prefixChip(c, p)
		w.singlePaneControls(c, v, p)
		if len(v.Tabs) > 0 {
			w.tabActionsMenu(c, p)
		}
	})
	native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
}

// singlePane reports whether the tab shows one terminal, alone or zoomed:
// it then has no card of its own, and its controls join the toolbar.
func (w *Window) singlePane(v app.View) bool {
	return w.terminal != nil && v.HasLayout && (len(v.Layout.Panes) <= 1 || v.Layout.Zoomed)
}

// singlePaneControls names the single pane's agent and offers its layout
// controls at the toolbar's trailing edge, where a split tab's cards carry
// their own.
func (w *Window) singlePaneControls(c *native.Context, v app.View, p design.Palette) {
	if !w.singlePane(v) {
		return
	}
	for _, pane := range v.Panes {
		if pane.ID == v.PaneID {
			w.paneToolbarControls(c, pane, v, p)
			return
		}
	}
}

func (w *Window) paneToolbarControls(c *native.Context, pane app.PaneRow, v app.View, p design.Palette) {
	caption := pane.Label
	if caption == "" {
		caption = pane.Agent
	}
	if caption != "" {
		native.Row(c.Key("toolbar-pane-caption")).Shrink(1).MinWidth(0).Gap(6).Padding(0, 8).AlignItems(native.Center).Label("Pane " + caption).Children(func() {
			agentMark(c, pane.Agent, 13, p.Secondary)
			native.Text(c, caption).FontSize(12).SingleLine().MinWidth(0).TextColor(p.Secondary)
		})
	}
	native.Row(c.Key("toolbar-pane-controls")).Shrink(0).Gap(2).AlignItems(native.Center).Children(func() {
		w.cardControls(c, pane, v, 1000, p)
	})
	native.Box(c).Width(1).Height(14).Shrink(0).MarginX(4).Background(p.Border)
}

// prefixChip marks an awaited prefix chord with its own key, so the strip
// shows "Ctrl + B" rather than the word Prefix.
func (w *Window) prefixChip(c *native.Context, p design.Palette) {
	a := w.controller
	if a == nil || !a.machine.Awaiting {
		return
	}
	label := prefixKeyLabel(a.machine.Bindings.Prefix)
	chip := native.Row(c.Key("prefix-chip")).Shrink(0).Height(20).Radius(10).Padding(0, 7).Gap(5).AlignItems(native.Center).Background(p.Warning.Alpha(.18))
	chip.Label("Prefix " + label + " · press an action key")
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
	pill := native.ButtonBase(c.Key("tab:"+tab.ID)).Shrink(0).Height(26).Radius(13).Padding(0, 4, 0, 10).Label(label).Selected(tab.Selected)
	if tab.Selected {
		pill.Background(p.Selected)
	} else {
		w.controlState(pill, p)
	}
	pill.Children(func() {
		native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
			if tab.Status != "" {
				_, _, stateLabel := status(tab.Status, c.Theme().Dark)
				statusMark(c, "tab:"+tab.ID, tab.Status, 10, p).Label(tab.Name + " · " + stateLabel)
			}
			name := native.Text(c, tab.Name).FontSize(13).SingleLine().MaxWidth(180).TextColor(p.Secondary)
			if tab.Selected {
				name.TextColor(p.Text).FontWeight(500)
			}
			close := native.ButtonBase(c.Key("close-tab:"+tab.ID)).Size(18, 18).Shrink(0).Radius(9).Center().Label("Close tab " + tab.Name)
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
	pill.ContextMenu(func(m *native.Menu) { w.tabMenu(m, tab) })
	if pill.DoubleClicked() {
		w.app.SelectTab(tab.ID)
		w.Dispatch("rename_tab")
	} else if pill.Clicked() {
		w.app.SelectTab(tab.ID)
		w.BlurInput()
	}
}

func (w *Window) tabOpenControl(c *native.Context, p design.Palette) {
	can := w.Can("new_tab")
	plus := native.ButtonBase(c.Key("new-tab")).Size(26, 26).Shrink(0).Radius(13).Center().Label("New tab")
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
	more := native.ButtonBase(c.Key("tab-actions")).Size(26, 26).Shrink(0).Radius(13).Center().Label("Tab actions")
	w.controlState(more, p)
	more.Children(func() { native.Icon(c, icons.Ellipsis).Size(15, 15).TextColor(p.Secondary) })
	if more.Clicked() {
		w.tabMenuOpen = !w.tabMenuOpen
	}
	native.PopoverBase(c, more, &w.tabMenuOpen, func(panel native.Element) {
		w.popoverPanel(c, panel, p)
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
	item := native.ButtonBase(c.Key(key)).FillWidth().Radius(w.shape.MenuItem).Padding(6, 8).Label(label)
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

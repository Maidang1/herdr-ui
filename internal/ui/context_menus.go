package ui

import (
	"github.com/Maidang1/herdr-ui/internal/app"
	native "github.com/egoist/mygo/ui"
)

// The context menus bring Herdr's keyboard-only commands to the pointer,
// on the thing they act on, as a Mac app's own menus do. Each item selects
// its row's workspace, tab or pane first, then dispatches the same action
// the prefix chord runs, so menus and keys can never disagree.

// menuAction adds an item for action, enabled while the action can run.
func (w *Window) menuAction(m *native.Menu, label, action string, target func()) {
	if m.Item(label).Disabled(!w.Can(action)).Chosen() {
		target()
		w.Dispatch(action)
	}
}

// workspaceMenu is a space row's menu: tabs, its name, its worktrees, its
// directory, and closing it.
func (w *Window) workspaceMenu(m *native.Menu, row app.WorkspaceRow) {
	selectRow := func() { w.app.SelectWorkspace(row.ID) }
	if m.Item("Open").Chosen() {
		selectRow()
		w.BlurInput()
	}
	w.menuAction(m, "New Tab", "new_tab", selectRow)
	w.menuAction(m, "Rename Workspace…", "rename_workspace", selectRow)
	if row.GitKnown {
		m.Separator()
		w.menuAction(m, "New Worktree…", "new_worktree", selectRow)
		w.menuAction(m, "Open Worktree…", "open_worktree", selectRow)
		w.menuAction(m, "Remove Worktree…", "remove_worktree", selectRow)
	}
	if row.Path != "" {
		m.Separator()
		if m.Item("Copy Path").Disabled(w.hooks.WriteClipboard == nil).Chosen() {
			w.shortcutError(w.hooks.WriteClipboard(row.Path))
		}
		if m.Item("Reveal in Finder").Disabled(w.hooks.RevealPath == nil).Chosen() {
			w.hooks.RevealPath(row.Path)
		}
	}
	m.Separator()
	w.menuAction(m, "Close Workspace…", "close_workspace", selectRow)
}

// tabMenu is a tab pill's menu: its name, its place in the strip, the
// splits of its focused pane, and closing it.
func (w *Window) tabMenu(m *native.Menu, tab app.TabRow) {
	selectTab := func() { w.app.SelectTab(tab.ID) }
	w.menuAction(m, "Rename Tab…", "rename_tab", selectTab)
	w.menuAction(m, "New Tab", "new_tab", selectTab)
	m.Separator()
	w.menuAction(m, "Move Tab Left", "move_tab_previous", selectTab)
	w.menuAction(m, "Move Tab Right", "move_tab_next", selectTab)
	m.Separator()
	w.menuAction(m, "Split Right", "split_vertical", selectTab)
	w.menuAction(m, "Split Down", "split_horizontal", selectTab)
	m.Separator()
	w.menuAction(m, "Close Tab", "close_tab", selectTab)
}

// paneMenu is a pane's menu, on its card header and its agent row: the
// layout commands, the history tools, and closing it.
func (w *Window) paneMenu(m *native.Menu, paneID string) {
	selectPane := func() { w.app.SelectPane(paneID) }
	w.menuAction(m, "Rename Pane…", "rename_pane", selectPane)
	m.Separator()
	w.menuAction(m, "Split Right", "split_vertical", selectPane)
	w.menuAction(m, "Split Down", "split_horizontal", selectPane)
	w.menuAction(m, w.ActionLabel("zoom"), "zoom", selectPane)
	m.Submenu("Swap Pane", func(m *native.Menu) {
		w.menuAction(m, "Left", "swap_pane_left", selectPane)
		w.menuAction(m, "Right", "swap_pane_right", selectPane)
		w.menuAction(m, "Up", "swap_pane_up", selectPane)
		w.menuAction(m, "Down", "swap_pane_down", selectPane)
	})
	m.Separator()
	w.menuAction(m, "Find…", "find", selectPane)
	w.menuAction(m, "Copy Mode", "copy_mode", selectPane)
	w.menuAction(m, "Edit Scrollback", "edit_scrollback", selectPane)
	w.menuAction(m, "Clear", "clear_pane", selectPane)
	m.Separator()
	w.menuAction(m, "Close Pane", "close_pane", selectPane)
}

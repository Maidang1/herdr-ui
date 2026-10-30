package ui

import "strings"

// MenuSection is one menu of the menu bar: its commands in groups, which
// the menu separates with lines.
type MenuSection struct {
	Title  string
	Groups [][]string
}

// MenuBar lays the window's commands out as a Mac app does: one menu per
// thing they act on, related commands together. Settings lives in the app
// menu and the keyboard reference in Help. Commands that only make sense
// with a number typed after the prefix (switch_tab, switch_workspace,
// focus_agent) stay keyboard-only.
var MenuBar = []MenuSection{
	{"Workspace", [][]string{
		{"new_workspace", "rename_workspace", "close_workspace"},
		{"new_worktree", "open_worktree", "remove_worktree"},
		{"reload_config", "detach"},
	}},
	{"Tab", [][]string{
		{"new_tab", "rename_tab", "close_tab"},
		{"move_tab_previous", "move_tab_next"},
	}},
	{"Pane", [][]string{
		{"split_vertical", "split_horizontal", "zoom", "rename_pane", "close_pane"},
		{"swap_pane_left", "swap_pane_right", "swap_pane_up", "swap_pane_down"},
		{"resize_mode", "resize_pane_left", "resize_pane_right", "resize_pane_up", "resize_pane_down"},
		{"find", "find_next", "find_previous"},
		{"copy_mode", "edit_scrollback", "clear_pane", "remote_image_paste"},
		{"pane_details", "watch_output"},
	}},
	{"Go", [][]string{
		{"goto", "workspace_picker"},
		{"attention", "open_notification_target", "previous_agent", "next_agent"},
		{"previous_workspace", "next_workspace", "previous_tab", "next_tab"},
		{"focus_pane_left", "focus_pane_right", "focus_pane_up", "focus_pane_down", "cycle_pane_next", "cycle_pane_previous", "last_pane"},
	}},
	{"View", [][]string{
		{"toggle_sidebar", "zen"},
		{"zoom_in", "zoom_out", "zoom_reset"},
	}},
}

// menuPrompts are the commands that ask for more before acting, which a
// Mac menu marks with an ellipsis.
var menuPrompts = map[string]bool{"rename_workspace": true, "rename_tab": true, "rename_pane": true, "new_worktree": true, "open_worktree": true, "remove_worktree": true, "settings": true}

// MenuTitle is a command's menu-bar title: Title Case, with an ellipsis
// when the command asks for more.
func (w *Window) MenuTitle(id string) string {
	title := titleCase(w.ActionLabel(id))
	if menuPrompts[id] {
		title += "…"
	}
	return title
}

// MenuLabel is the title with the command's Herdr chord. A chord after the
// prefix cannot be a native key equivalent, so it follows the title in
// the Mac's key symbols.
func (w *Window) MenuLabel(id string) string {
	label := w.MenuTitle(id)
	if binding := keyGlyphs(w.BindingLabel(id)); binding != "" {
		label += "    " + binding
	}
	return label
}

// titleCase capitalizes a sentence-case label as the Human Interface
// Guidelines title-case menu items, leaving short connecting words lower.
func titleCase(label string) string {
	minor := map[string]bool{"a": true, "an": true, "and": true, "as": true, "at": true, "by": true, "for": true, "in": true, "of": true, "on": true, "or": true, "the": true, "to": true}
	words := strings.Fields(label)
	for i, word := range words {
		if i > 0 && minor[word] {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

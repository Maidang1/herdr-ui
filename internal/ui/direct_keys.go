package ui

import (
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
)

// DirectKeys are the window commands' Mac key equivalents, as Terminal,
// iTerm2, Ghostty and Kero give them; Herdr's prefix chords stay beneath.
// Selecting a tab by number (⌘1–⌘9) is SelectTabNumber.
var DirectKeys = map[string]string{
	"new_tab": "CmdOrCtrl+T", "close_pane": "CmdOrCtrl+W",
	"split_vertical": "CmdOrCtrl+D", "split_horizontal": "CmdOrCtrl+Shift+D",
	"previous_tab": "CmdOrCtrl+Shift+[", "next_tab": "CmdOrCtrl+Shift+]",
	"cycle_pane_previous": "CmdOrCtrl+[", "cycle_pane_next": "CmdOrCtrl+]",
	"focus_pane_left": "CmdOrCtrl+Alt+Left", "focus_pane_right": "CmdOrCtrl+Alt+Right",
	"focus_pane_up": "CmdOrCtrl+Alt+Up", "focus_pane_down": "CmdOrCtrl+Alt+Down",
	"zoom_in": "CmdOrCtrl+=", "zoom_out": "CmdOrCtrl+-", "zoom_reset": "CmdOrCtrl+0",
	"toggle_sidebar": "CmdOrCtrl+B", "attention": "CmdOrCtrl+J",
	"find": "CmdOrCtrl+F", "find_next": "CmdOrCtrl+G", "find_previous": "CmdOrCtrl+Shift+G",
	"clear_pane": "CmdOrCtrl+K", "zen": "Ctrl+Cmd+Z", "settings": "CmdOrCtrl+,", "goto": "CmdOrCtrl+P",
}

// SelectTabNumber selects the nth tab of the workspace, from 1; 9 is the
// last, as ⌘9 is in Safari and Ghostty.
func (w *Window) SelectTabNumber(n int) {
	tabs := w.currentView().Tabs
	if n == 9 {
		n = len(tabs)
	}
	if n >= 1 && n <= len(tabs) {
		w.DispatchIndex("switch_tab", n-1)
	}
}

// ClosePaneKey closes the focused pane for ⌘W, asking first while an agent
// runs in it: a stray key must not end an agent's session.
func (w *Window) ClosePaneKey() {
	if w.controller == nil || !w.Can("close_pane") {
		return
	}
	if agent := focusedAgent(w.currentView()); agent != "" {
		prompt := closeAgentPrompt(agent)
		prompt.focus, prompt.submit = true, func(string) { w.Dispatch("close_pane") }
		w.controller.shortcut.prompt = &prompt
		return
	}
	w.Dispatch("close_pane")
}

// focusedAgent is the agent running in the view's focused pane, or "".
func focusedAgent(v app.View) string {
	for _, pane := range v.Panes {
		if pane.ID == v.PaneID {
			return pane.Agent
		}
	}
	return ""
}

// directKeyLabel is a command's direct key in the Mac's symbols, modifiers
// in the order menus show them (⌃⌥⇧⌘), or "" without one.
func directKeyLabel(id string) string {
	accelerator := DirectKeys[id]
	if accelerator == "" {
		return ""
	}
	parts := strings.Split(accelerator, "+")
	key := parts[len(parts)-1]
	var mods strings.Builder
	for _, mod := range []struct{ name, glyph string }{{"Ctrl", "⌃"}, {"Alt", "⌥"}, {"Shift", "⇧"}, {"Cmd", "⌘"}, {"CmdOrCtrl", "⌘"}} {
		for _, part := range parts[:len(parts)-1] {
			if part == mod.name {
				mods.WriteString(mod.glyph)
			}
		}
	}
	return mods.String() + keyGlyphs(key)
}

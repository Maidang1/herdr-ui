package ui

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
)

// DirectKeys are native application shortcuts. Terminal control keys remain
// available to the shell on Windows and Linux; Herdr prefix chords stay beneath.
var DirectKeys = directKeysForPlatform(runtime.GOOS)

func directKeysForPlatform(platform string) map[string]string {
	if platform != "darwin" {
		keys := map[string]string{
			"new_tab": "Ctrl+Shift+T", "close_pane": "Ctrl+Shift+W",
			"split_vertical": "Ctrl+Shift+D", "split_horizontal": "Ctrl+Alt+D",
			"previous_tab": "Ctrl+PageUp", "next_tab": "Ctrl+PageDown",
			"cycle_pane_previous": "Ctrl+Shift+[", "cycle_pane_next": "Ctrl+Shift+]",
			"focus_pane_left": "Ctrl+Alt+Left", "focus_pane_right": "Ctrl+Alt+Right",
			"focus_pane_up": "Ctrl+Alt+Up", "focus_pane_down": "Ctrl+Alt+Down",
			"zoom_in": "Ctrl+Shift+=", "zoom_out": "Ctrl+Shift+-", "zoom_reset": "Ctrl+Shift+0",
			"toggle_sidebar": "Ctrl+Shift+B", "attention": "Ctrl+Shift+J",
			"find": "Ctrl+Shift+F", "find_next": "Ctrl+Shift+G", "find_previous": "Ctrl+Alt+G",
			"clear_pane": "Ctrl+Shift+K", "zen": "Ctrl+Shift+Z", "settings": "Ctrl+Shift+,", "goto": "Ctrl+Shift+P",
		}
		if platform == "windows" {
			// Windows reports AltGr as Ctrl+Alt. Keep every global accelerator
			// away from that modifier pair so layout text reaches the widget.
			keys["split_horizontal"] = "Ctrl+Shift+E"
			keys["cycle_pane_previous"] = "Ctrl+Shift+PageUp"
			keys["cycle_pane_next"] = "Ctrl+Shift+PageDown"
			keys["focus_pane_left"] = "Ctrl+Shift+Left"
			keys["focus_pane_right"] = "Ctrl+Shift+Right"
			keys["focus_pane_up"] = "Ctrl+Shift+Up"
			keys["focus_pane_down"] = "Ctrl+Shift+Down"
			keys["find_previous"] = "Ctrl+Shift+H"
		}
		return keys
	}
	return map[string]string{
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
}

// TabNumberAccelerator returns the native shortcut for tabs 1 through 9.
func TabNumberAccelerator(n int) string { return tabNumberAccelerator(runtime.GOOS, n) }
func tabNumberAccelerator(platform string, n int) string {
	if n < 1 || n > 9 {
		return ""
	}
	modifier := "Ctrl+Alt+"
	if platform == "darwin" {
		modifier = "Cmd+"
	} else if platform == "windows" {
		modifier = "Ctrl+Shift+"
	}
	return modifier + strconv.Itoa(n)
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

// directKeyLabel uses the platform's native shortcut notation.
func directKeyLabel(id string) string {
	return acceleratorLabel(DirectKeys[id], runtime.GOOS)
}
func acceleratorLabel(accelerator, platform string) string {
	if accelerator == "" {
		return ""
	}
	if platform != "darwin" {
		return accelerator
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

func shortcutProfileDescription() string {
	if runtime.GOOS == "darwin" {
		return "⌘ keys are fixed, as on any Mac terminal. Prefix chords come from Herdr’s own configuration."
	}
	if runtime.GOOS == "windows" {
		return "App commands use Ctrl+Shift, tabs use Ctrl+PageUp/PageDown, and pane focus uses Ctrl+Shift+arrows. Ctrl+Alt remains available for AltGr text. Terminal control keys and Herdr’s configured prefix chords remain available."
	}
	return "App commands use Ctrl+Shift, tabs use Ctrl+PageUp/PageDown, and pane focus uses Ctrl+Alt+arrows. Terminal control keys and Herdr’s configured prefix chords remain available."
}

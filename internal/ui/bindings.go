package ui

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The shortcuts of the window.
//
// The shortcuts are written the way the Rust client writes them: the
// modifiers and the key, joined by dashes, as in `cmd-shift-n`. The user
// sets them in the settings, and they go into the overrides file under
// `[keybindings]`, the table the Rust client uses.

// Binding is one shortcut of the window.
type Binding struct {
	// Cmd, Ctrl, Alt and Shift are the modifiers the key is pressed
	// with.
	Cmd   bool
	Ctrl  bool
	Alt   bool
	Shift bool
	// Key is the key itself.
	Key ui.Key
}

// Modifiers returns the modifiers of the shortcut, the way the window
// hands them to the toolkit.
func (b Binding) Modifiers() ui.Modifiers {
	var mods ui.Modifiers
	if b.Cmd {
		mods |= ui.Cmd
	}
	if b.Ctrl {
		mods |= ui.Ctrl
	}
	if b.Alt {
		mods |= ui.Alt
	}
	if b.Shift {
		mods |= ui.Shift
	}
	return mods
}

// String writes the shortcut back, the way it is written in the settings.
func (b Binding) String() string {
	var parts []string
	if b.Cmd {
		parts = append(parts, "cmd")
	}
	if b.Ctrl {
		parts = append(parts, "ctrl")
	}
	if b.Alt {
		parts = append(parts, "alt")
	}
	if b.Shift {
		parts = append(parts, "shift")
	}
	parts = append(parts, keyName(b.Key))
	return strings.Join(parts, "-")
}

// ParseBinding reads a shortcut.
//
// The modifiers may come in any order, and the key is the last part. A
// shortcut may name no modifier at all, as in `f5`.
func ParseBinding(text string) (Binding, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Binding{}, fmt.Errorf("herdr-ui: the shortcut is empty")
	}
	parts := strings.Split(text, "-")
	if len(parts) == 0 {
		return Binding{}, fmt.Errorf("herdr-ui: %q is not a shortcut", text)
	}
	var binding Binding
	for index, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			return Binding{}, fmt.Errorf("herdr-ui: %q is not a shortcut", text)
		}
		if index == len(parts)-1 {
			key, ok := keyOf(part)
			if !ok {
				return Binding{}, fmt.Errorf("herdr-ui: %q is not the name of a key", part)
			}
			binding.Key = key
			continue
		}
		switch part {
		case "cmd", "command", "meta", "super":
			binding.Cmd = true
		case "ctrl", "control":
			binding.Ctrl = true
		case "alt", "option":
			binding.Alt = true
		case "shift":
			binding.Shift = true
		default:
			return Binding{}, fmt.Errorf("herdr-ui: %q is not a modifier", part)
		}
	}
	return binding, nil
}

// keyNames are the names of the keys a shortcut may name, the way the
// Rust client writes them.
//
// The list keeps its order, so that the name a key is written back with
// is always the same one.
var keyNames = []struct {
	name string
	key  ui.Key
}{
	{"enter", ui.KeyEnter},
	{"escape", ui.KeyEscape},
	{"esc", ui.KeyEscape},
	{"backspace", ui.KeyBackspace},
	{"tab", ui.KeyTab},
	{"space", ui.KeySpace},
	{"delete", ui.KeyDelete},
	{"insert", ui.KeyInsert},
	{"home", ui.KeyHome},
	{"end", ui.KeyEnd},
	{"pageup", ui.KeyPageUp},
	{"pagedown", ui.KeyPageDown},
	{"left", ui.KeyLeft},
	{"right", ui.KeyRight},
	{"up", ui.KeyUp},
	{"down", ui.KeyDown},
	{"minus", ui.KeyMinus},
	{"equal", ui.KeyEqual},
	{"comma", ui.KeyComma},
	{"period", ui.KeyPeriod},
	{"slash", ui.KeySlash},
	{"semicolon", ui.KeySemicolon},
	{"quote", ui.KeyQuote},
	{"bracketleft", ui.KeyBracketLeft},
	{"bracketright", ui.KeyBracketRight},
	{"backslash", ui.KeyBackslash},
	{"backquote", ui.KeyBackquote},
}

// keyOf reads the name of a key.
func keyOf(name string) (ui.Key, bool) {
	for _, entry := range keyNames {
		if entry.name == name {
			return entry.key, true
		}
	}
	if len(name) == 1 {
		letter := name[0]
		if letter >= 'a' && letter <= 'z' {
			return ui.Key(letter-'a') + ui.KeyA, true
		}
		if letter >= '0' && letter <= '9' {
			return ui.Key(letter-'0') + ui.Key0, true
		}
	}
	if len(name) >= 2 && name[0] == 'f' {
		number := 0
		for _, digit := range name[1:] {
			if digit < '0' || digit > '9' {
				return ui.KeyUnknown, false
			}
			number = number*10 + int(digit-'0')
		}
		if number >= 1 && number <= 12 {
			return ui.KeyF1 + ui.Key(number-1), true
		}
	}
	return ui.KeyUnknown, false
}

// keyName writes the name of a key, the way a shortcut names it.
func keyName(key ui.Key) string {
	for _, entry := range keyNames {
		if entry.key == key {
			return entry.name
		}
	}
	if key >= ui.KeyA && key <= ui.KeyZ {
		return string(rune('a' + int(key-ui.KeyA)))
	}
	if key >= ui.Key0 && key <= ui.Key9 {
		return string(rune('0' + int(key-ui.Key0)))
	}
	if key >= ui.KeyF1 && key <= ui.KeyF12 {
		return fmt.Sprintf("f%d", int(key-ui.KeyF1)+1)
	}
	return "unknown"
}

// Action is one thing the window does that the user can give a shortcut
// to.
type Action struct {
	// Name is the name of the action, the one the settings file uses.
	Name string
	// Title says what the action does.
	Title string
	// Default is the shortcut the action has when the user sets none.
	Default string
}

// The actions the window gives a shortcut to.
var actions = []Action{
	{Name: "command_palette", Title: "Command palette", Default: "cmd-k"},
	{Name: "find", Title: "Find in the terminal", Default: "cmd-f"},
	{Name: "copy_mode", Title: "Copy mode", Default: "cmd-shift-c"},
	{Name: "annotate", Title: "Note to the agent", Default: "cmd-shift-a"},
	{Name: "settings", Title: "Settings", Default: "cmd-comma"},
	{Name: "new_tab", Title: "New tab", Default: "cmd-t"},
	{Name: "close_tab", Title: "Close tab", Default: "cmd-w"},
	{Name: "next_tab", Title: "Next tab", Default: "cmd-shift-right"},
	{Name: "previous_tab", Title: "Previous tab", Default: "cmd-shift-left"},
	{Name: "update_panel", Title: "What is new", Default: "cmd-shift-u"},
	{Name: "quit", Title: "Quit", Default: "cmd-q"},
}

// ActionByName returns the action of a name.
func ActionByName(name string) (Action, bool) {
	for _, action := range actions {
		if action.Name == name {
			return action, true
		}
	}
	return Action{}, false
}

// bindingOf returns the shortcut of an action.
//
// The shortcut the user set stands first, and the default of the action
// stands when the user set none. A shortcut that cannot be read is passed
// over, and the default stands for it.
func bindingOf(name, setting string) Binding {
	if strings.TrimSpace(setting) != "" {
		for _, part := range strings.Split(setting, ",") {
			binding, err := ParseBinding(part)
			if err == nil {
				return binding
			}
		}
	}
	action, ok := ActionByName(name)
	if !ok {
		return Binding{}
	}
	binding, err := ParseBinding(action.Default)
	if err != nil {
		return Binding{}
	}
	return binding
}

// shortcut is one shortcut the window listens for.
type shortcut struct {
	mods ui.Modifiers
	key  ui.Key
	run  func()
}

// shortcuts returns the shortcuts of the window.
//
// The copy mode holds the keyboard while it is on, so the shortcuts of the
// window do not stand then.
func (a *App) shortcuts() []shortcut {
	a.mu.Lock()
	settings := a.keybindings
	a.mu.Unlock()

	find := func(name string) (Binding, bool) {
		text, ok := settings[name]
		if !ok {
			action, found := ActionByName(name)
			if !found {
				return Binding{}, false
			}
			text = action.Default
		}
		binding, err := ParseBinding(text)
		if err != nil {
			return Binding{}, false
		}
		return binding, true
	}

	var list []shortcut
	add := func(name string, run func()) {
		binding, ok := find(name)
		if !ok {
			return
		}
		list = append(list, shortcut{mods: binding.Modifiers(), key: binding.Key, run: run})
	}

	add("command_palette", a.togglePalette)
	add("find", a.toggleFinder)
	add("copy_mode", a.toggleCopyMode)
	add("annotate", a.startAnnotation)
	add("settings", a.openSettings)
	add("new_tab", a.newTab)
	add("close_tab", a.closeTab)
	add("next_tab", a.nextTab)
	add("previous_tab", a.previousTab)
	add("update_panel", a.openUpdatePanel)
	add("quit", a.quit)
	return list
}

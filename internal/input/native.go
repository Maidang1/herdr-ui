package input

import (
	"github.com/egoist/mygo/ui"
	"strconv"
	"strings"
)

// NativeKey translates layout-aware native key events to Herdr binding labels.
// Text and composition events remain owned by the terminal plugin.
func NativeKey(mods ui.Modifiers, key ui.Key) (string, bool) {
	if mods & ^(ui.Ctrl|ui.Alt|ui.Shift|ui.Super) != 0 {
		return "", false
	}
	var name string
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		name = string(rune('a' + key - ui.KeyA))
	case key >= ui.Key0 && key <= ui.Key9:
		name = string(rune('0' + key - ui.Key0))
	case key >= ui.KeyF1 && key <= ui.KeyF12:
		name = "f" + strconv.Itoa(int(key-ui.KeyF1)+1)
	default:
		name = map[ui.Key]string{ui.KeyHome: "home", ui.KeyEnd: "end", ui.KeyDelete: "delete", ui.KeyInsert: "insert", ui.KeyPageUp: "pageup", ui.KeyPageDown: "pagedown", ui.KeyEnter: "enter", ui.KeyEscape: "esc", ui.KeyBackspace: "backspace", ui.KeyTab: "tab", ui.KeySpace: "space", ui.KeyLeft: "left", ui.KeyRight: "right", ui.KeyUp: "up", ui.KeyDown: "down", ui.KeyMinus: "minus", ui.KeyEqual: "=", ui.KeyComma: "comma", ui.KeyPeriod: "period", ui.KeySlash: "slash", ui.KeySemicolon: "semicolon", ui.KeyQuote: "quote", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "backslash", ui.KeyBackquote: "backtick"}[key]
	}
	if name == "" {
		return "", false
	}
	parts := []string{}
	for _, m := range []struct {
		bit  ui.Modifiers
		name string
	}{{ui.Ctrl, "ctrl"}, {ui.Alt, "alt"}, {ui.Shift, "shift"}, {ui.Super, "cmd"}} {
		if mods&m.bit != 0 {
			parts = append(parts, m.name)
		}
	}
	n, err := Normalize(strings.Join(append(parts, name), "+"))
	return n, err == nil
}

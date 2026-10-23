package input

import (
	"fmt"
	"strings"
)

type Result struct {
	Consumed bool
	Action   string
	Index    int
	Send     []byte
}

// Machine is UI-thread owned. Reset on focus loss and target change.
// It reports action names; the caller owns dispatch and terminal transport.
type Machine struct {
	Bindings Bindings
	Awaiting bool
	Resize   bool
}

func (m *Machine) Reset() { m.Awaiting = false; m.Resize = false }

func (m *Machine) Key(key string) Result { return m.Handle(key, false) }

// Handle cancels local modes while an input method owns composition.
func (m *Machine) Handle(key string, composing bool) Result {
	if composing {
		m.Reset()
		return Result{}
	}
	if m.Resize {
		code := key[strings.LastIndex(key, "+")+1:]
		if code == "esc" || code == "enter" || m.Bindings.Actions["prefix+"+key] == "resize_mode" || m.Bindings.Actions[key] == "resize_mode" {
			m.Reset()
			return Result{Consumed: true}
		}
		action := map[string]string{"h": "resize_left", "left": "resize_left", "j": "resize_down", "down": "resize_down", "k": "resize_up", "up": "resize_up", "l": "resize_right", "right": "resize_right"}[code]
		return Result{Consumed: true, Action: action}
	}
	if m.Awaiting {
		m.Awaiting = false
		if key == m.Bindings.Prefix {
			return Result{Consumed: true, Send: LiteralKey(key)}
		}
		if _, code := splitKey(key); code == "esc" {
			return Result{Consumed: true}
		}
		action := m.Bindings.Actions["prefix+"+key]
		m.Resize = action == "resize_mode"
		return Result{Consumed: true, Action: action, Index: m.Bindings.Indices["prefix+"+key]}
	}
	if key == m.Bindings.Prefix {
		m.Awaiting = true
		return Result{Consumed: true}
	}
	if a := m.Bindings.Actions[key]; a != "" {
		m.Resize = a == "resize_mode"
		return Result{Consumed: true, Action: a, Index: m.Bindings.Indices[key]}
	}
	if key == "ctrl+b" {
		return Result{Consumed: true, Send: []byte{2, 2}}
	}

	return Result{}
}

// LiteralKey encodes a configured prefix for the attach stream. Ctrl+B is
// doubled because the attach transport reserves that byte as its escape.
func LiteralKey(key string) []byte {
	normalized, err := Normalize(key)
	if err != nil || strings.HasPrefix(normalized, "prefix+") {
		return nil
	}
	mods, code := splitKey(normalized)
	modifier := 1
	if mods["shift"] {
		modifier += 1
	}
	if mods["alt"] {
		modifier += 2
	}
	if mods["ctrl"] {
		modifier += 4
	}
	if mods["cmd"] {
		modifier += 8
	}
	var out string
	if final := map[string]string{"up": "A", "down": "B", "right": "C", "left": "D", "home": "H", "end": "F"}[code]; final != "" {
		if modifier == 1 {
			out = "\x1b[" + final
		} else {
			out = fmt.Sprintf("\x1b[1;%d%s", modifier, final)
		}
	} else if number := map[string]int{"insert": 2, "delete": 3, "pageup": 5, "pagedown": 6, "f5": 15, "f6": 17, "f7": 18, "f8": 19, "f9": 20, "f10": 21, "f11": 23, "f12": 24}[code]; number != 0 {
		if modifier == 1 {
			out = fmt.Sprintf("\x1b[%d~", number)
		} else {
			out = fmt.Sprintf("\x1b[%d;%d~", number, modifier)
		}
	} else if len(code) == 2 && code[0] == 'f' && code[1] >= '1' && code[1] <= '4' {
		final := string(rune('P' + code[1] - '1'))
		if modifier == 1 {
			out = "\x1bO" + final
		} else {
			out = fmt.Sprintf("\x1b[1;%d%s", modifier, final)
		}
	} else {
		char := map[string]rune{"enter": 13, "esc": 27, "tab": 9, "backspace": 127, "space": 32}[code]
		if char == 0 {
			chars := []rune(code)
			if len(chars) != 1 {
				return nil
			}
			char = chars[0]
		}
		// CSI-u retains modifiers that have no distinct legacy byte sequence.
		enhanced := mods["cmd"] || (mods["ctrl"] && mods["shift"]) || ((code == "enter" || code == "tab" || code == "esc") && mods["ctrl"]) || ((code == "enter" || code == "backspace" || code == "esc") && mods["shift"])
		if enhanced {
			out = fmt.Sprintf("\x1b[%d;%du", char, modifier)
		} else {
			out = string(char)
			encodedModifiers := false
			if mods["shift"] {
				out = strings.ToUpper(out)
			}
			if code == "tab" && mods["shift"] {
				out = "\x1b[Z"
			} else if mods["ctrl"] {
				if code == "backspace" {
					out = "\x08"
				} else {
					c := char
					if c >= 'a' && c <= 'z' {
						c -= 32
					}
					switch {
					case c >= '@' && c <= '_':
						out = string(rune(c & 31))
					case c == ' ' || c == '2':
						out = "\x00"
					case c >= '3' && c <= '7':
						out = string(rune(c - '3' + 27))
					case c == '8' || c == '?':
						out = "\x7f"
					default:
						out = fmt.Sprintf("\x1b[%d;%du", char, modifier)
						encodedModifiers = true
					}
				}
			}
			if mods["alt"] && !encodedModifiers {
				out = "\x1b" + out
			}
		}
	}
	return []byte(strings.ReplaceAll(out, "\x02", "\x02\x02"))
}

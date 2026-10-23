package input

import "strings"

type Result struct {
	Consumed bool
	Action   string
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
			if key == "ctrl+b" {
				return Result{Consumed: true, Send: []byte{2, 2}}
			}
			if strings.HasPrefix(key, "ctrl+") && len(key) == 6 {
				return Result{Consumed: true, Send: []byte{key[5] - 'a' + 1}}
			}
			return Result{Consumed: true}
		}
		if key == "esc" {
			return Result{Consumed: true}
		}
		action := m.Bindings.Actions["prefix+"+key]
		m.Resize = action == "resize_mode"
		return Result{Consumed: true, Action: action}
	}
	if key == m.Bindings.Prefix {
		m.Awaiting = true
		return Result{Consumed: true}
	}
	if key == "ctrl+b" {
		return Result{Consumed: true, Send: []byte{2, 2}}
	}
	if a := m.Bindings.Actions[key]; a != "" {
		m.Resize = a == "resize_mode"
		return Result{Consumed: true, Action: a}
	}
	return Result{}
}

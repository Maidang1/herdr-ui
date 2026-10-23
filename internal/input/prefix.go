package input

import "strings"

type Result struct {
	Consumed bool
	Action   string
	Send     []byte
}

// Machine is UI-thread owned. Reset on focus loss and target change. The M0
// harness reports actions; it never executes split/close/etc against a session.
type Machine struct {
	Bindings Bindings
	Awaiting bool
}

func (m *Machine) Reset() { m.Awaiting = false }

func (m *Machine) Key(key string) Result {
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
		return Result{Consumed: true, Action: m.Bindings.Actions["prefix+"+key]}
	}
	if key == m.Bindings.Prefix {
		m.Awaiting = true
		return Result{Consumed: true}
	}
	if key == "ctrl+b" {
		return Result{Consumed: true, Send: []byte{2, 2}}
	}
	if a := m.Bindings.Actions[key]; a != "" {
		return Result{Consumed: true, Action: a}
	}
	return Result{}
}

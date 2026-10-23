// Package terminalhost owns local attach clients independently of JSON sessions.
package terminalhost

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

type Spec struct {
	TargetID                uint64
	TerminalID, PaneID, CWD string
}
type Binding struct {
	TargetID   uint64
	TerminalID string
	Generation uint64
}
type State uint8

const (
	Starting State = iota
	Attached
	Failed
	Closed
)

type Status struct {
	State    State
	Message  string
	ExitCode int
}

// InputState reports whether native composition can be observed.
type InputState struct{ Composing, Guarded bool }
type KeyResult struct {
	Consumed bool
	Send     []byte
}

type Config struct {
	Binary   string
	Target   herdr.Target
	Env      []string
	Post     func(func())
	Changed  func()
	OnScroll func(Binding, int)
	OnKey    func(Binding, ui.Modifiers, ui.Key, InputState) KeyResult
}
type instance interface {
	View(*ui.Context) ui.Element
	Send([]byte)
	Close() error
}
type factory func(terminal.Options) (instance, error)
type key struct {
	target   uint64
	terminal string
}
type entry struct {
	spec      Spec
	binding   Binding
	status    Status
	owned     instance
	installed bool
	retry     bool
	closing   bool
	openDone  bool
	closeDone bool
	finished  bool
	previous  <-chan struct{}
	lifetime  chan struct{}
}

// Host's methods Reconcile, Retry and View run on the UI thread. Close runs
// after UI detachment. The mutex also fences background completion/callbacks.
type Host struct {
	mu         sync.Mutex
	cfg        Config
	create     factory
	entries    map[key]*entry
	tails      map[key]*entry
	generation uint64
	closed     bool
	tasks      int
	done       chan struct{}
	closeErr   error
}

func New(c Config) *Host { return newHost(c, openTerminal) }
func newHost(c Config, f factory) *Host {
	c.Env = append([]string(nil), c.Env...)
	return &Host{cfg: c, create: f, entries: make(map[key]*entry), tails: make(map[key]*entry), done: make(chan struct{})}
}
func identity(s Spec) key           { return key{s.TargetID, s.TerminalID} }
func (h *Host) valid(e *entry) bool { return !h.closed && h.entries[identity(e.spec)] == e }

// Reconcile validates the complete demand before changing any ownership. Pane
// remapping recreates the attach so its UI callbacks cannot address an old pane.
func (h *Host) Reconcile(desired []Spec) ([]Binding, error) {
	seen := make(map[key]bool, len(desired))
	panes := make(map[key]bool, len(desired))
	for _, s := range desired {
		k := identity(s)
		pane := key{s.TargetID, s.PaneID}
		if s.TargetID == 0 || s.TerminalID == "" || s.PaneID == "" || seen[k] || panes[pane] {
			return nil, errors.New("terminal demand has missing or duplicate identity")
		}
		seen[k] = true
		panes[pane] = true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, errors.New("terminal host closed")
	}
	if h.cfg.Post == nil {
		return nil, errors.New("terminal host requires asynchronous UI Post")
	}
	for k, e := range h.entries {
		if !seen[k] {
			delete(h.entries, k)
			h.dispose(e)
		}
	}
	out := make([]Binding, 0, len(desired))
	for _, s := range desired {
		k := identity(s)
		e := h.entries[k]
		if e != nil && (e.retry || e.spec.PaneID != s.PaneID) {
			delete(h.entries, k)
			h.dispose(e)
			e = nil
		}
		if e == nil {
			h.generation++
			e = &entry{spec: s, binding: Binding{s.TargetID, s.TerminalID, h.generation}, status: Status{State: Starting}, lifetime: make(chan struct{})}
			if previous := h.tails[k]; previous != nil {
				e.previous = previous.lifetime
			}
			h.tails[k] = e
			h.entries[k] = e
			h.tasks++
			go h.open(e)
		}
		out = append(out, e.binding)
	}
	return out, nil
}

// dispose is called with mu held, after the binding has been detached. Each
// instance is closed exactly once and all cleanup contributes to one budget.
func (h *Host) dispose(e *entry) {
	h.finishLifetime(e)
	if e.owned == nil || e.closing {
		return
	}
	e.closing = true
	t := e.owned
	e.installed = false
	h.tasks++
	go func() {
		err := t.Close()
		h.mu.Lock()
		h.closeErr = errors.Join(h.closeErr, err)
		e.closeDone = true
		h.finishLifetime(e)
		h.taskDone()
		h.mu.Unlock()
	}()
}

// A lifetime ends only after preparation and cleanup both finish. Retired
// generations waiting on a predecessor must still wait before releasing their
// own barrier, preserving the chain across rapid remove/re-add sequences.
func (h *Host) finishLifetime(e *entry) {
	if e.finished || !e.openDone || (e.owned != nil && !e.closeDone) {
		return
	}
	e.finished = true
	close(e.lifetime)
	k := identity(e.spec)
	if h.tails[k] == e {
		delete(h.tails, k)
	}
}
func (h *Host) finishOpen(e *entry) {
	e.openDone = true
	h.finishLifetime(e)
	h.taskDone()
}
func (h *Host) taskDone() {
	h.tasks--
	if h.closed && h.tasks == 0 {
		close(h.done)
	}
}
func (h *Host) open(e *entry) {
	if e.previous != nil {
		<-e.previous
	}
	h.mu.Lock()
	if !h.valid(e) {
		h.finishOpen(e)
		h.mu.Unlock()
		return
	}
	h.mu.Unlock()
	o, err := h.options(e)
	var t instance
	if err == nil {
		t, err = h.create(o)
	}
	if t == nil && err == nil {
		err = errors.New("terminal factory returned no terminal")
	}
	h.mu.Lock()
	e.owned = t
	if !h.valid(e) {
		h.dispose(e)
		h.finishOpen(e)
		h.mu.Unlock()
		return
	}
	// Ownership is registered before Post: shutdown does not depend on the
	// window ever executing the queued installation.
	if err != nil {
		h.dispose(e)
	}
	h.mu.Unlock()
	h.cfg.Post(func() {
		h.mu.Lock()
		valid := h.valid(e)
		if valid {
			if err != nil {
				e.status = Status{State: Failed, Message: "Terminal unavailable: " + err.Error()}
			} else {
				e.installed = true
				if e.status.State == Starting {
					e.status = Status{State: Attached}
				}
			}
		}
		h.mu.Unlock()
		if valid {
			h.changed()
		}
	})
	h.mu.Lock()
	h.finishOpen(e)
	h.mu.Unlock()
}
func (h *Host) changed() {
	if h.cfg.Changed != nil {
		h.cfg.Changed()
	}
}
func (h *Host) post(e *entry, update func()) {
	h.mu.Lock()
	valid := h.valid(e)
	h.mu.Unlock()
	if !valid {
		return
	}
	h.cfg.Post(func() {
		h.mu.Lock()
		valid := h.valid(e)
		if valid {
			update()
		}
		h.mu.Unlock()
		if valid {
			h.changed()
		}
	})
}
func (h *Host) lookup(b Binding) *entry {
	e := h.entries[key{b.TargetID, b.TerminalID}]
	if e == nil || e.binding != b || h.closed {
		return nil
	}
	return e
}
func (h *Host) Status(b Binding) Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e := h.lookup(b); e != nil {
		return e.status
	}
	return Status{State: Closed}
}
func (h *Host) Retry(b Binding) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e := h.lookup(b); e != nil && e.status.State == Failed {
		e.retry = true
	}
}
func (h *Host) View(c *ui.Context, b Binding) ui.Element {
	h.mu.Lock()
	e := h.lookup(b)
	var t instance
	if e != nil && e.installed {
		t = e.owned
	}
	h.mu.Unlock()
	if t == nil {
		return ui.Element{}
	}
	return t.View(c)
}

// Close marks all callbacks stale before closing any plugin. It waits only for
// the caller's context, and subsequent calls can continue waiting for cleanup.
func (h *Host) Close(ctx context.Context) error {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		for k, e := range h.entries {
			delete(h.entries, k)
			h.dispose(e)
		}
		if h.tasks == 0 {
			close(h.done)
		}
	}
	h.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-h.done:
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.closeErr
	}
}
func (h *Host) options(e *entry) (terminal.Options, error) {
	cmd, env, err := h.cfg.Target.Attach(h.cfg.Binary, e.spec.TerminalID, h.cfg.Env)
	if err != nil {
		return terminal.Options{}, err
	}
	// The plugin merges Env into os.Environ rather than replacing it. Empty
	// these inherited caller identities explicitly so filtered values cannot
	// reappear when the plugin builds the child process environment.
	env = append(env, "HERDR_SESSION=", "HERDR_PANE_ID=", "HERDR_TAB_ID=", "HERDR_WORKSPACE_ID=")
	light, dark := terminalThemes()
	return terminal.Options{Command: cmd, Env: env, Dir: e.spec.CWD, Font: terminal.Font{Size: 13}, Padding: terminal.Padding{X: 16, Y: 12}, Theme: light, DarkTheme: dark, RightClick: terminal.RightClickCopyOrPaste, ClipboardRead: terminal.ClipboardDeny,
		OnTitle: func(_ string) { h.post(e, func() {}) },
		OnExit: func(code int) {
			h.post(e, func() {
				e.status = Status{State: Failed, Message: fmt.Sprintf("Attach exited (%d). Retry to reconnect.", code), ExitCode: code}
			})
		},
		OnKey: func(mods ui.Modifiers, k ui.Key) bool {
			h.mu.Lock()
			valid := h.valid(e) && e.installed && e.status.State == Attached
			var state InputState
			if valid {
				state = instanceInputState(e.owned)
			}
			h.mu.Unlock()
			if !valid {
				return mods == ui.Ctrl && k == ui.KeyB
			}
			result := KeyResult{}
			if h.cfg.OnKey != nil {
				result = h.cfg.OnKey(e.binding, mods, k, state)
			}
			if !result.Consumed && mods == ui.Ctrl && k == ui.KeyB {
				result = KeyResult{Consumed: true, Send: []byte{2, 2}}
			}
			if result.Consumed && len(result.Send) > 0 {
				h.mu.Lock()
				if h.valid(e) && e.installed && e.status.State == Attached {
					e.owned.Send(result.Send)
				}
				h.mu.Unlock()
			}
			return result.Consumed
		},
		OnScroll: func(rows int) bool {
			h.mu.Lock()
			valid := h.valid(e) && e.installed && e.status.State == Attached
			h.mu.Unlock()
			if valid && h.cfg.OnScroll != nil {
				h.cfg.OnScroll(e.binding, rows)
			}
			return true
		},
		OnClear: func(terminal.Clear) bool {
			h.mu.Lock()
			valid := h.valid(e)
			if valid {
				e.status.Message = "Clear unavailable for a mirrored Herdr screen"
			}
			h.mu.Unlock()
			if valid {
				h.changed()
			}
			return true
		},
	}, nil
}

func instanceInputState(t instance) InputState {
	if capable, ok := t.(interface{ InputState() InputState }); ok {
		return capable.InputState()
	}
	return InputState{}
}

// InputState returns an unavailable capability for stale or unattached bindings.
func (h *Host) InputState(b Binding) InputState {
	h.mu.Lock()
	defer h.mu.Unlock()
	if e := h.lookup(b); e != nil && e.installed && e.status.State == Attached {
		return instanceInputState(e.owned)
	}
	return InputState{}
}

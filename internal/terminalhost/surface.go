package terminalhost

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/Maidang1/herdr-ui/internal/surfaceview"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

type surfaceSelection struct {
	generation uint64
	tab, pane  string
	cols, rows uint16
	focused    bool
}
type surfaceInput struct {
	selection    surfaceSelection
	connection   *endpoint.Client
	binding      Binding
	boot, target string
	event        endpoint.PaneEvent
	popup        bool
}

// surfaceBackend owns one connection per window, not one per pane. Selection
// and geometry belong to that connection; no attach subprocess or PTY exists.
// A single worker serializes input and selection changes without blocking UI.
type surfaceBackend struct {
	h        *Host
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	wake     chan struct{}
	inputs   chan surfaceInput
	mu       sync.Mutex
	client   *endpoint.Client
	desired  surfaceSelection
	geometry surfaceGeometryState
	err      error
}

func newSurfaceBackend(h *Host) *surfaceBackend {
	ctx, cancel := context.WithCancel(context.Background())
	s := &surfaceBackend{h: h, ctx: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1), inputs: make(chan surfaceInput, 128)}
	go s.run()
	return s
}
func (s *surfaceBackend) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}
func (s *surfaceBackend) selection() surfaceSelection {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.desired
}
func (s *surfaceBackend) update(v surfaceSelection) {
	s.mu.Lock()
	v.generation = s.desired.generation
	changed := s.desired != v
	if changed {
		v.generation++
	}
	s.desired = v
	s.mu.Unlock()
	if changed {
		s.signal()
	}
}
func (s *surfaceBackend) publish(c *endpoint.Client, err error) {
	s.mu.Lock()
	s.client, s.err = c, err
	s.mu.Unlock()
	s.h.changed()
}
func (s *surfaceBackend) snapshot() endpoint.State {
	state, _ := s.snapshotConnection()
	return state
}
func (s *surfaceBackend) snapshotConnection() (endpoint.State, *endpoint.Client) {
	s.mu.Lock()
	c := s.client
	s.mu.Unlock()
	if c == nil {
		return endpoint.State{}, nil
	}
	return c.Latest(), c
}
func (s *surfaceBackend) status() Status {
	s.mu.Lock()
	err := s.err
	s.mu.Unlock()
	if err != nil {
		return Status{State: Failed, Message: "Herdr surface unavailable: " + err.Error()}
	}
	v := s.snapshot()
	if !v.Connected || v.Surface == nil {
		return Status{State: Starting, Message: "Connecting terminal surface…"}
	}
	return Status{State: Attached}
}
func (s *surfaceBackend) run() {
	defer close(s.done)
	for s.ctx.Err() == nil {
		// Do not connect or acquire geometry before a visible tab is selected.
		if s.selection().tab == "" {
			select {
			case <-s.ctx.Done():
				return
			case <-s.wake:
			}
			continue
		}
		c := endpoint.NewSurface(s.h.cfg.Target.Socket)
		c.SetChanged(func() { s.signal(); s.h.changed() })
		if err := c.Start(s.ctx); err != nil {
			s.publish(nil, err)
		} else {
			s.publish(c, nil)
			err := s.connected(c)
			c.Close()
			s.publish(nil, err)
		}
		// Queued input is never replayed across a connection or boot change.
		for len(s.inputs) > 0 {
			<-s.inputs
		}
		select {
		case <-s.ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
func (s *surfaceBackend) connected(c *endpoint.Client) error {
	var applied surfaceSelection
	for s.ctx.Err() == nil {
		state := c.Latest()
		if !state.Connected {
			if state.Err != nil {
				return state.Err
			}
			return errors.New("endpoint disconnected")
		}
		want := s.selection()
		if state.Snapshot != nil && want.tab != "" && want.cols > 0 && want.rows > 0 && want != applied {
			if applied.tab == "" {
				// Passive clients may set geometry, but the daemon requires an
				// active surface before accepting per-window focus requests.
				if err := c.ResizeSurface(s.ctx, want.cols, want.rows); err != nil {
					return err
				}
				if err := c.SetSurfaceActive(s.ctx, true); err != nil {
					return err
				}
			}
			if applied.tab != want.tab {
				if err := c.FocusTab(s.ctx, want.tab); err != nil {
					return err
				}
			}
			if applied.tab != "" && (applied.cols != want.cols || applied.rows != want.rows) {
				if err := c.ResizeSurface(s.ctx, want.cols, want.rows); err != nil {
					return err
				}
			}
			if applied.pane != want.pane && want.pane != "" {
				if err := c.FocusPane(s.ctx, want.pane); err != nil {
					return err
				}
			}
			if applied.tab == "" || applied.focused != want.focused {
				if err := c.FocusSurface(s.ctx, want.focused); err != nil {
					return err
				}
			}
			applied = want
		} else if want.tab == "" && applied.tab != "" {
			if err := c.SetSurfaceActive(s.ctx, false); err != nil {
				return err
			}
			applied = surfaceSelection{}
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-s.wake:
		case input := <-s.inputs:
			// A key queued before a tab switch or pane retirement must not be
			// delivered later to a hidden pane or a newly reconnected process.
			if input.selection != applied || input.connection != c || s.selection() != applied || input.binding.TerminalID == "" {
				continue
			}
			s.h.mu.Lock()
			e := s.h.lookup(input.binding)
			valid := e != nil && e.installed && e.spec.PaneID == applied.pane
			s.h.mu.Unlock()
			state = c.Latest()
			if !valid || state.Snapshot == nil || state.Snapshot.BootID != input.boot || state.Snapshot.FocusedTabID != applied.tab {
				continue
			}
			hasPopup := state.Surface != nil && state.Surface.Popup != nil
			if hasPopup != input.popup {
				continue
			}
			if input.popup && (state.Snapshot.FocusedPaneID != applied.pane || state.Surface.Popup.TerminalID != input.target) {
				continue
			}
			if err := c.SendPaneEvent(s.ctx, input.boot, input.target, input.event, input.popup); err != nil {
				return err
			}
		}
	}
	return s.ctx.Err()
}

type surfaceInstance struct {
	windowsTextKeys bool
	suppressText    bool
	scrollRemainder float32
	pendingTextKey  *surfacePendingKey
	backend         *surfaceBackend
	entry           *entry
	view            *surfaceview.View
	frame           *endpoint.SurfaceFrame
	frameConnection *endpoint.Client
	frameSelection  surfaceSelection
	closed          atomic.Bool
	mouseButton     int
}
type surfacePendingKey struct {
	event      ui.InputEvent
	selection  surfaceSelection
	connection *endpoint.Client
}

func (s *surfaceBackend) open(e *entry, o terminal.Options) instance {
	p := &surfaceInstance{backend: s, entry: e, mouseButton: -1, windowsTextKeys: runtime.GOOS == "windows"}
	p.view = surfaceview.New(surfaceview.Config{Font: o.Font, Theme: o.Theme, DarkTheme: o.DarkTheme, CopyOnSelect: o.CopyOnSelect, MinimumContrast: o.MinimumContrast,
		OnInput: p.input,
		OnPaste: func(text string) bool { p.eventText(text, true); return true },
		Copy: func(text string) {
			if mygo.Clipboard.Write(transfer.TextData(text)) == nil && s.h.cfg.OnCopy != nil {
				s.h.cfg.OnCopy()
			}
		},
		Paste: func() string {
			if data, err := mygo.Clipboard.Read(); err == nil && s.h.cfg.OnPaste != nil && s.h.cfg.OnPaste(e.binding, data) {
				return ""
			}
			return mygo.Clipboard.ReadText()
		},
		OnSearch: func(status terminal.SearchStatus) {
			if s.h.cfg.OnSearch != nil {
				s.h.cfg.OnSearch(e.binding, status)
			}
		},
	})
	return p
}
func (p *surfaceInstance) View(c *ui.Context) ui.Element {
	state, connection := p.backend.snapshotConnection()
	selected := p.backend.selection()
	present := false
	if state.Surface != nil {
		for _, pane := range state.Surface.Panes {
			if pane.PaneID == p.entry.spec.PaneID {
				present = true
				break
			}
		}
	}
	if state.Connected && state.Snapshot != nil && state.Snapshot.FocusedTabID == selected.tab && state.SurfaceSnapshot != nil && state.SurfaceSnapshot.FocusedTabID == selected.tab && present && state.Surface.BootID == state.Snapshot.BootID {
		p.frame = state.Surface
		p.frameConnection, p.frameSelection = connection, selected
		p.view.SetFrame(*state.Surface, p.entry.spec.PaneID)
	} else {
		p.frame = nil
		p.frameConnection = nil
		p.view.SetFrame(endpoint.SurfaceFrame{}, p.entry.spec.PaneID)
	}
	el := p.view.View(c)
	if !el.Focused() || p.view.Composing() {
		p.pendingTextKey = nil
		p.suppressText = false
	}
	return el
}
func (p *surfaceInstance) enqueue(event endpoint.PaneEvent) {
	if p.closed.Load() || len(event) == 0 || p.frame == nil {
		return
	}
	want := p.backend.selection()
	if want.pane != p.entry.spec.PaneID || want != p.frameSelection {
		return
	}
	target, popup := p.entry.spec.PaneID, false
	if p.frame.Popup != nil && p.pane() != nil && p.pane().Focused {
		target, popup = p.frame.Popup.TerminalID, true
	}
	item := surfaceInput{selection: p.frameSelection, connection: p.frameConnection, binding: p.entry.binding, boot: p.frame.BootID, target: target, popup: popup, event: event}
	select {
	case p.backend.inputs <- item:
	default:
		// Saturation is visible and disconnects rather than losing a byte
		// inside a command while continuing to send subsequent input.
		p.backend.mu.Lock()
		c := p.backend.client
		p.backend.err = errors.New("terminal input queue is full; reconnecting")
		p.backend.mu.Unlock()
		if c != nil {
			c.Close()
		}
	}
}
func (p *surfaceInstance) eventText(text string, paste bool) {
	event, err := endpoint.TextEvent(text, paste)
	if err == nil {
		p.enqueue(event)
	}
}
func (p *surfaceInstance) input(ev ui.InputEvent) bool {
	if p.closed.Load() {
		return true
	}
	s := p.backend
	switch ev.Kind {
	case ui.InputText:
		p.pendingTextKey = nil
		if p.suppressText {
			p.suppressText = false
			return true
		}
		p.eventText(ev.Text, false)
		return true
	case ui.InputKeyUp:
		if p.pendingTextKey != nil && ev.Key == p.pendingTextKey.event.Key {
			p.flushTextKey()
			return true
		}
	case ui.InputKeyDown:
		p.flushTextKey()
		p.suppressText = false
		if s.h.cfg.OnKey != nil && !p.view.Composing() {
			result := s.h.cfg.OnKey(p.entry.binding, ev.Mods, ev.Key, p.InputState())
			if result.Consumed {
				p.suppressText = true
				if len(result.Send) > 0 {
					code, char, fn, ok := surfaceKey(ev.Key)
					if code == 10 && ev.Mods&ui.Shift != 0 {
						code = 11
					}
					if ok {
						event, err := endpoint.KeyEvent(code, char, fn, surfaceModifiers(ev.Mods), ev.Repeat)
						if err == nil {
							p.enqueue(event)
						}
					}
				}
				return true
			}
		}
		code, char, fn, ok := surfaceKey(ev.Key)
		if !ok {
			return false
		}
		if code == 10 && ev.Mods&ui.Shift != 0 {
			code = 11
		}
		// Printable characters arrive through InputText after the input
		// method has resolved the keyboard layout and composition.
		if code == 15 && ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) == 0 {
			return false
		}
		// Windows reports AltGr as Ctrl+Alt, before the committed WM_CHAR.
		// Wait for that text; a genuine chord with no text is sent on release.
		if p.windowsTextKeys && code == 15 && ev.Mods&(ui.Ctrl|ui.Alt|ui.Super) == ui.Ctrl|ui.Alt {
			p.pendingTextKey = &surfacePendingKey{event: ev, selection: p.frameSelection, connection: p.frameConnection}
			return true
		}
		event, err := endpoint.KeyEvent(code, char, fn, surfaceModifiers(ev.Mods), ev.Repeat)
		if err == nil {
			p.enqueue(event)
		}
		return true
	case ui.InputScroll:
		_, height := surfaceview.CellMetrics(s.h.cfg.Font)
		rows := surfaceScrollRows(ev, height, &p.scrollRemainder)
		if rows == 0 {
			return true
		}
		pane := p.pane()
		popup := p.frame != nil && p.frame.Popup != nil && pane != nil && pane.Focused
		reporting := pane != nil && (pane.MouseReporting || pane.AlternateScreenActive)
		if popup {
			reporting = p.frame.Popup.MouseReporting
		}
		if reporting {
			p.mouseWithLines(ev, rows)
			return true
		}
		if s.h.cfg.OnScroll != nil && ev.DY != 0 {
			s.h.cfg.OnScroll(p.entry.binding, rows)
		}
		return true
	case ui.InputPointerDown, ui.InputPointerUp, ui.InputPointerMove:
		pane := p.pane()
		reporting := pane != nil && pane.MouseReporting
		if p.frame != nil && p.frame.Popup != nil && pane != nil && pane.Focused {
			reporting = p.frame.Popup.MouseReporting
		}
		if reporting && (ev.Mods&ui.Shift == 0 || p.mouseButton >= 0) {
			p.mouse(ev)
			return true
		}
	}
	return false
}
func (p *surfaceInstance) flushTextKey() {
	pending := p.pendingTextKey
	p.pendingTextKey = nil
	if pending == nil || p.view.Composing() || pending.selection != p.frameSelection || pending.connection != p.frameConnection {
		return
	}
	code, char, fn, ok := surfaceKey(pending.event.Key)
	if ok {
		event, err := endpoint.KeyEvent(code, char, fn, surfaceModifiers(pending.event.Mods), pending.event.Repeat)
		if err == nil {
			p.enqueue(event)
		}
	}
}
func (p *surfaceInstance) pane() *endpoint.SurfacePane {
	if p.frame != nil {
		for i := range p.frame.Panes {
			if p.frame.Panes[i].PaneID == p.entry.spec.PaneID {
				return &p.frame.Panes[i]
			}
		}
	}
	return nil
}
func (p *surfaceInstance) mouse(ev ui.InputEvent) {
	p.mouseWithLines(ev, 0)
}
func (p *surfaceInstance) mouseWithLines(ev ui.InputEvent, scrollRows int) {
	w, h := surfaceview.CellMetrics(p.backend.h.cfg.Font)
	cols, rows := 1, 1
	if pane := p.pane(); pane != nil {
		cols, rows = int(pane.InnerRect.Width), int(pane.InnerRect.Height)
	}
	if p.frame != nil && p.frame.Popup != nil && p.pane() != nil && p.pane().Focused {
		cols, rows = int(p.frame.Popup.Frame.Width), int(p.frame.Popup.Frame.Height)
	}
	col, row := uint16(max(0, min(cols-1, int(ev.X/w)))), uint16(max(0, min(rows-1, int(ev.Y/h))))
	kind := uint8(3)
	button := uint8(ev.Button)
	if button > 2 {
		button = 0
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		kind = 0
		p.mouseButton = int(button)
	case ui.InputPointerUp:
		kind = 1
		if p.mouseButton >= 0 {
			button = uint8(p.mouseButton)
		}
		p.mouseButton = -1
	case ui.InputPointerMove:
		if p.mouseButton >= 0 {
			kind = 2
			button = uint8(p.mouseButton)
		}
	case ui.InputScroll:
		kind = 5
		if ev.DY < 0 {
			kind = 4
		}
		if ev.DY == 0 {
			kind = 7
			if ev.DX < 0 {
				kind = 6
			}
		}
	}
	lines := uint16(0)
	if ev.Kind == ui.InputScroll {
		lines = uint16(min(1000, max(1, int(math.Abs(float64(scrollRows))))))
	}
	event, err := endpoint.MouseEvent(kind, button, col, row, surfaceModifiers(ev.Mods), lines)
	if err == nil {
		p.enqueue(event)
	}
}
func surfaceModifiers(m ui.Modifiers) uint8 {
	var b uint8
	if m&ui.Shift != 0 {
		b |= 1
	}
	if m&ui.Ctrl != 0 {
		b |= 2
	}
	if m&ui.Alt != 0 {
		b |= 4
	}
	if m&ui.Super != 0 {
		b |= 8
	}
	return b
}
func surfaceKey(k ui.Key) (uint8, rune, uint8, bool) {
	if k >= ui.KeyA && k <= ui.KeyZ {
		return 15, 'a' + rune(k-ui.KeyA), 0, true
	}
	if k >= ui.Key0 && k <= ui.Key9 {
		return 15, '0' + rune(k-ui.Key0), 0, true
	}
	if k >= ui.KeyF1 && k <= ui.KeyF12 {
		return 16, 0, uint8(k-ui.KeyF1) + 1, true
	}
	keys := map[ui.Key]uint8{ui.KeyBackspace: 0, ui.KeyEnter: 1, ui.KeyLeft: 2, ui.KeyRight: 3, ui.KeyUp: 4, ui.KeyDown: 5, ui.KeyHome: 6, ui.KeyEnd: 7, ui.KeyPageUp: 8, ui.KeyPageDown: 9, ui.KeyTab: 10, ui.KeyDelete: 12, ui.KeyInsert: 13, ui.KeyEscape: 14}
	if code, ok := keys[k]; ok {
		return code, 0, 0, true
	}
	chars := map[ui.Key]rune{ui.KeySpace: ' ', ui.KeyMinus: '-', ui.KeyEqual: '=', ui.KeyComma: ',', ui.KeyPeriod: '.', ui.KeySlash: '/', ui.KeySemicolon: ';', ui.KeyQuote: '\'', ui.KeyBracketLeft: '[', ui.KeyBracketRight: ']', ui.KeyBackslash: '\\', ui.KeyBackquote: '`'}
	r, ok := chars[k]
	return 15, r, 0, ok
}

func (p *surfaceInstance) Send(b []byte) {
	// This interface carries literal prefix/copy-mode keys. Undo attach's
	// doubled Ctrl+B escape; do not inject terminal escape sequences as text.
	if len(b) == 2 && b[0] == 2 && b[1] == 2 {
		b = b[:1]
	}
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		b = b[n:]
		if r < 32 || r == 127 {
			code, char, mods := uint8(15), r+96, uint8(2)
			switch r {
			case 13:
				code, mods = 1, 0
			case 9:
				code, mods = 10, 0
			case 27:
				code, mods = 14, 0
			case 127:
				code, mods = 0, 0
			}
			ev, err := endpoint.KeyEvent(code, char, 0, mods, false)
			if err == nil {
				p.enqueue(ev)
			}
		} else {
			p.eventText(string(r), false)
		}
	}
}
func (p *surfaceInstance) SetTheme(light, dark *terminal.Theme)    { p.view.SetTheme(light, dark) }
func (p *surfaceInstance) SetFont(f terminal.Font)                 { p.view.SetFont(f) }
func (p *surfaceInstance) SetCopyOnSelect(v terminal.CopyOnSelect) { p.view.SetCopyOnSelect(v) }
func (p *surfaceInstance) SetMinimumContrast(v float32)            { p.view.SetMinimumContrast(v) }
func (p *surfaceInstance) Search(q string)                         { p.view.Search(q) }
func (p *surfaceInstance) SearchNext()                             { p.view.SearchNext() }
func (p *surfaceInstance) SearchPrevious()                         { p.view.SearchPrevious() }
func (p *surfaceInstance) EndSearch()                              { p.view.EndSearch() }
func (p *surfaceInstance) InputState() InputState {
	return InputState{Composing: p.view.Composing(), Guarded: true}
}
func (p *surfaceInstance) Close() error { p.closed.Store(true); return nil }

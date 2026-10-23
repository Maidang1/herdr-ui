package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// Only lifecycle state crosses threads. Window ownership and focus changes stay
// on the UI thread; construction and PTY teardown never run there.
type commandSurfaceState struct {
	mu       sync.Mutex
	title    string
	instance commandSurfaceTerminal
	err      error
	closed   bool
	cancel   context.CancelFunc
	focus    bool // UI thread only.
}
type commandSurfaceTerminal interface {
	View(*native.Context) native.Element
	Close() error
}
type commandSurfaceFactory func(terminal.Options) (commandSurfaceTerminal, error)
type commandSurfacePlugin struct{ terminal *terminal.Terminal }

func (p *commandSurfacePlugin) View(c *native.Context) native.Element {
	return terminal.View(c, p.terminal)
}
func (p *commandSurfacePlugin) Close() error {
	err := p.terminal.Close()
	<-p.terminal.Done()
	return err
}
func createCommandSurface(options terminal.Options) (commandSurfaceTerminal, error) {
	t, err := terminal.New(options)
	if err != nil {
		return nil, err
	}
	return &commandSurfacePlugin{terminal: t}, nil
}

// OpenCommandSurface starts one command in a dedicated terminal, replacing an
// earlier command surface. Call on the UI thread. Creation errors appear inside
// the surface; validation errors are returned before launching any process.
func (w *Window) OpenCommandSurface(title string, command, env []string, cwd string) error {
	return w.openCommandSurface(title, command, env, cwd, createCommandSurface)
}
func (w *Window) openCommandSurface(title string, command, env []string, cwd string, create commandSurfaceFactory) error {
	if w.terminal == nil || w.terminal.post == nil || w.terminal.ctx == nil {
		return errors.New("command surface requires a connected window")
	}
	if err := w.terminal.ctx.Err(); err != nil {
		return err
	}
	if len(command) == 0 || !filepath.IsAbs(command[0]) {
		return errors.New("command surface requires an absolute executable path")
	}
	if strings.TrimSpace(title) == "" {
		title = "Command"
	}
	w.CloseCommandSurface()
	ctx, cancel := context.WithCancel(w.terminal.ctx)
	s := &commandSurfaceState{title: title, cancel: cancel, focus: true}
	w.commandSurfaceState = s
	w.BlurInput()
	post := w.terminal.post
	exited := func() {
		if !s.close() {
			return
		}
		post(func() {
			if w.commandSurfaceState == s {
				w.CloseCommandSurface()
			}
		})
	}
	// Identity blanks are necessary because terminal.Options.Env augments the
	// inherited environment. Caller-supplied values may intentionally override.
	childEnv := append([]string{"HERDR_SESSION=", "HERDR_PANE_ID=", "HERDR_TAB_ID=", "HERDR_WORKSPACE_ID="}, env...)
	opts := terminal.Options{
		Command: slices.Clone(command), Env: childEnv, Dir: cwd,
		Font: terminal.Font{Size: 13}, Padding: terminal.Padding{X: 12, Y: 10},
		ClipboardRead: terminal.ClipboardDeny, RightClick: terminal.RightClickCopyOrPaste,
		OnExit: func(int) { exited() },
	}
	go func() {
		if ctx.Err() != nil {
			return
		}
		instance, err := create(opts)
		s.install(instance, err)
		post(func() {}) // Schedule a paint, including any startup diagnostic.
	}()
	go func() { <-ctx.Done(); exited() }()
	return nil
}
func (s *commandSurfaceState) install(instance commandSurfaceTerminal, err error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		if instance != nil {
			go instance.Close()
		}
		return
	}
	s.instance, s.err = instance, err
	s.mu.Unlock()
}
func (s *commandSurfaceState) close() bool {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	s.closed = true
	instance, cancel := s.instance, s.cancel
	s.instance = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if instance != nil {
		go instance.Close()
	}
	return true
}
func (s *commandSurfaceState) current() (commandSurfaceTerminal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instance, s.err
}

// CloseCommandSurface is UI-thread owned and returns before PTY cleanup.
func (w *Window) CloseCommandSurface() {
	s := w.commandSurfaceState
	if s == nil {
		return
	}
	w.commandSurfaceState = nil
	s.close()
	w.BlurInput()
}

func (w *Window) commandSurface(c *native.Context, p design.Palette) {
	s := w.commandSurfaceState
	if s == nil {
		return
	}
	native.Column(c.Key("command-surface")).Grow(1).FillWidth().MinHeight(0).AlignItems(native.Stretch).Background(p.Content).Clip().Children(func() {
		native.Row(c.Key("command-surface-header")).FillWidth().Shrink(0).Padding(10, 14).Gap(12).AlignItems(native.Center).Background(p.Elevated).Children(func() {
			native.Text(c, s.title).Bold().SingleLine().Grow(1)
			if native.Button(c, "Close").Label("Close " + s.title).Clicked() {
				w.CloseCommandSurface()
			}
		})
		if w.commandSurfaceState != s {
			return
		}
		instance, err := s.current()
		if err != nil {
			native.Column(c).Grow(1).FillWidth().Center().Gap(8).Padding(24).Children(func() {
				native.Text(c, "Unable to start "+s.title).Bold().TextColor(p.Danger)
				native.Text(c, fmt.Sprint(err)).TextColor(p.Secondary).Selectable()
			})
			return
		}
		if instance == nil {
			native.Column(c).Grow(1).FillWidth().Center().Children(func() { native.Text(c, "Starting…").TextColor(p.Secondary) })
			return
		}
		// No outer shortcut hook is installed: the child client owns prefix keys,
		// modal commands, mouse reporting, text composition and terminal encoding.
		el := instance.View(c.Key("command-surface-terminal")).Grow(1).FillWidth().MinHeight(0)
		if s.focus {
			el.Focus()
			s.focus = false
		}
	})
}

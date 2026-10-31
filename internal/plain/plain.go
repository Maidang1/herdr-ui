// Package plain is Hergo without Herdr: a quiet window of local shells in
// tabs, for when Herdr is not installed or not wanted. It keeps only what
// a terminal needs — tabs, the shell, and a way back to Herdr.
package plain

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// Options are what a plain window starts its shells with, and how it
// reaches the rest of the app. Every callback runs on the UI thread.
type Options struct {
	// Shell is the command each tab runs; DefaultShell when empty.
	Shell []string
	Dir   string
	Env   []string
	Font  terminal.Font
	// CopyOnSelect, MinimumContrast, OptionAsAlt and the cursor follow
	// the persisted terminal settings.
	CopyOnSelect    terminal.CopyOnSelect
	MinimumContrast float32
	OptionAsAlt     bool
	Cursor          terminal.CursorStyle
	NoBlink         bool
	// Post runs fn on the UI thread with the next frame.
	Post func(fn func())
	// Installation reports what was found of Herdr; Recheck looks again
	// and then posts a frame.
	Installation func() herdr.Installation
	Recheck      func()
	// ConnectHerdr opens Hergo's Herdr window; nil hides the way back.
	ConnectHerdr func() error
	// AlwaysPlain reports and sets whether Hergo opens as a plain terminal
	// even when Herdr is installed.
	AlwaysPlain    func() bool
	SetAlwaysPlain func(bool)
	// OpenURL opens a web page with the system.
	OpenURL func(string)
	// Close closes the window, as when its last tab ends and Close is set.
	Close func()
}

// DefaultShell is the user's login shell: $SHELL, or the platform's.
func DefaultShell(goos string, getenv func(string) string) []string {
	if goos == "windows" {
		if comspec := getenv("COMSPEC"); comspec != "" {
			return []string{comspec}
		}
		return []string{"powershell.exe", "-NoLogo"}
	}
	shell := getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
		if goos == "darwin" {
			shell = "/bin/zsh"
		}
	}
	return []string{shell, "-l"}
}

// tab is one shell.
type tab struct {
	id       int
	term     *terminal.Terminal
	starting bool
	err      string
}

// Window is the plain window's state. Its methods run on the UI thread.
type Window struct {
	opts      Options
	tabs      []*tab
	selected  int
	nextID    int
	herdrOpen bool
	message   string
	closed    bool
}

// New makes the window's state without a shell; NewTab starts the first
// once the window that shows it exists, as Post needs it.
func New(opts Options) *Window {
	if len(opts.Shell) == 0 {
		opts.Shell = DefaultShell(runtime.GOOS, os.Getenv)
	}
	if opts.Dir == "" {
		opts.Dir, _ = os.UserHomeDir()
	}
	if opts.Post == nil {
		opts.Post = func(fn func()) { fn() }
	}
	return &Window{opts: opts}
}

// Tabs reports how many shells the window holds.
func (w *Window) Tabs() int { return len(w.tabs) }

// Selected is the index of the tab on show.
func (w *Window) Selected() int { return w.selected }

// NewTab starts a shell in a new tab, in the directory of the shell on
// show, as a terminal's new tab does.
func (w *Window) NewTab() {
	if w.closed {
		return
	}
	w.nextID++
	t := &tab{id: w.nextID, starting: true}
	dir := w.opts.Dir
	if current := w.current(); current != nil && current.term != nil {
		if d := current.term.Dir(); d != "" {
			dir = d
		}
	}
	w.tabs = append(w.tabs, t)
	w.selected = len(w.tabs) - 1
	go w.start(t, dir)
}

func (w *Window) start(t *tab, dir string) {
	env := append(append([]string(nil), w.opts.Env...), "TERM_PROGRAM=Hergo", "HERDR_SESSION=", "HERDR_PANE_ID=", "HERDR_TAB_ID=", "HERDR_WORKSPACE_ID=")
	font := w.opts.Font
	if font.Size <= 0 {
		font.Size = 13
	}
	light, dark := themes()
	term, err := terminal.New(terminal.Options{
		Command: w.opts.Shell, Dir: dir, Env: env, Font: font,
		Theme: light, DarkTheme: dark,
		Padding:      terminal.Padding{X: 14, Y: 10},
		PaddingColor: terminal.PaddingExtend,
		CopyOnSelect: w.opts.CopyOnSelect, MinimumContrast: w.opts.MinimumContrast, OptionAsAlt: w.opts.OptionAsAlt,
		Cursor: w.opts.Cursor, NoBlink: w.opts.NoBlink,
		RightClick: terminal.RightClickCopyOrPaste,
		OnTitle:    func(string) { w.opts.Post(func() {}) },
		OnExit: func(int) {
			w.opts.Post(func() { w.closeTab(t) })
		},
		OnKey: func(mods native.Modifiers, key native.Key) bool {
			command, ok := Shortcut(runtime.GOOS, mods, key)
			if ok {
				w.opts.Post(func() { w.Command(command) })
			}
			return ok
		},
	})
	w.opts.Post(func() {
		t.starting = false
		if err != nil {
			t.err = err.Error()
			return
		}
		if w.closed || w.index(t) < 0 {
			term.Close()
			return
		}
		t.term = term
	})
}

// themes are the terminal's colors on Hergo's own surfaces, so the shell
// and the window read as one sheet.
func themes() (*terminal.Theme, *terminal.Theme) {
	light, dark := terminal.LightTheme(), terminal.DarkTheme()
	light.Background = design.Colors(false).Content
	dark.Background = design.Colors(true).Content
	light.CursorText, dark.CursorText = light.Background, dark.Background
	return light, dark
}

func (w *Window) index(t *tab) int {
	for i, candidate := range w.tabs {
		if candidate == t {
			return i
		}
	}
	return -1
}

func (w *Window) current() *tab {
	if w.selected < 0 || w.selected >= len(w.tabs) {
		return nil
	}
	return w.tabs[w.selected]
}

// closeTab ends a tab's shell and drops the tab. The last tab leaves the
// window's empty state, or closes the window when it can.
func (w *Window) closeTab(t *tab) {
	i := w.index(t)
	if i < 0 {
		return
	}
	if t.term != nil {
		go t.term.Close()
		t.term = nil
	}
	w.tabs = append(w.tabs[:i], w.tabs[i+1:]...)
	if w.selected >= len(w.tabs) {
		w.selected = len(w.tabs) - 1
	} else if w.selected > i {
		w.selected--
	}
	if len(w.tabs) == 0 {
		w.selected = 0
		if w.opts.Close != nil {
			w.opts.Close()
		}
	}
}

// Command runs a window command: new_tab, close_tab, next_tab,
// previous_tab, or select_tab_N (1–9, 9 the last).
func (w *Window) Command(id string) bool {
	switch id {
	case "new_tab":
		w.NewTab()
	case "close_tab", "close_pane":
		if t := w.current(); t != nil {
			w.closeTab(t)
		}
	case "next_tab", "previous_tab":
		if len(w.tabs) > 1 {
			delta := 1
			if id == "previous_tab" {
				delta = -1
			}
			w.selected = (w.selected + delta + len(w.tabs)) % len(w.tabs)
		}
	default:
		var n int
		if _, err := fmt.Sscanf(id, "select_tab_%d", &n); err != nil || n < 1 || n > 9 {
			return false
		}
		if n == 9 || n > len(w.tabs) {
			n = len(w.tabs)
		}
		w.selected = n - 1
	}
	return true
}

// Shortcut maps a key to a window command: Command with the key on macOS,
// Control+Shift elsewhere, so that Control alone stays the shell's.
func Shortcut(goos string, mods native.Modifiers, key native.Key) (string, bool) {
	primary := native.Ctrl | native.Shift
	if goos == "darwin" {
		primary = native.Super
	}
	if mods == native.Ctrl && key == native.KeyTab {
		return "next_tab", true
	}
	if mods == native.Ctrl|native.Shift && key == native.KeyTab {
		return "previous_tab", true
	}
	shifted := primary | native.Shift
	switch {
	case mods == primary && key == native.KeyT:
		return "new_tab", true
	case mods == primary && key == native.KeyW:
		return "close_tab", true
	case mods == shifted && key == native.KeyBracketRight:
		return "next_tab", true
	case mods == shifted && key == native.KeyBracketLeft:
		return "previous_tab", true
	case mods == primary && key >= native.Key1 && key <= native.Key9:
		return fmt.Sprintf("select_tab_%d", int(key-native.Key1)+1), true
	}
	return "", false
}

// Close ends every shell.
func (w *Window) Close() {
	w.closed = true
	for _, t := range w.tabs {
		if t.term != nil {
			t.term.Close()
			t.term = nil
		}
	}
}

// title is a tab's name: the program's title, or the directory it is in,
// or the shell.
func (t *tab) title(shell []string) string {
	if t.term != nil {
		if title := strings.TrimSpace(t.term.Title()); title != "" {
			return title
		}
		if dir := t.term.Dir(); dir != "" {
			if home, _ := os.UserHomeDir(); home != "" && dir == home {
				return "~"
			}
			return filepath.Base(dir)
		}
	}
	if len(shell) > 0 {
		return filepath.Base(shell[0])
	}
	return "Shell"
}

// WindowTitle is the native window's title: the tab on show.
func (w *Window) WindowTitle() string {
	if t := w.current(); t != nil {
		return t.title(w.opts.Shell) + " — Hergo"
	}
	return "Hergo"
}

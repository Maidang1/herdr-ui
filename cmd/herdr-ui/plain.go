package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/plain"
	"github.com/Maidang1/herdr-ui/internal/prefs"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/plugins/updater"
	native "github.com/egoist/mygo/ui"
)

var isDarwin = runtime.GOOS == "darwin"

// startupPlain decides whether Hergo opens as a plain terminal: always
// when asked to, never when Herdr was asked for, and otherwise when Herdr
// is not installed.
func startupPlain(mode string, forcePlain, forceHerdr bool, install herdr.Installation) bool {
	switch {
	case forcePlain:
		return true
	case forceHerdr:
		return false
	case mode == "plain":
		return true
	case mode == "herdr":
		return false
	}
	return install.Path == ""
}

// installationWatch holds what was last found of Herdr, for any thread.
type installationWatch struct {
	binary string
	value  atomic.Pointer[herdr.Installation]
	mu     sync.Mutex
}

func newInstallationWatch(binary string, first herdr.Installation) *installationWatch {
	w := &installationWatch{binary: binary}
	w.value.Store(&first)
	return w
}

func (w *installationWatch) Get() herdr.Installation { return *w.value.Load() }

// Refresh looks for Herdr again; it blocks, so callers run it off the UI
// thread.
func (w *installationWatch) Refresh(ctx context.Context) herdr.Installation {
	w.mu.Lock()
	defer w.mu.Unlock()
	found := herdr.Detect(ctx, w.binary)
	w.value.Store(&found)
	return found
}

// plainWindowConfig is what a plain window needs from the app.
type plainWindowConfig struct {
	settings *prefs.Store
	install  *installationWatch
	// connect opens Herdr's window; nil when that window is already open.
	connect func() error
}

// plainWindows remembers the open plain windows, for the menu commands
// sent while one of them is in front.
var plainWindows sync.Map // *mygo.Window → *plain.Window

func plainFront(front *mygo.Window) *plain.Window {
	if front == nil {
		return nil
	}
	if v, ok := plainWindows.Load(front); ok {
		return v.(*plain.Window)
	}
	return nil
}

// openPlainWindow opens a window of local shells. Call it on the UI thread
// once the app is ready.
func openPlainWindow(cfg plainWindowConfig) *mygo.Window {
	persisted := cfg.settings.Settings()
	copyOnSelect := terminal.CopyOnSelectNone
	if persisted.CopyOnSelect {
		copyOnSelect = terminal.CopyOnSelectClipboard
	}
	var win *mygo.Window
	var closed atomic.Bool
	var screen *plain.Window
	post := func(fn func()) {
		if closed.Load() || win == nil {
			return
		}
		win.Update(func() {
			if closed.Load() {
				return
			}
			fn()
			win.SetTitle(screen.WindowTitle())
		})
	}
	screen = plain.New(plain.Options{
		Env:  os.Environ(),
		Font: terminal.Font{Family: persisted.FontFamily, Size: persisted.FontSize, Weight: persisted.FontWeight, LineHeight: persisted.LineHeight},
		CopyOnSelect: copyOnSelect, MinimumContrast: persisted.MinimumContrast, OptionAsAlt: persisted.OptionAsAlt,
		Cursor: cursorStyle(persisted.Cursor), NoBlink: !persisted.CursorBlink,
		Post:         post,
		Installation: cfg.install.Get,
		Recheck: func() {
			go func() {
				cfg.install.Refresh(context.Background())
				post(screen.Rechecked)
			}()
		},
		ConnectHerdr: cfg.connect,
		AlwaysPlain:  func() bool { return cfg.settings.Settings().StartupMode == "plain" },
		SetAlwaysPlain: func(on bool) {
			mode := "auto"
			if on {
				mode = "plain"
			}
			go func() {
				if _, err := cfg.settings.Update(func(v *prefs.Settings) { v.StartupMode = mode }); err != nil {
					log.Printf("prefs: %v", err)
				}
			}()
		},
		OpenURL: func(url string) {
			go func() {
				if err := mygo.Shell.OpenExternal(url); err != nil {
					log.Printf("open %s: %v", url, err)
				}
			}()
		},
		Close: func() {
			if win != nil {
				win.Close()
			}
		},
	})
	win = mygo.NewWindow(mygo.WindowOptions{Title: "Hergo", Width: 960, Height: 640, MinWidth: 480, MinHeight: 320, StateKey: "plain", TitleBarStyle: platformTitleBar(), TitleBarHeight: 40, Content: native.View(screen.View)})
	plainWindows.Store(win, screen)
	win.Update(screen.NewTab)
	win.OnClosed(func() {
		closed.Store(true)
		plainWindows.Delete(win)
		screen.Close()
	})
	return win
}

// runPlainApp runs Hergo as a plain terminal: no Herdr connection, one
// window of shells, and a way back to Herdr once it is installed.
func runPlainApp(settings *prefs.Store, install *installationWatch) {
	mygo.Theme.SetSource(mygo.ThemeSource(settings.Settings().WindowTheme))
	mygo.App.WhenReady(func() {
		connect := func() error {
			if !install.Get().Installed() {
				return fmt.Errorf("Herdr is not installed")
			}
			if err := relaunch("-connect"); err != nil {
				return err
			}
			mygo.App.Quit()
			return nil
		}
		var newWindow func()
		newWindow = func() {
			openPlainWindow(plainWindowConfig{settings: settings, install: install, connect: connect})
		}
		command := func(id string) func(*mygo.MenuItem, *mygo.Window) {
			return func(_ *mygo.MenuItem, front *mygo.Window) {
				if screen := plainFront(front); screen != nil {
					front.Update(func() { screen.Command(id) })
				} else if id == "new_tab" {
					newWindow()
				}
			}
		}
		var tabs []*mygo.MenuItem
		for n := 1; n <= 9; n++ {
			label := fmt.Sprintf("Tab %d", n)
			if n == 9 {
				label = "Last Tab"
			}
			tabs = append(tabs, &mygo.MenuItem{Label: label, Accelerator: plainAccelerator(fmt.Sprint(n)), Click: command(fmt.Sprintf("select_tab_%d", n))})
		}
		shell := []*mygo.MenuItem{
			{Label: "New Tab", Accelerator: plainAccelerator("T"), Click: command("new_tab")},
			{Label: "New Window", Accelerator: plainAccelerator("N"), Click: func(*mygo.MenuItem, *mygo.Window) { newWindow() }},
			{Type: mygo.MenuItemSeparator},
			{Label: "Close Tab", Accelerator: plainAccelerator("W"), Click: command("close_tab")},
			{Type: mygo.MenuItemSeparator},
			{Label: "Next Tab", Accelerator: plainAccelerator("Shift+]"), Click: command("next_tab")},
			{Label: "Previous Tab", Accelerator: plainAccelerator("Shift+["), Click: command("previous_tab")},
			{Label: "Select Tab", Submenu: tabs},
		}
		appItems := []*mygo.MenuItem{{Role: mygo.RoleAbout}, updater.MenuItem(), {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleServices}, {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleQuit}}
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "Hergo", Submenu: appItems}, platformEditMenu(), {Label: "Shell", Submenu: shell}, {Role: mygo.RoleWindowMenu}}))
		newWindow()
		go install.Refresh(context.Background())
	})
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
}

// plainAccelerator is a plain window shortcut in the platform's terms:
// Command on macOS, Control+Shift elsewhere, as plain.Shortcut reads them.
func plainAccelerator(key string) string {
	if isDarwin {
		return "Cmd+" + key
	}
	if len(key) > 6 && key[:6] == "Shift+" {
		return "Ctrl+" + key
	}
	return "Ctrl+Shift+" + key
}

// relaunch starts a new instance of this executable with extra arguments,
// for switching between the plain terminal and Herdr.
func relaunch(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = os.Environ()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

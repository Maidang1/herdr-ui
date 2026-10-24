package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/prefs"
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/Maidang1/herdr-ui/internal/sidebarinfo"
	"github.com/Maidang1/herdr-ui/internal/terminalhost"
	view "github.com/Maidang1/herdr-ui/internal/ui"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

func runStateWindow(socket, binary string, closeAfter time.Duration) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	target, err := herdr.ResolveTarget(socket, home, os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	settings, prefsErr := prefs.Open("")
	if prefsErr != nil {
		log.Printf("prefs: %v", prefsErr)
	}
	// The saved appearance applies before the first window exists, as the
	// framework's SetSource contract expects.
	mygo.Theme.SetSource(mygo.ThemeSource(settings.Settings().WindowTheme))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := session.New(target)
	a := app.New()
	resolvedBinary := binary
	if path, err := exec.LookPath(binary); err == nil {
		if absolute, err := filepath.Abs(path); err == nil {
			resolvedBinary = absolute
		}
	}
	var host *terminalhost.Host
	var sidebar *sidebarinfo.Collector
	var notifier *notifier
	var closed, pending, starting, started atomic.Bool
	var startup sync.WaitGroup
	mygo.App.WhenReady(func() {
		var win *mygo.Window
		var schedule func()
		schedule = func() {
			if closed.Load() || !pending.CompareAndSwap(false, true) {
				return
			}
			win.Update(func() {
				if closed.Load() {
					pending.Store(false)
					return
				}
				latest := s.Latest()
				a.Update(latest)
				notifier.Step(latest)
				if sidebar != nil {
					sidebar.Update(latest)
				}
				log.Printf("session target=%d epoch=%d revision=%d state=%s", latest.TargetID, latest.Epoch, latest.Revision, latest.State)
				pending.Store(false)
				// A notification coalesced while pending was true must not lose the
				// final publication, even if no further event will arrive.
				if s.Latest() != latest {
					schedule()
				}
			})
		}
		start := func() {
			if closed.Load() {
				return
			}
			if started.Load() {
				s.Refresh()
				return
			}
			if !starting.CompareAndSwap(false, true) {
				return
			}
			startup.Add(1)
			go func() {
				defer startup.Done()
				defer starting.Store(false)
				if err := baseline(ctx, binary); err != nil {
					if !closed.Load() {
						win.Update(func() {
							if !closed.Load() {
								a.Update(session.PublishedSnapshot{State: session.Incompatible, Stale: true, Error: err.Error()})
							}
						})
					}
					return
				}
				if closed.Load() {
					return
				}
				started.Store(true)
				s.Start(ctx, schedule)
			}()
		}
		screen := view.New(a, start)
		bindings, bindingErr := input.LoadBindings("")
		screen.ConfigureBindings(bindings, bindingErr)
		post := func(fn func()) {
			if !closed.Load() {
				win.Update(func() {
					if !closed.Load() {
						fn()
					}
				})
			}
		}
		notifier = newNotifier(screen.WindowActive, func(workspaceID, paneID string) { post(func() { screen.JumpTo(workspaceID, paneID) }) })
		sidebar = sidebarinfo.New(target, func(p sidebarinfo.Publication) { post(func() { a.UpdateSidebar(p) }) })
		host = terminalhost.New(terminalhost.Config{Binary: resolvedBinary, Target: target, Env: os.Environ(), Font: terminal.Font{Size: settings.Settings().FontSize}, Post: post, Changed: func() { post(func() {}) }, OnScroll: screen.Scroll, OnKey: screen.Key})
		screen.ConnectTerminal(host, s, ctx, post)
		screen.SetInitialFontSize(settings.Settings().FontSize)
		preferences := mygo.Theme.Preferences()
		screen.SetReducedTransparency(preferences.ReduceTransparency)
		material := mygo.VibrancySidebar
		if preferences.ReduceTransparency || preferences.HighContrast {
			material = mygo.VibrancyNone
		}
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Herdr", Width: 1200, Height: 800, MinWidth: 820, MinHeight: 560, TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 44, Vibrancy: material, Content: native.View(screen.View)})
		keyboard := &keyboardEndpoint{}
		hooks := keyboardHooks(ctx, s, keyboard, screen, post, func() { win.Close() }, resolvedBinary, target)
		hooks.FontSize = func(size float32) {
			host.SetFont(terminal.Font{Size: size})
			go func() {
				if _, err := settings.Update(func(v *prefs.Settings) { v.FontSize = size }); err != nil {
					log.Printf("prefs: %v", err)
				}
			}()
		}
		screen.SetActionHooks(hooks)
		go keyboard.run(ctx, target.Socket, func() { post(func() {}) })
		win.OnBlur(func() { post(func() { screen.SetWindowActive(false) }) })
		win.OnFocus(func() { post(func() { screen.SetWindowActive(true); notifier.Clear() }) })
		themeItems := map[mygo.ThemeSource]*mygo.MenuItem{}
		offAppearance := mygo.Theme.OnUpdated(func() {
			if closed.Load() {
				return
			}
			preferences := mygo.Theme.Preferences()
			material := mygo.VibrancySidebar
			if preferences.ReduceTransparency || preferences.HighContrast {
				material = mygo.VibrancyNone
			}
			for source, item := range themeItems {
				item.SetChecked(source == mygo.Theme.Source())
			}
			win.SetVibrancy(material)
			post(func() { screen.SetReducedTransparency(preferences.ReduceTransparency) })
		})
		viewItems := []*mygo.MenuItem{}
		terminalItems := []*mygo.MenuItem{}
		helpItems := []*mygo.MenuItem{}
		menuItems := map[string]*mygo.MenuItem{}
		menuEnabled := map[string]bool{}
		menuLabels := map[string]string{}
		// Window-level commands keep their direct key equivalents, as Herdr
		// and Kero do; the prefix chords remain Herdr's own layer beneath.
		accelerators := map[string]string{"zoom_in": "CmdOrCtrl+=", "zoom_out": "CmdOrCtrl+-", "zoom_reset": "CmdOrCtrl+0", "toggle_sidebar": "CmdOrCtrl+B", "attention": "CmdOrCtrl+J", "find": "CmdOrCtrl+F", "find_next": "CmdOrCtrl+G", "find_previous": "CmdOrCtrl+Shift+G", "clear_pane": "CmdOrCtrl+K", "zen": "Ctrl+Cmd+Z"}
		for _, action := range view.Actions {
			action := action
			label := screen.ActionLabel(action.ID)
			if binding := screen.BindingLabel(action.ID); binding != "" {
				label += " · " + binding
			}
			item := &mygo.MenuItem{Label: label, Accelerator: accelerators[action.ID], Disabled: !screen.Can(action.ID), Click: func(_ *mygo.MenuItem, _ *mygo.Window) { post(func() { screen.Dispatch(action.ID) }) }}
			switch action.ID {
			case "toggle_sidebar", "previous_tab", "next_tab", "zoom_in", "zoom_out", "zoom_reset", "zen":
				viewItems = append(viewItems, item)
			case "attention":
				terminalItems = append([]*mygo.MenuItem{item}, terminalItems...)
			case "help":
				helpItems = append(helpItems, item)
			default:
				terminalItems = append(terminalItems, item)
			}
			menuItems[action.ID] = item
			menuEnabled[action.ID] = !item.Disabled
			menuLabels[action.ID] = label
		}
		appearanceItems := []*mygo.MenuItem{}
		for _, choice := range []struct {
			label  string
			source mygo.ThemeSource
		}{{"System", mygo.ThemeSystem}, {"Light", mygo.ThemeLight}, {"Dark", mygo.ThemeDark}} {
			source := choice.source
			item := &mygo.MenuItem{Label: choice.label, Type: mygo.MenuItemRadio, Checked: mygo.Theme.Source() == source, Click: func(_ *mygo.MenuItem, _ *mygo.Window) {
				mygo.Theme.SetSource(source)
				go func() {
					if _, err := settings.Update(func(v *prefs.Settings) { v.WindowTheme = string(source) }); err != nil {
						log.Printf("prefs: %v", err)
					}
				}()
			}}
			themeItems[source] = item
			appearanceItems = append(appearanceItems, item)
		}
		viewItems = append(viewItems, &mygo.MenuItem{Type: mygo.MenuItemSeparator}, &mygo.MenuItem{Label: "Appearance", Submenu: appearanceItems})
		screen.SetActionStateChanged(func(id string, enabled bool) {
			label := screen.ActionLabel(id)
			if binding := screen.BindingLabel(id); binding != "" {
				label += " · " + binding
			}
			if menuLabels[id] != label {
				menuLabels[id] = label
				menuItems[id].SetLabel(label)
			}
			if menuEnabled[id] != enabled {
				menuEnabled[id] = enabled
				menuItems[id].SetEnabled(enabled)
			}
		})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}, {Label: "View", Submenu: viewItems}, {Label: "Terminal", Submenu: terminalItems}, {Label: "Help", Submenu: helpItems}, {Role: mygo.RoleWindowMenu}}))
		win.OnClosed(func() {
			offAppearance()
			screen.CloseCommandSurface()
			screen.DetachTerminal()
			closed.Store(true)
			cancel()
		})
		start()
		if closeAfter > 0 {
			time.AfterFunc(closeAfter, func() {
				if !closed.Load() {
					win.Update(func() {
						if !closed.Load() {
							win.Close()
						}
					})
				}
			})
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Print(err)
	}
	closed.Store(true)
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	if sidebar != nil {
		if err := sidebar.Close(shutdown); err != nil {
			log.Printf("sidebar shutdown: %v", err)
		}
	}
	if err := s.Close(shutdown); err != nil {
		log.Printf("session shutdown: %v", err)
	}
	if host != nil {
		if err := host.Close(shutdown); err != nil {
			log.Printf("terminal shutdown: %v", err)
		}
	}
	finished := make(chan struct{})
	go func() { startup.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-shutdown.Done():
		log.Print("CLI startup cleanup exceeded shared shutdown budget")
	}
}

func baseline(ctx context.Context, binary string) error {
	run := func(args ...string) ([]byte, error) {
		timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		out, err := exec.CommandContext(timeout, binary, args...).Output()
		if err != nil {
			return nil, fmt.Errorf("Herdr CLI %v: %w", args, err)
		}
		return out, nil
	}
	version, err := run("--version")
	if err != nil {
		return err
	}
	schema, err := run("api", "schema", "--json")
	if err != nil {
		return err
	}
	if !json.Valid(schema) {
		return fmt.Errorf("Herdr CLI returned an invalid API schema")
	}
	log.Printf("Herdr CLI %s schema_sha256=%x", version, sha256.Sum256(schema))
	return nil
}

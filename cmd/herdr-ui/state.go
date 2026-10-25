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
	// Notification delivery starts from Herdr's own [ui.toast], as the
	// terminal client would deliver them, until chosen in Settings.
	settings.InheritToast(prefs.HerdrToast(configPath()))
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
	var settingsWin *mygo.Window
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
		notifier = newNotifier(func(n app.Notice) { post(func() { screen.OpenNotice(n) }) })
		persisted := settings.Settings()
		notifier.SetEnabled(persisted.Toast.Delivery != "off")
		// pumpNotifications delivers Herdr's notices outside frames too, so
		// system notifications reach a hidden window's user on time.
		var pumpTimer *time.Timer
		var pumpNotifications func()
		pumpNotifications = func() {
			if pumpTimer != nil {
				pumpTimer.Stop()
				pumpTimer = nil
			}
			if wake := screen.PumpNotifications(); wake > 0 {
				pumpTimer = time.AfterFunc(wake, func() { post(pumpNotifications) })
			}
		}
		font := terminal.Font{Family: persisted.FontFamily, Size: persisted.FontSize, Weight: persisted.FontWeight, LineHeight: persisted.LineHeight}
		copyOnSelect := terminal.CopyOnSelectNone
		if persisted.CopyOnSelect {
			copyOnSelect = terminal.CopyOnSelectClipboard
		}
		sidebar = sidebarinfo.New(target, func(p sidebarinfo.Publication) { post(func() { a.UpdateSidebar(p) }) })
		host = terminalhost.New(terminalhost.Config{Binary: resolvedBinary, Target: target, Env: os.Environ(), Font: font, CopyOnSelect: copyOnSelect, MinimumContrast: persisted.MinimumContrast, OptionAsAlt: persisted.OptionAsAlt, Cursor: cursorStyle(persisted.Cursor), NoBlink: !persisted.CursorBlink, Post: post, Changed: func() { post(func() {}) }, OnScroll: screen.Scroll, OnKey: screen.Key, OnPaste: screen.Paste})
		screen.ConnectTerminal(host, s, ctx, post)
		screen.SetInitialSettings(view.SettingsSnapshot{Theme: persisted.WindowTheme, Font: font, CopyOnSelect: persisted.CopyOnSelect, MinimumContrast: persisted.MinimumContrast, OptionAsAlt: persisted.OptionAsAlt, Cursor: persisted.Cursor, CursorBlink: persisted.CursorBlink, Toast: view.ToastSettings{Delivery: persisted.Toast.Delivery, Position: persisted.Toast.Position, DelaySeconds: persisted.Toast.DelaySeconds}})
		preferences := mygo.Theme.Preferences()
		screen.SetReducedTransparency(preferences.ReduceTransparency)
		material := mygo.VibrancySidebar
		if preferences.ReduceTransparency || preferences.HighContrast {
			material = mygo.VibrancyNone
		}
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Herdr", Width: 1200, Height: 800, MinWidth: 820, MinHeight: 560, TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 44, Vibrancy: material, Content: native.View(screen.View)})
		persist := func(fn func(*prefs.Settings)) {
			go func() {
				if _, err := settings.Update(fn); err != nil {
					log.Printf("prefs: %v", err)
				}
			}()
		}
		openSettings := func() {
			if settingsWin != nil {
				settingsWin.Focus()
				return
			}
			settingsWin = mygo.NewWindow(mygo.WindowOptions{Title: "Settings", Width: 720, Height: 520, MinWidth: 560, MinHeight: 420, StateKey: "settings", Content: native.View(screen.SettingsView)})
			settingsWin.OnClosed(func() { settingsWin = nil })
		}
		keyboard := &keyboardEndpoint{}
		hooks := keyboardHooks(ctx, s, keyboard, screen, post, func() { win.Close() }, resolvedBinary, target)
		hooks.Settings = func() error { openSettings(); return nil }
		hooks.EditConfig = func() error { return editHerdrConfig(resolvedBinary, target, screen) }
		hooks.Font = func(f terminal.Font) {
			host.SetFont(f)
			persist(func(v *prefs.Settings) {
				v.FontFamily, v.FontSize, v.FontWeight, v.LineHeight = f.Family, f.Size, f.Weight, f.LineHeight
			})
		}
		hooks.Theme = func(source string) {
			mygo.Theme.SetSource(mygo.ThemeSource(source))
			persist(func(v *prefs.Settings) { v.WindowTheme = source })
		}
		hooks.CopyOnSelect = func(enabled bool) {
			behavior := terminal.CopyOnSelectNone
			if enabled {
				behavior = terminal.CopyOnSelectClipboard
			}
			host.SetCopyOnSelect(behavior)
			persist(func(v *prefs.Settings) { v.CopyOnSelect = enabled })
		}
		hooks.MinimumContrast = func(ratio float32) {
			host.SetMinimumContrast(ratio)
			persist(func(v *prefs.Settings) { v.MinimumContrast = ratio })
		}
		hooks.OptionAsAlt = func(on bool) {
			host.SetOptionAsAlt(on)
			persist(func(v *prefs.Settings) { v.OptionAsAlt = on })
		}
		hooks.Cursor = func(style string, blink bool) {
			host.SetCursor(cursorStyle(style), !blink)
			persist(func(v *prefs.Settings) { v.Cursor, v.CursorBlink = style, blink })
		}
		hooks.Notifications = func(t view.ToastSettings) {
			notifier.SetEnabled(t.Delivery != "off")
			persist(func(v *prefs.Settings) {
				v.Toast = prefs.Toast{Delivery: t.Delivery, Position: t.Position, DelaySeconds: t.DelaySeconds}
			})
			// The main window's toast and queue follow the change now.
			post(pumpNotifications)
		}
		hooks.PostNotification = notifier.Post
		screen.SetActionHooks(hooks)
		go keyboard.run(ctx, target.Socket, func() { post(pumpNotifications) })
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
		menuItems := map[string]*mygo.MenuItem{}
		menuEnabled := map[string]bool{}
		menuLabels := map[string]string{}
		// Window-level commands keep their direct key equivalents, as Herdr
		// and Kero do; the prefix chords remain Herdr's own layer beneath,
		// shown after each title.
		accelerators := view.DirectKeys
		commandItem := func(id string) *mygo.MenuItem {
			label := screen.MenuLabel(id)
			click := func(_ *mygo.MenuItem, _ *mygo.Window) { post(func() { screen.Dispatch(id) }) }
			if id == "close_pane" {
				// ⌘W closes the window in front when that is not the
				// terminal window, as Settings; it never reaches past it.
				click = func(_ *mygo.MenuItem, front *mygo.Window) {
					if front != nil && front != win {
						front.Close()
						return
					}
					post(screen.ClosePaneKey)
				}
			}
			item := &mygo.MenuItem{Label: label, Accelerator: accelerators[id], Disabled: !screen.Can(id), Click: click}
			menuItems[id] = item
			menuEnabled[id] = !item.Disabled
			menuLabels[id] = label
			return item
		}
		sections := map[string][]*mygo.MenuItem{}
		for _, section := range view.MenuBar {
			var items []*mygo.MenuItem
			for i, group := range section.Groups {
				if i > 0 {
					items = append(items, &mygo.MenuItem{Type: mygo.MenuItemSeparator})
				}
				for _, id := range group {
					items = append(items, commandItem(id))
				}
			}
			sections[section.Title] = items
		}
		var tabNumbers []*mygo.MenuItem
		for n := 1; n <= 9; n++ {
			label := fmt.Sprintf("Tab %d", n)
			if n == 9 {
				label = "Last Tab"
			}
			tabNumbers = append(tabNumbers, &mygo.MenuItem{Label: label, Accelerator: fmt.Sprintf("CmdOrCtrl+%d", n), Click: func(_ *mygo.MenuItem, _ *mygo.Window) { post(func() { screen.SelectTabNumber(n) }) }})
		}
		sections["Tab"] = append(sections["Tab"], &mygo.MenuItem{Type: mygo.MenuItemSeparator}, &mygo.MenuItem{Label: "Select Tab", Submenu: tabNumbers})
		helpItems := []*mygo.MenuItem{commandItem("help")}
		appItems := []*mygo.MenuItem{{Role: mygo.RoleAbout}, {Type: mygo.MenuItemSeparator}, commandItem("settings"), {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleServices}, {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleHide}, {Role: mygo.RoleHideOthers}, {Role: mygo.RoleUnhide}, {Type: mygo.MenuItemSeparator}, {Role: mygo.RoleQuit}}
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
		sections["View"] = append(sections["View"], &mygo.MenuItem{Type: mygo.MenuItemSeparator}, &mygo.MenuItem{Label: "Appearance", Submenu: appearanceItems})
		screen.SetActionStateChanged(func(id string, enabled bool) {
			item, ok := menuItems[id]
			if !ok {
				return
			}
			label := screen.MenuLabel(id)
			if menuLabels[id] != label {
				menuLabels[id] = label
				item.SetLabel(label)
			}
			if menuEnabled[id] != enabled {
				menuEnabled[id] = enabled
				item.SetEnabled(enabled)
			}
		})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "Herdr", Submenu: appItems}, {Role: mygo.RoleEditMenu}, {Label: "Workspace", Submenu: sections["Workspace"]}, {Label: "Tab", Submenu: sections["Tab"]}, {Label: "Pane", Submenu: sections["Pane"]}, {Label: "Go", Submenu: sections["Go"]}, {Label: "View", Submenu: sections["View"]}, {Role: mygo.RoleWindowMenu}, {Label: "Help", Submenu: helpItems}}))
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

// cursorStyle is the plugin's cursor for a persisted shape name.
func cursorStyle(name string) terminal.CursorStyle {
	switch name {
	case "bar":
		return terminal.CursorBar
	case "underline":
		return terminal.CursorUnderline
	}
	return terminal.CursorBlock
}

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

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/input"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/bytedance/herdr-ui/internal/terminalhost"
	view "github.com/bytedance/herdr-ui/internal/ui"
	"github.com/egoist/mygo"
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
		host = terminalhost.New(terminalhost.Config{Binary: resolvedBinary, Target: target, Env: os.Environ(), Post: post, Changed: func() { post(func() {}) }, OnScroll: screen.Scroll, OnKey: screen.Key})
		screen.ConnectTerminal(host, s, ctx, post)
		win = mygo.NewWindow(mygo.WindowOptions{Title: "Herdr", Width: 1200, Height: 800, MinWidth: 820, MinHeight: 560, TitleBarStyle: mygo.TitleBarHiddenInset, TitleBarHeight: 44, Content: native.View(screen.View)})
		win.OnBlur(screen.BlurInput)
		viewItems := []*mygo.MenuItem{}
		terminalItems := []*mygo.MenuItem{}
		helpItems := []*mygo.MenuItem{}
		menuItems := map[string]*mygo.MenuItem{}
		menuEnabled := map[string]bool{}
		for _, action := range view.Actions {
			action := action
			label := action.Label
			if binding := screen.BindingLabel(action.ID); binding != "" {
				label += " · " + binding
			}
			item := &mygo.MenuItem{Label: label, Disabled: !screen.Can(action.ID), Click: func(_ *mygo.MenuItem, _ *mygo.Window) { post(func() { screen.Dispatch(action.ID) }) }}
			switch action.ID {
			case "toggle_sidebar", "previous_tab", "next_tab":
				viewItems = append(viewItems, item)
			case "help":
				helpItems = append(helpItems, item)
			default:
				terminalItems = append(terminalItems, item)
			}
			menuItems[action.ID] = item
			menuEnabled[action.ID] = !item.Disabled
		}
		screen.SetActionStateChanged(func(id string, enabled bool) {
			if menuEnabled[id] != enabled {
				menuEnabled[id] = enabled
				menuItems[id].SetEnabled(enabled)
			}
		})
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}, {Label: "View", Submenu: viewItems}, {Label: "Terminal", Submenu: terminalItems}, {Label: "Help", Submenu: helpItems}, {Role: mygo.RoleWindowMenu}}))
		win.OnClosed(func() { screen.DetachTerminal(); closed.Store(true); cancel() })
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

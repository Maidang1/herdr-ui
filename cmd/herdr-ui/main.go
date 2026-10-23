// Herdr UI's M0 native window. This is an explicit terminal capability harness,
// not the M1 session browser. No workspace/pane mutation is issued here.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func main() {
	socket := flag.String("socket", "", "Herdr API socket override")
	binary := flag.String("herdr", "herdr", "Herdr executable")
	terminalID := flag.String("terminal", "", "existing terminal ID (never a pane ID)")
	local := flag.Bool("local-fixture", false, "run /bin/cat for local IME/copy/paste verification; does not connect to Herdr")
	closeAfter := flag.Duration("close-after", 0, "close the test window after a duration")
	flag.Parse()
	if !*local && *terminalID == "" {
		fmt.Fprintln(os.Stderr, "M0: use -terminal <terminal_id> or -local-fixture; use cmd/m0-probe to inspect IDs")
		os.Exit(2)
	}
	var command, env []string
	if *local {
		command = []string{"/bin/cat"}
		env = os.Environ()
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		target, err := herdr.ResolveTarget(*socket, home, os.Getenv)
		if err != nil {
			log.Fatal(err)
		}
		path, err := exec.LookPath(*binary)
		if err != nil {
			log.Fatal(err)
		}
		command, env, err = target.Attach(path, *terminalID, os.Environ())
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("M0 attach socket=%s terminal=%s (no takeover)", target.Socket, *terminalID)
	}
	var closed atomic.Bool
	var cleanup sync.WaitGroup
	var ownerMu sync.Mutex
	var owned *terminal.Terminal
	mygo.App.WhenReady(func() {
		var term *terminal.Terminal
		var lastCols, lastRows int
		status := "Loading terminal library…"
		win := mygo.NewWindow(mygo.WindowOptions{
			Title: "Herdr UI · M0", Width: 1000, Height: 680, MinWidth: 820, MinHeight: 560,
			Content: ui.View(func(c *ui.Context) {
				ui.Column(c).Fill().Children(func() {
					ui.Text(c, "M0 · "+status).Height(36)
					if term != nil {
						terminal.View(c, term).Fill().AutoFocus()
						cols, rows := term.Size()
						if cols != lastCols || rows != lastRows {
							lastCols, lastRows = cols, rows
							log.Printf("M0 view grid=%dx%d", cols, rows)
						}
					}
				})
			}),
		})
		win.OnClosed(func() {
			closed.Store(true)
			term = nil
			ownerMu.Lock()
			old := owned
			owned = nil
			ownerMu.Unlock()
			if old != nil {
				cleanup.Add(1)
				go func() {
					defer cleanup.Done()
					if err := old.Close(); err != nil {
						log.Printf("M0 close: %v", err)
					}
					log.Print("M0 terminal closed")
				}()
			}
		})
		cleanup.Add(1)
		go func() {
			defer cleanup.Done()
			// Load/New never block the UI. Install only if the window still exists.
			created, err := terminal.New(terminal.Options{
				Command: command, Env: env, Padding: terminal.Padding{X: 16, Y: 12},
				OnExit: func(code int) {
					if closed.Load() {
						return
					}
					win.Update(func() {
						if closed.Load() {
							return
						}
						status = fmt.Sprintf("attach exited (%d); remote session is not closed", code)
						log.Printf("M0 OnExit code=%d", code)
					})
				},
				OnClear: func(terminal.Clear) bool {
					if *local {
						return false
					}
					status = "Clear unavailable for a mirrored Herdr screen"
					return true
				},
				// M0 observes hook calls without implementing M3's business prefix.
				// A single Ctrl-B must never leak into direct attach's escape state.
				OnKey: func(mods ui.Modifiers, key ui.Key) bool {
					if !*local && mods == ui.Ctrl && key == ui.KeyB {
						if term != nil {
							term.Send([]byte{2, 2})
						}
						status = "Literal Ctrl-B sent through attach escape"
						return true
					}
					return false
				},
				OnScroll: func(rows int) bool {
					status = fmt.Sprintf("OnScroll rows=%d (server scroll integration is M2)", rows)
					return !*local
				},
			})
			ownerMu.Lock()
			if closed.Load() {
				ownerMu.Unlock()
				if created != nil {
					created.Close()
				}
				return
			}
			owned = created
			ownerMu.Unlock()
			win.Update(func() {
				if closed.Load() {
					return
				}
				if err != nil {
					status = "Terminal unavailable: " + err.Error()
					log.Print(status)
					return
				}
				term = created
				status = "Terminal ready · IME / resize / copy / paste / Ctrl-B / clear / wheel"
				log.Print("M0 terminal installed")
			})
		}()
		if *closeAfter > 0 {
			time.AfterFunc(*closeAfter, func() {
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
		log.Fatal(err)
	}
	done := make(chan struct{})
	go func() { cleanup.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Print("M0 cleanup exceeded shared 5s budget")
	}
}

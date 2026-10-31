// Hergo shows the selected session pane through the M2 terminal host.
// Explicit terminal/local-fixture flags retain the M0 capability harness.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/prefs"
	"github.com/Maidang1/herdr-ui/internal/shellenv"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	updatenative "github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
)

func main() {
	socket := flag.String("socket", "", "Herdr API socket override")
	binary := flag.String("herdr", "herdr", "Herdr executable")
	terminalID := flag.String("terminal", "", "existing terminal ID (never a pane ID)")
	local := flag.Bool("local-fixture", false, "run /bin/cat for local IME/copy/paste verification; does not connect to Herdr")
	closeAfter := flag.Duration("close-after", 0, "close the test window after a duration")
	plainMode := flag.Bool("plain", false, "open as a plain terminal without connecting to Herdr")
	connect := flag.Bool("connect", false, "connect to Herdr this time even when the plain terminal is the startup choice")
	nativeSurface := flag.Bool("native-surface", runtime.GOOS == "windows", "render Herdr endpoint cells directly (always enabled on Windows)")
	flag.Parse()
	// Opened from Finder or the Dock, the app has launchd's PATH, without
	// the directory herdr is installed in.
	if err := shellenv.Apply(context.Background(), *binary); err != nil {
		log.Printf("PATH: %v", err)
	}
	if !*local && *terminalID == "" {
		mygo.Use(updatenative.Plugin)
		settings, err := prefs.Open("")
		if err != nil || settings == nil {
			log.Printf("prefs: %v", err)
			settings = prefs.Memory()
		}
		install := newInstallationWatch(*binary, herdr.Detect(context.Background(), *binary))
		if found := install.Get(); found.Path == "" {
			log.Printf("Herdr: not installed (%s)", *binary)
		} else {
			log.Printf("Herdr: %s %s %s", found.Path, found.Version, found.Error)
		}
		if startupPlain(settings.Settings().StartupMode, *plainMode, *connect, install.Get()) {
			runPlainApp(settings, install)
			return
		}
		runStateWindow(*socket, *binary, *closeAfter, *nativeSurface, settings, install)
		return
	}
	var command, env []string
	if *local {
		command = []string{"/bin/cat"}
		if runtime.GOOS == "windows" {
			command = []string{"cmd.exe", "/Q"}
		}
		env = os.Environ()
	} else {
		if runtime.GOOS == "windows" {
			log.Fatal("-terminal is the legacy Unix attach harness; open the normal native-surface window on Windows")
		}
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
			Title: "Hergo · M0", Width: 1000, Height: 680, MinWidth: 820, MinHeight: 560,
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
					<-old.Done()
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
					<-created.Done()
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

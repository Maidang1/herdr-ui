// Package ui draws the Herdr session in a native window.
//
// The window is a client of the Herdr daemon: it shows the workspaces in a
// sidebar, the tabs of the focused workspace in a tab bar, and the panes of
// the focused tab as the terminal cells the daemon painted. Browser pages
// live in tabs beside the terminal tabs.
package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"herdr-ui/internal/config"
	"herdr-ui/internal/control"
	"herdr-ui/internal/herdr"
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Options are the settings of the window.
type Options struct {
	// SocketPath is the daemon's client socket. Empty means the socket the
	// environment points at.
	SocketPath string
	// ControlSocket is where the control server listens. Empty means the
	// socket under the state directory.
	ControlSocket string
	// CellWidthPx and CellHeightPx are the size of one character cell.
	CellWidthPx  uint32
	CellHeightPx uint32
	// Cols and Rows are how many cells the window shows.
	Cols uint16
	Rows uint16
	// SSHTarget is the host to reach over SSH, as user@host. Empty means
	// the daemon of this machine.
	SSHTarget string
	// SSHSession is the session of that host.
	SSHSession string
}

// App is the running window.
type App struct {
	opts     Options
	endpoint *herdr.Endpoint
	win      *mygo.Window
	// keys is where the keys of the user go. It is the endpoint when the
	// window is open, and something else in a test.
	keys paneInput

	mu           sync.Mutex
	snapshot     *herdr.Snapshot
	surface      *herdr.SurfaceFrame
	workspace    string
	tab          string
	pane         string
	pages        []*Page
	page         string
	input        string
	status       string
	statusAt     time.Time
	palette      bool
	query        string
	dismissed    bool
	cellWidth    float32
	worktrees    []WorktreeInfo
	integrations []IntegrationInfo
	finder       findState
	selection    selectionState
	copyMode     copyModeState
	link         linkState
	updatePanel  bool
	annotate     annotateState
	// fontSize is the size of the characters of the terminal, and
	// rowHeight the height of a row, as the settings say.
	fontSize  float32
	rowHeight float32
	// uiFontSize is the size of the characters of the rest of the window.
	uiFontSize float32
	// themeName is the name of the theme of the terminal, and themeLight
	// and themeDark the themes it stands for on each side of the system.
	themeName  string
	themeLight *theme.Theme
	themeDark  *theme.Theme
	// themeError says what went wrong the last time the theme was read.
	themeError string
	// keybindings are the shortcuts the user set, by the name of the
	// action they belong to.
	keybindings map[string]string
	// layout is the name of the layout of the sidebar.
	layout string
	// notificationsEnabled says the notices of the daemon are shown, and
	// notificationsFinished says the notices that say a task is done.
	notificationsEnabled  bool
	notificationsFinished bool
	// soundVolume is how loud the sounds of the agents are.
	soundVolume int
	// bellAttention bounces the icon of the window when a pane rings, and
	// bellSound plays the sound of the system then.
	bellAttention bool
	bellSound     bool
	// settings are the settings in force, and pending the ones the user
	// changes in the settings panel.
	settings config.Settings
	pending  config.Settings
	// pendingBindings are the shortcuts the user changes in the settings
	// panel, by the name of the action.
	pendingBindings map[string]*string
	// railView is the view the column of views at the left shows.
	railView string
	// load is how busy the machine is.
	load load
	// ports are the ports the machine listens on.
	ports []portReading
	// dropSelection says the selection goes when it was copied, instead
	// of staying highlighted.
	dropSelection bool
	// settingsOpen says the settings are showing.
	settingsOpen bool
	// toasts are the notices the daemon gave.
	toasts []toast
	// suppressText drops the text that follows a key the window already
	// sent to the pane as a chord.
	suppressText bool
	// terminalRect is where the terminal stands in the window.
	terminalRect ui.Rect
	// clipboardWriter puts text on the clipboard. It is the system's
	// clipboard when the window is open.
	clipboardWriter func(string) error
}

// paneInput is where the window sends what the user types.
type paneInput interface {
	// SendText types text into a pane.
	SendText(paneID, text string) error
	// SendKey presses one key in a pane.
	SendKey(paneID string, key herdr.PaneKey) error
}

// Run opens the window and runs until the user closes it.
func Run(opts Options) error {
	app := &App{opts: opts}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	connection := herdr.EndpointOptions{
		SocketPath:    opts.SocketPath,
		CellWidthPx:   opts.CellWidthPx,
		CellHeightPx:  opts.CellHeightPx,
		Cols:          opts.Cols,
		Rows:          opts.Rows,
		SurfaceActive: true,
	}
	var endpoint *herdr.Endpoint
	var err error
	if opts.SSHTarget != "" {
		logf("herdr-ui: connecting to %s over SSH", opts.SSHTarget)
		endpoint, err = herdr.ConnectSSH(ctx, connection, opts.SSHTarget, opts.SSHSession)
	} else {
		logf("herdr-ui: connecting to the daemon")
		endpoint, err = herdr.Connect(ctx, connection)
	}
	if err != nil {
		return err
	}
	logf("herdr-ui: connected, generation %d", endpoint.Welcome().Generation)
	app.endpoint = endpoint
	app.keys = endpoint
	app.loadSettings()
	app.watchOutage(endpoint)
	app.watchNotices(endpoint)
	// The meters of the machine are asked on their own, so that the
	// window keeps them fresh without waiting for the daemon.
	app.watchMeters()
	defer func() {
		// The daemon sends the keys to the client that holds the keyboard.
		// Another client may want it back.
		_ = endpoint.Focus(false)
		_ = endpoint.Close()
	}()

	go app.watch(ctx)

	socket := opts.ControlSocket
	if socket == "" {
		resolved, err := control.DefaultSocketPath()
		if err != nil {
			return err
		}
		socket = resolved
		if opts.SSHTarget != "" {
			// A window of a remote host keeps its own control
			// socket, so that it does not answer for the window of
			// this machine.
			socket = remoteSocketPath(socket, opts.SSHTarget)
		}
	}
	server, err := control.Listen(socket, app)
	if err != nil {
		return fmt.Errorf("herdr-ui: could not listen on %s: %w", socket, err)
	}
	defer server.Close()
	go server.Serve()
	logf("herdr-ui: control socket at %s", socket)

	var ready bool
	var readyMu sync.Mutex
	mygo.App.WhenReady(func() {
		readyMu.Lock()
		ready = true
		readyMu.Unlock()

		app.win = mygo.NewWindow(mygo.WindowOptions{
			Title:    "Herdr",
			Width:    1180,
			Height:   760,
			MinWidth: 720,
			Content:  ui.View(app.view),
		})
		app.win.OnClosed(func() {
			cancel()
		})
		app.win.Invalidate()
		// The daemon sends input to the client that has the keyboard.
		if err := endpoint.Focus(true); err != nil {
			logf("herdr-ui: could not take the keyboard: %v", err)
		}
	})
	if err := mygo.App.Run(); err != nil {
		return err
	}
	readyMu.Lock()
	defer readyMu.Unlock()
	if !ready {
		return fmt.Errorf("herdr-ui: the window never opened")
	}
	return nil
}

// watch follows the daemon and keeps the window in step with it.
func (a *App) watch(ctx context.Context) {
	go func() {
		first := true
		for snapshot := range a.endpoint.Snapshots() {
			a.mu.Lock()
			a.snapshot = snapshot
			if a.workspace == "" && snapshot.FocusedWorkspaceID != nil {
				a.workspace = *snapshot.FocusedWorkspaceID
			}
			if a.tab == "" && snapshot.FocusedTabID != nil {
				a.tab = *snapshot.FocusedTabID
			}
			a.mu.Unlock()
			a.invalidate()
			if first {
				first = false
				// The daemon serves the methods that act on a surface only
				// to a client that holds one.
				a.activateSurface()
				a.refreshPanels()
			}
		}
	}()
	go func() {
		painted := true
		for frame := range a.endpoint.Surfaces() {
			a.mu.Lock()
			a.surface = frame
			a.mu.Unlock()
			if painted {
				painted = false
				logf("herdr-ui: painting %d panes, %d cells", len(frame.Panes), len(frame.Grid.Cells))
			}
			a.invalidate()
		}
	}()
	<-ctx.Done()
}

func (a *App) invalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

// setStatus shows a short message in the status bar.
func (a *App) setStatus(format string, args ...any) {
	a.mu.Lock()
	a.status = fmt.Sprintf(format, args...)
	a.statusAt = time.Now()
	a.mu.Unlock()
	a.invalidate()
}

// view draws the whole window.
func (a *App) view(c *ui.Context) {
	theme := c.Theme()

	a.mu.Lock()
	snapshot := a.snapshot
	tab := a.tab
	pages := append([]*Page(nil), a.pages...)
	activePage := a.page
	palette := a.palette
	query := a.query
	a.mu.Unlock()

	// The window is a rail of views at the left, the sidebar beside it and
	// the terminal in the middle, over the bar of the session.
	colors := a.newChrome(theme.Dark)
	root := ui.Column(c).Fill().Background(colors.background)
	// The shortcuts of the window come from the settings, so that the
	// user may give them the keys they like.
	for _, item := range a.shortcuts() {
		item := item
		root.OnShortcut(item.mods, item.key, item.run)
	}
	root.Children(func() {
		if snapshot == nil {
			ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, "Waiting for the Herdr daemon…").TextColor(colors.muted)
			})
			return
		}

		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
			a.drawRail(c, colors, "")
			a.drawSidebarPanel(c, colors, snapshot, a.workspaceID(snapshot))
			ui.Column(c).Grow(1).MinWidth(0).Background(colors.background).Children(func() {
				a.drawTopBar(c, colors, snapshot)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					a.drawFindBar(c)
					a.drawAnnotationBar(c)
					ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Stretch).Children(func() {
						if activePage != "" {
							a.drawPage(c, pages, activePage)
							return
						}
						a.drawTerminal(c, snapshot)
					})
				})
				a.drawBottomBar(c, colors, snapshot, tab, pages, activePage)
			})
		})
	})

	if palette {
		a.drawPalette(c, snapshot, query)
	}
	a.drawUpdatePanel(c, snapshot)
	a.drawSettings(c)
}

// workspaceID returns the workspace the window shows.
func (a *App) workspaceID(snapshot *herdr.Snapshot) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.workspace != "" {
		return a.workspace
	}
	if snapshot != nil && snapshot.FocusedWorkspaceID != nil {
		return *snapshot.FocusedWorkspaceID
	}
	return ""
}

// focusWorkspace asks the daemon to focus a workspace.
func (a *App) focusWorkspace(id string) {
	a.mu.Lock()
	a.workspace = id
	a.page = ""
	a.mu.Unlock()
	a.request("workspace.focus", map[string]any{"workspace_id": id})
	a.invalidate()
}

// focusTab asks the daemon to focus a tab.
func (a *App) focusTab(id string) {
	a.mu.Lock()
	a.tab = id
	a.page = ""
	a.mu.Unlock()
	a.request("tab.focus", map[string]any{"tab_id": id})
	a.invalidate()
}

// newTab asks the daemon for another tab in the focused workspace.
func (a *App) newTab() {
	workspace := a.workspaceID(a.snapshotOf())
	if workspace == "" {
		a.setStatus("no workspace is focused")
		return
	}
	a.request("tab.create", map[string]any{"workspace_id": workspace})
	a.setStatus("asked for a new tab")
}

// request sends one API request to the daemon and reports what it says.
func (a *App) request(method string, params map[string]any) {
	if a.endpoint == nil {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := a.endpoint.Request(ctx, method, params, nil); err != nil {
			a.setStatus("%s: %v", method, err)
		}
	}()
}

// sendText types text into the pane the user aims at.
func (a *App) sendText(text string) {
	snapshot := a.snapshotOf()
	pane, ok := a.targetPane(snapshot)
	if !ok {
		a.setStatus("no pane is focused")
		return
	}
	if err := a.endpoint.SendText(pane.PaneID, text); err != nil {
		a.setStatus("could not send: %v", err)
		return
	}
	a.setStatus("sent %d characters to %s", len(text), pane.PaneID)
}

// remoteSocketPath says the control socket of a window of a remote host.
//
// The name of the host is put into the name of the socket, with the
// characters a path may not hold replaced.
func remoteSocketPath(socket, target string) string {
	name := strings.NewReplacer("@", "-", ":", "-", "/", "-", "\\", "-").Replace(target)
	if name == "" {
		return socket
	}
	folder := filepath.Dir(socket)
	base := strings.TrimSuffix(filepath.Base(socket), ".sock")
	return filepath.Join(folder, base+"-"+name+".sock")
}

// ControlSocketPath returns where the control server listens.
func ControlSocketPath() (string, error) {
	if path := os.Getenv("HERDR_UI_CONTROL_SOCKET"); path != "" {
		return path, nil
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "herdr", "herdr-ui", "control.sock"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "herdr", "herdr-ui", "control.sock"), nil
}

// agentColor picks the color of an agent status.
func agentColor(theme *ui.Theme, status string) ui.Color {
	switch strings.ToLower(status) {
	case "working":
		return theme.Accent
	case "blocked":
		return theme.Warning
	case "done":
		return theme.Success
	default:
		return theme.TextMuted
	}
}

// logf writes to the log, for the parts of the app that have no window yet.
func logf(format string, args ...any) {
	log.Printf(format, args...)
}

// contextWithTimeout returns a context for one API request.
func contextWithTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

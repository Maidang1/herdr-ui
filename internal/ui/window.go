// Package ui renders App's read-only projection and dispatches local choices.
package ui

import (
	"strings"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

type Window struct {
	app                 *app.App
	hooks               ActionHooks
	commandSurfaceState *commandSurfaceState
	retry               func()
	hidden              bool
	terminal            *terminalPanel
	controller          *actionController
	actionState         func(string, bool)
	projected           app.View
	projectionVersion   uint64
	viewportWidth       float32
	sidebarWidth        float32
	sidebarVisible      bool
	sidebarDragging     bool
	sidebarDragOffset   float32
	helpFocusPending    bool
	appearance          windowAppearance
	projectionValid     bool
	sidebarFilter       string
	sidebarFilterOn     bool
	sidebarFilterFocus  bool
	sidebarMenuOpen     bool
	tabMenuOpen         bool
	agentsGrouped       bool
	fontSize            float32
	fontSizeDefault     float32
}

func New(a *app.App, retry func()) *Window {
	return &Window{app: a, retry: retry, sidebarWidth: 248, viewportWidth: 1200, sidebarVisible: true, agentsGrouped: true, fontSize: 13, fontSizeDefault: 13, appearance: windowAppearance{active: true}}
}

// SetInitialFontSize applies the persisted terminal font size before the
// first terminal attaches, and remembers it as the size Actual Size
// returns to.
func (w *Window) SetInitialFontSize(size float32) {
	w.fontSize = clampFontSize(size)
	w.fontSizeDefault = w.fontSize
}

// SetFontSize pushes a new size to every attached terminal through the
// FontSize hook, which also persists it.
func (w *Window) SetFontSize(size float32) {
	w.fontSize = clampFontSize(size)
	if w.hooks.FontSize != nil {
		w.hooks.FontSize(w.fontSize)
	}
}

func clampFontSize(size float32) float32 { return max(8, min(48, size)) }

// applyFontZoom steps the terminal font size, as Herdr's and Kero's zoom
// commands do; a zero step returns to the persisted default.
func (w *Window) applyFontZoom(step float32) {
	if step == 0 {
		w.SetFontSize(w.fontSizeDefault)
		return
	}
	w.SetFontSize(w.fontSize + step)
}

// jumpToAttention cycles through the agents that need a reply, starting
// after the one currently selected, as Kero's ⌘J does.
func (w *Window) jumpToAttention() {
	v := w.currentView()
	var waiting []app.AgentRow
	for _, row := range v.Agents {
		if strings.EqualFold(row.Status, "blocked") {
			waiting = append(waiting, row)
		}
	}
	if len(waiting) == 0 {
		if w.controller != nil {
			w.controller.message = "No agent is waiting for you."
		}
		return
	}
	start := 0
	for i, row := range waiting {
		if row.Selected {
			start = i + 1
			break
		}
	}
	target := waiting[start%len(waiting)]
	w.app.SelectWorkspace(target.WorkspaceID)
	w.app.SelectPane(target.PaneID)
	w.BlurInput()
}

func (w *Window) currentView() app.View {
	if !w.projectionValid || w.projectionVersion != w.app.Version() {
		w.projected = w.app.View()
		w.projectionVersion = w.app.Version()
		w.projectionValid = true
	}
	return w.projected
}

func (w *Window) View(c *native.Context) {
	w.syncInput()
	width, _ := c.Size()
	w.viewportWidth = width
	w.sidebarVisible = chromeLayout(width, w.sidebarWidth, w.hidden, w.referenceOpen()).sidebarVisible
	if w.actionState != nil {
		for _, action := range Actions {
			w.actionState(action.ID, w.Can(action.ID))
		}
	}
	p := w.appearancePalette(c)
	v := w.currentView()
	root := native.Row(c).Fill().Padding(8).Gap(8).AlignItems(native.Stretch).TextColor(p.Text).FontSize(13)
	if v.PaneID == "" && !w.modalOpen() && !w.referenceOpen() && !root.FocusWithin() {
		root.Focus()
	}
	w.styleShell(c, root, p)
	root.HandleInput(func(ev native.InputEvent) bool {
		if ev.Kind != native.InputKeyDown || w.controller == nil || w.controller.bindingError != "" || w.modalOpen() || w.sidebarFilterOn || w.currentView().PaneID != "" {
			return false
		}
		label, ok := input.NativeKey(ev.Mods, ev.Key)
		if !ok {
			return false
		}
		r := w.controller.machine.Handle(label, false)
		if r.Action != "" {
			w.DispatchIndex(r.Action, r.Index)
		}
		return r.Consumed
	})
	root.Children(func() {
		if w.sidebarVisible {
			w.sidebar(c, v, p)
		}
		main := native.Column(c.Key("main-area")).Grow(1).MinWidth(0).AlignItems(native.Stretch).Gap(8)
		main.Children(func() {
			if len(v.Tabs) > 0 && w.sidebarVisible {
				w.tabBar(c, v, p)
			}
			content := native.Column(c.Key("terminal-content")).Label("Terminal content").Grow(1).MinWidth(0).AlignItems(native.Stretch)
			if w.terminal != nil && v.HasLayout && len(v.Layout.Panes) > 1 {
				content.Background(native.Transparent)
			} else {
				content.Radius(16).Background(p.Content).Border(1, p.Border).Shadow(0, 1, 5, 0, p.ContentShadow).Clip()
			}
			content.Children(func() {
				w.windowControls(c, v, p)
				w.actionBar(c, v, p)
				w.shortcutPanel(c, p)
				if v.Stale && v.Error != "" {
					native.Row(c).Padding(10, 16).Gap(12).AlignItems(native.Center).Children(func() {
						native.Icon(c, icons.Unplug).Size(16, 16).TextColor(p.Danger)
						native.Text(c, v.Error).TextColor(p.Danger).Grow(1)
						if native.Button(c, "Retry").Label("Retry connection").Clicked() && w.retry != nil {
							w.retry()
						}
					})
					if !v.Updated.IsZero() {
						native.Text(c, "Last synced "+v.Updated.Format("15:04:05")).FontSize(12).TextColor(p.Secondary).Padding(0, 16, 8, 16)
					}
				}
				if w.commandSurfaceState != nil {
					w.commandSurface(c, p)
				} else {
					w.content(c, v, p)
				}
			})
		})
		if w.referenceOpen() {
			w.referencePanel(c, p)
		}
	})
}

// Reserve native traffic lights only when the sidebar no longer holds them.
// The strip stays at 36pt so its controls' centers land on the lights' own
// line (measured center y=26), and carries the tab strip inline — as the
// reference apps do — so no empty bar sits between the lights and content.
func (w *Window) windowControls(c *native.Context, v app.View, p design.Palette) {
	if w.sidebarVisible {
		return
	}
	bar := c.TitleBar()
	native.Row(c).Height(36).Shrink(0).FillWidth().Padding(0, 4, 0, 0).AlignItems(native.Center).DragWindow().Children(func() {
		native.Box(c).Width(bar.Left).Shrink(0)
		if w.iconButton(c, "show-sidebar", icons.PanelLeft, "Show sidebar", p).Clicked() {
			w.Dispatch("toggle_sidebar")
		}
		native.Box(c).Width(4).Shrink(0)
		if len(v.Tabs) > 0 {
			w.tabStripControls(c, v, p, true)
		} else {
			native.Box(c).Grow(1)
		}
	})
}

func (w *Window) content(c *native.Context, v app.View, p design.Palette) {
	if len(v.Rows) == 0 {
		if w.terminal != nil {
			w.DetachTerminal()
		}
		title, detail := "Connecting to Herdr…", "Waiting for the local session."
		if v.State == session.Ready {
			title = "No workspaces"
			detail = "Press the prefix then Shift+N to create a workspace."
		} else if v.State == session.Incompatible {
			title = "This Herdr session is unavailable"
			detail = "Check the connection details above and retry."
		} else if v.Error != "" {
			title = "Connection interrupted"
			detail = "Your session will appear when the connection is restored."
		}
		native.Column(c).Grow(1).FillWidth().Center().Gap(8).Padding(24).Children(func() { native.Text(c, title).FontSize(16).Bold(); native.Text(c, detail).TextColor(p.Secondary) })
		return
	}
	if w.terminal != nil {
		w.terminalContent(c, v, p)
		return
	}
	native.Scroll(c).Grow(1).Children(func() {
		native.Column(c).Padding(20, 16).Gap(12).Children(func() {
			if len(v.Panes) == 0 {
				native.Text(c, "No panes in this tab").TextColor(p.Secondary)
			}
			for _, pane := range v.Panes {
				row := native.ButtonBase(c.Key("detail:" + pane.ID)).FillWidth().Radius(10).Padding(14).Label(paneDescription(pane))
				if pane.Selected {
					row.Background(p.Inset)
				} else if row.Hovered() {
					row.Background(p.Hover)
				}
				row.Children(func() {
					native.Row(c).FillWidth().Gap(12).AlignItems(native.Center).Children(func() {
						native.Icon(c, icons.Agent(pane.Agent)).Size(18, 18).TextColor(p.Secondary).Tooltip(pane.Agent)
						native.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
							native.Text(c, pane.Title).FontSize(13).SingleLine()
							if pane.Agent != "" {
								native.Text(c, pane.Agent).FontSize(12).TextColor(p.Secondary)
							}
							if pane.CWD != "" {
								native.Text(c, pane.CWD).FontSize(12).TextColor(p.Secondary).Selectable().SingleLine().Tooltip(pane.CWD)
							}
						})
						w.stateIcon(c, pane.Title, pane.Status)
					})
				})
				if row.Clicked() {
					w.app.SelectPane(pane.ID)
					w.BlurInput()
				}
			}
		})
	})
}

func (w *Window) stateIcon(c *native.Context, name, value string) {
	icon, color, label := status(value, c.Theme().Dark)
	native.Icon(c, icon).Size(14, 14).TextColor(color).Tooltip(label).Label(name + " · " + label)
}
func statusLabel(value string) string {
	switch strings.ToLower(value) {
	case "working":
		return "Working"
	case "idle":
		return "Idle"
	case "blocked":
		return "Waiting for reply"
	case "done":
		return "Done"
	case "", "unknown":
		return "Unknown"
	default:
		return "Unknown (" + value + ")"
	}
}
func status(value string, dark bool) (*native.SVG, native.Color, string) {
	p := design.Colors(dark)
	label := statusLabel(value)
	switch strings.ToLower(value) {
	case "working":
		return icons.Activity, p.Working, label
	case "idle":
		return icons.Circle, p.Idle, label
	case "blocked":
		return icons.MessageCircle, p.Warning, label
	case "done":
		return icons.Check, p.Success, label
	default:
		return icons.CircleHelp, p.Idle, label
	}
}
func connection(state session.State, dark bool) (*native.SVG, native.Color, string) {
	p := design.Colors(dark)
	switch state {
	case session.Ready:
		return icons.CircleCheck, p.Success, "Local connected"
	case session.Incompatible, session.Closed, session.Disconnected:
		return icons.Unplug, p.Danger, "Local " + strings.ToLower(string(state))
	default:
		return icons.LoaderCircle, p.Warning, "Local " + strings.ToLower(string(state))
	}
}

func paneDescription(pane app.PaneRow) string {
	label := pane.Title
	if pane.Agent != "" {
		label += " · " + pane.Agent
	}
	return label + " · " + statusLabel(pane.Status)
}

func (w *Window) SelectionRevision() uint64 { return w.app.ViewRevision() }

// WindowActive reports whether the window currently holds the system
// focus, for callers that stay quiet while the user is watching.
func (w *Window) WindowActive() bool { return w.appearance.active }

// JumpTo selects a workspace and the pane inside it, as a notification
// click does.
func (w *Window) JumpTo(workspaceID, paneID string) {
	w.app.SelectWorkspace(workspaceID)
	w.app.SelectPane(paneID)
	w.BlurInput()
}

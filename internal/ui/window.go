// Package ui renders App's read-only projection and dispatches local choices.
package ui

import (
	"fmt"
	"strings"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	"github.com/bytedance/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

type Window struct {
	app               *app.App
	retry             func()
	hidden            bool
	terminal          *terminalPanel
	controller        *actionController
	actionState       func(string, bool)
	projected         app.View
	projectionVersion uint64
	viewportWidth     float32
	sidebarWidth      float32
	sidebarVisible    bool
	sidebarDragging   bool
	sidebarDragOffset float32
	helpFocusPending  bool
	appearance        windowAppearance
	projectionValid   bool
}

func New(a *app.App, retry func()) *Window {
	return &Window{app: a, retry: retry, sidebarWidth: 248, viewportWidth: 1200, sidebarVisible: true, appearance: windowAppearance{active: true}}
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
	w.styleShell(c, root, p)
	root.Children(func() {
		if w.sidebarVisible {
			w.sidebar(c, v, p)
		}
		native.Column(c.Key("terminal-content")).Label("Terminal content").Grow(1).MinWidth(0).AlignItems(native.Stretch).Radius(16).Background(p.Content).Border(1, p.Border).Shadow(0, 1, 5, 0, p.ContentShadow).Clip().Children(func() {
			w.header(c, v, p)
			w.actionBar(c, v, p)
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
			w.content(c, v, p)
		})
		if w.referenceOpen() {
			w.referencePanel(c, p)
		}
	})
}

func (w *Window) header(c *native.Context, v app.View, p design.Palette) {
	width, _ := c.Size()
	compact := width < 1000 && (w.referenceOpen() || !w.sidebarVisible)
	native.Row(c).Height(40).Shrink(0).PaddingX(12).Gap(8).AlignItems(native.Center).DragWindow().Children(func() {
		if !w.sidebarVisible {
			native.Box(c).Width(c.TitleBar().Left).Shrink(0)
			toggle := w.iconButton(c, "show-sidebar", icons.PanelLeft, "Show sidebar", p)
			if toggle.Clicked() {
				w.Dispatch("toggle_sidebar")
			}
		}
		title := v.WorkspaceName
		if title == "" {
			title = "Herdr"
		}
		titleWidth := float32(180)
		if compact {
			titleWidth = 110
		}
		native.Text(c, title).FontSize(13).Bold().SingleLine().MaxWidth(titleWidth).MinWidth(0).Shrink(1).Tooltip(title)
		if len(v.Tabs) > 0 {
			chosen := ""
			for _, tab := range v.Tabs {
				if tab.Selected {
					chosen = fmt.Sprintf("%d · %s", tab.Number, tab.Name)
				}
			}
			tabWidth := float32(144)
			if compact {
				tabWidth = 112
			}
			tabs := native.ButtonBase(c.Key("tabs")).Width(tabWidth).MinWidth(64).Height(28).Radius(6).PaddingX(8).Label("Current tab").Tooltip(chosen)
			w.controlState(tabs, p)
			tabs.Children(func() {
				native.Row(c).Fill().Gap(5).AlignItems(native.Center).Children(func() {
					native.Text(c, chosen).SingleLine().Grow(1).MinWidth(0)
					native.Icon(c, icons.ChevronDown).Size(12, 12).TextColor(p.Secondary)
				})
			})
			tabs.Menu(func(m *native.Menu) {
				for _, tab := range v.Tabs {
					label := fmt.Sprintf("%d · %s · %d panes", tab.Number, tab.Name, tab.PaneCount)
					if m.Item(label).Checked(tab.Selected).Chosen() {
						w.app.SelectTab(tab.ID)
						w.BlurInput()
					}
				}
			})
		}
		if v.Path != "" && !compact {
			native.Text(c, v.Path).SingleLine().Selectable().TextColor(p.Secondary).Tooltip(v.Path).Grow(1).MinWidth(0)
		} else {
			native.Box(c).Grow(1)
		}
		if w.controller != nil && (w.controller.machine.Awaiting || w.controller.machine.Resize) {
			hint := "Prefix"
			if w.controller.machine.Resize {
				hint = "Resize"
			}
			native.Icon(c, icons.Keyboard).Size(14, 14).TextColor(p.Warning).Label(hint)
			if !compact {
				native.Text(c, hint).FontSize(12).TextColor(p.Warning)
			}
		}
		layout := w.iconButton(c, "layout-menu", icons.Columns2, "Terminal layout", p)
		layout.Menu(func(m *native.Menu) {
			for _, id := range []string{"split_vertical", "split_horizontal", "zoom", "resize_mode"} {
				for _, action := range Actions {
					if action.ID == id {
						label := w.ActionLabel(action.ID)
						if binding := w.BindingLabel(id); binding != "" {
							label += " · " + binding
						}
						if m.Item(label).Disabled(!w.Can(id)).Chosen() {
							w.Dispatch(id)
						}
					}
				}
			}
		})
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
			detail = "Workspaces opened in Herdr will appear here."
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

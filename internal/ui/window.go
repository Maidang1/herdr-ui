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
	expanded          map[string]bool
	terminal          *terminalPanel
	controller        *actionController
	actionState       func(string, bool)
	projected         app.View
	projectionVersion uint64
	projectionValid   bool
}

func New(a *app.App, retry func()) *Window {
	return &Window{app: a, retry: retry, expanded: map[string]bool{}}
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
	if w.actionState != nil {
		for _, action := range Actions {
			w.actionState(action.ID, w.Can(action.ID))
		}
	}
	p := design.Colors(c.Theme().Dark)
	v := w.currentView()
	root := native.Row(c).Fill().Padding(8).Gap(8).AlignItems(native.Stretch).TextColor(p.Text).FontSize(13)
	root.LinearGradient(native.LinearGradient{Angle: 180, Stops: []native.GradientStop{{Color: p.Top, At: 0}, {Color: p.Middle, At: .52}, {Color: p.Bottom, At: 1}}})
	root.Children(func() {
		if !w.hidden {
			w.sidebar(c, v, p)
		}
		native.Column(c).Grow(1).MinWidth(0).AlignItems(native.Stretch).Radius(16).Background(p.Content).Border(1, p.Border).Clip().Children(func() {
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
	})
}

func (w *Window) sidebar(c *native.Context, v app.View, p design.Palette) {
	native.Column(c).Width(248).Shrink(0).AlignItems(native.Stretch).Gap(6).Children(func() {
		native.Row(c).Height(36).Shrink(0).Padding(0, 8).AlignItems(native.Center).DragWindow().Children(func() {
			native.Box(c).Grow(1)
			toggle := native.ButtonBase(c.Key("hide-sidebar")).Size(28, 28).Radius(6).Center().Label("Hide sidebar").Tooltip("Hide sidebar")
			if toggle.Hovered() {
				toggle.Background(p.Hover)
			}
			toggle.Children(func() { native.Icon(c, icons.PanelLeft).Size(16, 16).TextColor(p.Secondary) })
			if toggle.Clicked() {
				w.hidden = true
				w.BlurInput()
			}
		})
		native.Text(c, "Workspaces").FontSize(12).TextColor(p.Secondary).PaddingX(12)
		native.Scroll(c).Grow(1).Label("Workspaces").Children(func() {
			native.Column(c).Gap(0).Children(func() {
				for _, row := range v.Rows {
					w.workspace(c, row, p)
				}
			})
		})
		native.Row(c).Height(28).PaddingX(12).AlignItems(native.Center).Gap(7).Children(func() {
			icon, color, label := connection(v.State, c.Theme().Dark)
			native.Icon(c, icon).Size(14, 14).TextColor(color).Label(label).Tooltip(label)
			native.Text(c, "Local").FontSize(12).TextColor(p.Secondary)
		})
	})
}

func (w *Window) workspace(c *native.Context, row app.WorkspaceRow, p design.Palette) {
	native.Row(c.Key("workspace:"+row.ID)).Padding(0, 0, 0, float32(row.Depth*16)).Height(42).Children(func() {
		button := native.ButtonBase(c.Key("select:"+row.ID)).Grow(1).MinWidth(0).Height(42).Radius(8).Padding(4, 8).Label(row.Name + " · " + statusLabel(row.Status)).Tooltip(row.Path)
		if row.Selected {
			button.Background(p.Selected)
		} else if button.Hovered() {
			button.Background(p.Hover)
		}
		button.Children(func() {
			native.Row(c).FillWidth().Gap(8).AlignItems(native.Center).Children(func() {
				icon := icons.Folder
				if row.Depth > 0 {
					icon = icons.GitBranch
				}
				native.Icon(c, icon).Size(16, 16).TextColor(p.Secondary)
				native.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
					native.Text(c, row.Name).FontSize(13).SingleLine()
					native.Text(c, row.Path).FontSize(12).TextColor(p.Secondary).SingleLine()
				})
				if len(row.Panes) == 0 {
					w.stateIcon(c, row.Name, row.Status)
				}
			})
		})
		if button.Clicked() {
			w.app.SelectWorkspace(row.ID)
			w.BlurInput()
		}
		if len(row.Panes) > 0 {
			toggle := native.ButtonBase(c.Key("expand:"+row.ID)).Size(24, 42).Center().Label("Toggle panes for " + row.Name)
			toggle.Children(func() {
				icon := icons.ChevronRight
				if w.expanded[row.ID] {
					icon = icons.ChevronDown
				}
				native.Icon(c, icon).Size(14, 14).TextColor(p.Secondary)
			})
			if toggle.Clicked() {
				w.expanded[row.ID] = !w.expanded[row.ID]
			}
		}
	})
	if w.expanded[row.ID] {
		for _, pane := range row.Panes {
			button := native.ButtonBase(c.Key("pane:"+pane.ID)).Height(28).FillWidth().Padding(0, 8, 0, float32(32+row.Depth*16)).Radius(6).Label(pane.Title + " · " + statusLabel(pane.Status))
			if pane.Selected {
				button.Background(p.Selected)
			} else if button.Hovered() {
				button.Background(p.Hover)
			}
			button.Children(func() {
				native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
					native.Icon(c, icons.Agent(pane.Agent)).Size(14, 14).TextColor(p.Secondary)
					native.Text(c, pane.Title).SingleLine().FontSize(12).Grow(1).MinWidth(0)
					w.stateIcon(c, pane.Title, pane.Status)
				})
			})
			if button.Clicked() {
				w.app.SelectPane(pane.ID)
				w.BlurInput()
			}
		}
	}
}

func (w *Window) header(c *native.Context, v app.View, p design.Palette) {
	native.Row(c).Height(40).Shrink(0).PaddingX(16).Gap(12).AlignItems(native.Center).DragWindow().Children(func() {
		if w.hidden {
			native.Box(c).Width(c.TitleBar().Left)
			toggle := native.ButtonBase(c.Key("show-sidebar")).Size(28, 28).Center().Label("Show sidebar").Tooltip("Show sidebar")
			toggle.Children(func() { native.Icon(c, icons.PanelLeft).Size(16, 16).TextColor(p.Secondary) })
			if toggle.Clicked() {
				w.hidden = false
				w.BlurInput()
			}
		}
		title := v.WorkspaceName
		if title == "" {
			title = "Herdr"
		}
		native.Text(c, title).FontSize(13).Bold().SingleLine()
		if len(v.Tabs) > 0 {
			labels := make([]string, len(v.Tabs))
			chosen := ""
			for i, tab := range v.Tabs {
				labels[i] = fmt.Sprintf("%d · %s", tab.Number, tab.Name)
				if tab.Selected {
					chosen = labels[i]
				}
			}
			if native.Select(c.Key("tabs"), &chosen, labels).Width(144).Label("Current tab").Changed() {
				for i, label := range labels {
					if label == chosen {
						w.app.SelectTab(v.Tabs[i].ID)
						w.BlurInput()
						break
					}
				}
			}
		}
		if v.Path != "" {
			native.Text(c, v.Path).SingleLine().Selectable().TextColor(p.Secondary).Tooltip(v.Path).Grow(1).MinWidth(0)
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
				row := native.ButtonBase(c.Key("detail:" + pane.ID)).FillWidth().Radius(10).Padding(14).Label(pane.Title + " · " + statusLabel(pane.Status))
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
	switch value {
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
	switch value {
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

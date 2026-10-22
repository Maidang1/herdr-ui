package ui

import (
	"strings"
	"time"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The bar of the session.
//
// It holds the tabs of the focused workspace and the meters of the machine,
// the way the Rust client does: the tabs at the top of the bar, the meters
// below them.

// drawBottomBar shows the tabs of the session and the meters of the machine.
func (a *App) drawBottomBar(c *ui.Context, colors chrome, snapshot *herdr.Snapshot, tab string, pages []*Page, activePage string) {
	ui.Column(c).MinWidth(0).Background(colors.panel).Children(func() {
		a.drawTabStrip(c, colors, snapshot, tab, pages, activePage)
		a.drawMeterRow(c, colors, snapshot)
	})
}

// drawTabStrip shows the tabs of the focused workspace.
func (a *App) drawTabStrip(c *ui.Context, colors chrome, snapshot *herdr.Snapshot, tab string, pages []*Page, activePage string) {
	fontSize := a.uiFontSizePx()
	ui.Row(c).Height(tabBarHeight).Padding(0, 8).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, item := range snapshot.TabsOf(a.workspaceID(snapshot)) {
			item := item
			active := item.TabID == tab && activePage == ""
			label := item.Label
			if label == "" {
				label = itoa(item.Number)
			}
			row := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(4).Children(func() {
				mark := stateMark(item.AgentStatus)
				if mark != nil {
					ui.Icon(c, mark).FontSize(statusWidth).TextColor(colors.stateColor(item.AgentStatus))
				}
				text := ui.Text(c, label).FontSize(fontSize)
				if active {
					text.Bold()
				}
				text.OnClick(func() {
					a.focusTab(item.TabID)
				})
				if close := icon("close"); close != nil {
					ui.Icon(c, close).FontSize(fontSize - 2).TextColor(colors.muted).OnClick(func() {
						a.closeTabByID(item.TabID)
					})
				}
			})
			if active {
				// The tab that is showing wears a quiet face, the way
				// the Rust client draws it.
				row.Background(colors.panel)
			}
		}
		for _, page := range pages {
			page := page
			row := ui.Row(c).Gap(4).AlignItems(ui.Center).Padding(2, 6).Radius(4).Children(func() {
				ui.Text(c, page.Title).FontSize(fontSize).OnClick(func() {
					a.selectPage(page.ID)
				})
				if close := icon("close"); close != nil {
					ui.Icon(c, close).FontSize(fontSize - 2).TextColor(colors.muted).OnClick(func() {
						a.closePage(page.ID)
					})
				}
			})
			if page.ID == activePage {
				row.Background(colors.panel)
			}
		}
		if mark := icon("plus"); mark != nil {
			ui.Icon(c, mark).FontSize(fontSize).TextColor(colors.muted).OnClick(func() {
				a.newTab()
			})
		}
		ui.Row(c).Grow(1)
		for _, segment := range snapshot.TabBarRight {
			color := colors.muted
			if segment.Accent {
				color = colors.accent
			}
			ui.Text(c, segment.Text).FontSize(fontSize - 1).TextColor(color)
		}
	})
}

// drawMeterRow shows the state of the session and of the machine.
func (a *App) drawMeterRow(c *ui.Context, colors chrome, snapshot *herdr.Snapshot) {
	fontSize := a.uiFontSizePx() - 1
	a.mu.Lock()
	status := a.status
	input := a.input
	loading := a.load
	a.mu.Unlock()

	ui.Row(c).Height(meterBarHeight).Padding(0, 10).Gap(10).MinWidth(0).AlignItems(ui.Center).Children(func() {
		// The left of the row says which pane the window is on, and takes
		// the text the user types for it.
		title := ""
		cwd := ""
		agent := ""
		if pane, ok := a.targetPane(snapshot); ok {
			if info, ok := snapshot.AgentOf(pane.PaneID); ok {
				if info.TerminalTitle != nil {
					title = *info.TerminalTitle
				}
				if info.Agent != nil {
					agent = *info.Agent
				}
				agent += " " + info.AgentStatus
			}
			if pane.CWD != nil {
				cwd = *pane.CWD
			}
		}
		if title == "" {
			title = "No pane focused"
		}
		if title != "" {
			ui.Text(c, title).FontSize(fontSize).TextColor(colors.muted).Ellipsis("…").NoWrap()
		}
		if cwd != "" {
			ui.Text(c, cwd).FontSize(fontSize).TextColor(colors.dim).Ellipsis("…").NoWrap()
		}
		if mode, on := a.copyModeStatus(); on {
			ui.Text(c, mode).FontSize(fontSize).TextColor(colors.accent)
		} else if status != "" {
			ui.Text(c, status).FontSize(fontSize).TextColor(colors.muted)
		}
		value := input
		field := ui.TextInput(c, &value)
		field.Height(fontSize + 12)
		field.Placeholder("Type to send to the pane")
		field.Width(200)
		field.OnChange(func() {
			a.mu.Lock()
			a.input = value
			a.mu.Unlock()
		})
		field.OnSubmit(func() {
			a.mu.Lock()
			text := a.input
			a.input = ""
			a.mu.Unlock()
			if text != "" {
				a.sendText(text)
			}
		})

		ui.Row(c).Grow(1)

		// The ports the machine listens on.
		if list := a.portsOfSession(); len(list) > 0 {
			mark := icon("globe")
			if mark != nil {
				ui.Icon(c, mark).FontSize(fontSize).TextColor(colors.muted)
			}
			ui.Text(c, strings.Join(list, " ")).FontSize(fontSize).TextColor(colors.muted)
		}

		// The load of the machine.
		ui.Text(c, "CPU").FontSize(fontSize).TextColor(colors.muted)
		drawMeter(c, colors, loading.CPU, fontSize)
		ui.Text(c, percent(loading.CPU)).FontSize(fontSize).TextColor(colors.muted)
		ui.Text(c, "MEM").FontSize(fontSize).TextColor(colors.muted)
		drawMeter(c, colors, loading.Memory, fontSize)
		ui.Text(c, percent(loading.Memory)).FontSize(fontSize).TextColor(colors.muted)

		// The day it is.
		ui.Text(c, time.Now().Format("2006.01.02")).FontSize(fontSize).TextColor(colors.muted)
	})
}

// drawMeter draws the bar of one meter of the machine.
func drawMeter(c *ui.Context, colors chrome, value float64, fontSize float32) {
	width := float32(44)
	filled := width * float32(value) / 100
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	color := colors.green
	switch {
	case value >= 90:
		color = colors.red
	case value >= 70:
		color = colors.yellow
	}
	ui.Row(c).Width(width).Height(fontSize).Radius(2).Background(colors.dim).Children(func() {
		if filled > 0 {
			ui.Box(c).Width(filled).Height(fontSize).Radius(2).Background(color)
		}
	})
}

// percent writes a number of a meter.
func percent(value float64) string {
	return itoa(int(value+0.5)) + "%"
}

// itoa writes a whole number.
func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var digits []byte
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

// shorten cuts a text short, with a mark that says that it was cut.
func shorten(text string, limit int) string {
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit-1] + "…"
}

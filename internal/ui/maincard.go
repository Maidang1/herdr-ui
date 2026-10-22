package ui

import (
	"herdr-ui/internal/design"
	"herdr-ui/internal/protocol"
	"herdr-ui/internal/ui/widgets"

	"github.com/egoist/mygo/ui"
)

// mainCard draws the card in the middle of the window: the terminal, the
// strip of the tabs, and the row of the meters.
//
// The design file's rule is that the card is the only thing in the window
// with a face of its own: the panel of frosted glass with a hairline around
// it and the roundest corners of the window.
func (a *App) mainCard(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Grow(1).
		Background(t.Panel).
		Radius(design.Radius3XL).
		Border(design.StrokeNormal, t.MainCardBorder()).
		Children(func() {
			a.terminal(c, t, a.live)
			a.tabStrip(c, t)
			a.meterRow(c, t)
		})
}

// tabStrip draws the strip of the tabs.
//
// The design file's rule is that the tab that is chosen has a face of its
// own, and that the strip is a bar of the panel with a hairline above it.
func (a *App) tabStrip(c *ui.Context, t design.Tokens) {
	tabs := a.tabs()
	ui.Column(c).
		Height(design.TabStripHeight).
		Background(t.Panel).
		Children(func() {
			ui.Row(c).Height(design.SidebarDividerHeight).Background(t.Separator)
			ui.Row(c).
				Grow(1).
				Gap(design.SpaceSM).
				Padding(0, design.SpaceLG).
				AlignItems(ui.Center).
				Children(func() {
					for _, tab := range tabs {
						a.tab(c, t, tab)
					}
					// The button that makes a new tab.
					ui.Row(c).
						Size(24, 24).
						Radius(design.RadiusMD).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "plus", design.IconSmall, t.TextSecondary)
						})
					ui.Row(c).Grow(1)
					// The button that splits the panes, and the menu
					// of the tabs.
					ui.Row(c).
						Size(26, 24).
						Radius(design.RadiusMD).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "panel-right", design.IconSmall, t.TextSecondary)
						})
					ui.Row(c).
						Size(26, 24).
						Radius(design.RadiusMD).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "ellipsis", design.IconSmall, t.TextSecondary)
						})
				})
		})
}

// tabs returns the tabs of the workspace in front.
//
// They are the tabs of the daemon when there is one, and the tabs of the
// sample when there is not.
func (a *App) tabs() []Tab {
	snapshot := a.live.Snapshot()
	if snapshot == nil {
		return a.session.Tabs
	}
	var tabs []Tab
	for _, tab := range snapshot.Tabs {
		if snapshot.FocusedWorkspaceID != "" && tab.WorkspaceID != snapshot.FocusedWorkspaceID {
			continue
		}
		dot := ToneNeutral
		switch tab.AgentStatus {
		case protocol.AgentWorking:
			dot = ToneAccent
		case protocol.AgentBlocked:
			dot = ToneDanger
		case protocol.AgentDone:
			dot = ToneSuccess
		}
		tabs = append(tabs, Tab{
			Number: tab.Number,
			Label:  tab.Label,
			Active: tab.Focused,
			Dot:    dot,
		})
	}
	if len(tabs) == 0 {
		return a.session.Tabs
	}
	return tabs
}

// tab draws one tab of the strip of the tabs.
func (a *App) tab(c *ui.Context, t design.Tokens, tab Tab) {
	element := ui.Row(c).
		Height(design.TabHeight).
		Radius(design.RadiusMD).
		Gap(design.SpaceLG).
		Padding(0, design.SpaceLG).
		AlignItems(ui.Center)
	if tab.Active {
		element = element.
			Background(ui.Hex("#FFFFFFB3")).
			Border(design.StrokeNormal, ui.Hex("#FFFFFF8C"))
	}
	labelColor := t.TextSecondary
	labelWeight := design.WeightNormal
	if tab.Active {
		labelColor = t.TextPrimary
		labelWeight = design.WeightMedium
	}
	element.Children(func() {
		dotColor := t.TextTertiary
		switch tab.Dot {
		case ToneSuccess:
			dotColor = t.Green
		case ToneWarning:
			dotColor = t.Yellow
		case ToneDanger:
			dotColor = t.Red
		case ToneAccent:
			dotColor = t.Accent
		}
		ui.Row(c).Size(7, 7).Radius(4).Background(dotColor)
		widgets.Label(c, t, tab.Label, widgets.LabelOptions{
			Size:   design.FontUI,
			Color:  labelColor,
			Weight: labelWeight,
			Mono:   true,
		})
		closeColor := t.TextTertiary
		if tab.Active {
			closeColor = t.TextSecondary
		}
		widgets.Icon(c, t, "x", 11, closeColor)
	})
}

// meterRow draws the row of the meters under the tabs.
//
// The design file's rule is that the row is a bar of the panel with a
// hairline above it, and that the meters themselves say very little: a bar
// of four pixels and a word of ten.
func (a *App) meterRow(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Height(design.MeterRowHeight).
		Background(t.Panel).
		Children(func() {
			ui.Row(c).Height(design.SidebarDividerHeight).Background(t.Separator)
			ui.Row(c).
				Grow(1).
				Gap(design.SpaceSM).
				Padding(0, design.SpaceXL).
				AlignItems(ui.Center).
				Children(func() {
					for _, meter := range a.session.Meters {
						a.meter(c, t, meter)
					}
					ui.Row(c).Grow(1)
					// The ports the session listens on.
					ui.Row(c).
						Gap(design.SpaceMD).
						AlignItems(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "globe", design.IconSmall, t.TextTertiary)
							widgets.Label(c, t, a.session.Ports, widgets.LabelOptions{
								Size:  design.FontMicro,
								Color: t.TextSecondary,
								Mono:  true,
							})
						})
					a.gauge(c, t, "CPU", a.session.CPU)
					a.gauge(c, t, "MEM", a.session.Memory)
					// The time.
					ui.Row(c).
						Gap(design.SpaceMD).
						AlignItems(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "calendar", 12, t.TextTertiary)
							widgets.Label(c, t, a.session.Date, widgets.LabelOptions{
								Size:  design.FontMicro,
								Color: t.TextTertiary,
							})
						})
				})
		})
}

// meter draws one meter of the row of the meters.
func (a *App) meter(c *ui.Context, t design.Tokens, meter Meter) {
	tone := widgets.ToneNeutral
	switch meter.Tone {
	case ToneSuccess:
		tone = widgets.ToneSuccess
	case ToneWarning:
		tone = widgets.ToneWarning
	case ToneDanger:
		tone = widgets.ToneDanger
	case ToneAccent:
		tone = widgets.ToneAccent
	case ToneInfo:
		tone = widgets.ToneInfo
	}
	widgets.Meter(c, t, meter.Icon, meter.Value, meter.Label, tone)
}

// gauge draws the bar of the processor or of the memory.
func (a *App) gauge(c *ui.Context, t design.Tokens, name string, value float32) {
	fill := t.Green
	if value > 0.7 {
		fill = t.Yellow
	}
	ui.Row(c).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			widgets.Label(c, t, name, widgets.LabelOptions{
				Size:   design.FontMicro,
				Color:  t.TextSecondary,
				Weight: design.WeightMedium,
			})
			ui.Row(c).
				Background(ui.Hex("#FFFFFF8C")).
				Radius(design.RadiusXS).
				Size(design.CPUBarWidth, design.CPUBarHeight).
				Children(func() {
					if value <= 0 {
						return
					}
					width := design.CPUBarWidth * value
					if width > design.CPUBarWidth {
						width = design.CPUBarWidth
					}
					ui.Row(c).
						Background(fill).
						Radius(design.RadiusXS).
						Size(width, design.CPUBarHeight)
				})
			widgets.Label(c, t, percent(value), widgets.LabelOptions{
				Size:  design.FontMicro,
				Color: t.TextSecondary,
			})
		})
}

// percent writes a share the way the row of meters writes it.
func percent(value float32) string {
	return itoa(int(value*100)) + "%"
}

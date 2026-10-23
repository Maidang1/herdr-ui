package ui

import (
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	"github.com/bytedance/herdr-ui/internal/layout"
	native "github.com/egoist/mygo/ui"
	"math"
)

const cardHeaderHeight float64 = 32
const cardGap float64 = 8

// Inset internal edges around the unchanged authoritative divider center.
// Outside edges stay flush with the available terminal area.
func cardBounds(r, viewport layout.Rect) layout.Rect {
	left, top, right, bottom := 0.0, 0.0, 0.0, 0.0
	if r.X > viewport.X+1e-6 {
		left = cardGap / 2
	}
	if r.Y > viewport.Y+1e-6 {
		top = cardGap / 2
	}
	if r.X+r.W < viewport.X+viewport.W-1e-6 {
		right = cardGap / 2
	}
	if r.Y+r.H < viewport.Y+viewport.H-1e-6 {
		bottom = cardGap / 2
	}
	return layout.Rect{X: r.X + left, Y: r.Y + top, W: math.Max(0, r.W-left-right), H: math.Max(0, r.H-top-bottom)}
}

// A card is drawable only while its plugin content has positive dimensions.
func hasCardContent(r layout.Rect, multi bool) bool {
	border, header := 0.0, cardHeaderHeight
	if multi {
		border = 2
	}
	return r.W > 32+border && r.H > 24+border+header
}

func (w *Window) paneCard(c *native.Context, r layout.Rect, pane app.PaneRow, frameView app.View, selected, multi bool, p design.Palette, body func()) {
	frame := native.Column(c.Key("pane-frame:"+pane.ID)).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.W), float32(r.H)).Clip().AlignItems(native.Stretch)
	if multi {
		border := p.Border
		if selected && w.appearance.active {
			border = p.Focus.Alpha(.35)
			if c.Preferences().HighContrast {
				border = p.Focus
			}
		}
		frame.Radius(12).Background(p.Content).Border(1, border).Shadow(0, 1, 5, 0, p.ContentShadow)
	}
	frame.Children(func() {
		caption := pane.Label
		if caption == "" && pane.Agent == "" {
			caption = "Terminal"
		}
		focusLabel := caption
		if focusLabel == "" {
			focusLabel = pane.Agent
		}
		native.Row(c.Key("card-toolbar:" + pane.ID)).Height(float32(cardHeaderHeight)).Shrink(0).FillWidth().PaddingX(8).Gap(4).AlignItems(native.Center).Children(func() {
			if r.W >= 180 {
				focus := native.ButtonBase(c.Key("card-heading:" + pane.ID)).Grow(1).Shrink(1).MinWidth(0).Height(26).Radius(4).Label("Focus " + focusLabel).KeepFocus().TextColor(p.Text)
				if focus.FocusVisible() {
					focus.Border(2, p.Focus)
				}
				focus.Children(func() {
					native.Row(c).FillWidth().MinWidth(0).Gap(6).AlignItems(native.Center).Children(func() {
						native.Icon(c, icons.Agent(pane.Agent)).Size(14, 14).TextColor(p.Secondary)
						if caption != "" {
							native.Text(c, caption).FontSize(12).SingleLine().MinWidth(0).TextColor(p.Secondary)
						}
					})
				})
				if focus.Clicked() {
					w.app.SelectPane(pane.ID)
					w.BlurInput()
				}
				if pane.Agent != "" && r.W >= 260 {
					native.Box(c).Radius(10).Background(p.Inset).Padding(2, 8).MaxWidth(88).Children(func() {
						native.Text(c, pane.Agent).FontSize(12).SingleLine().TextColor(p.Secondary).Tooltip(pane.Agent)
					})
				}
			}
			w.cardControls(c, pane, frameView, r.W, p)
		})
		body()
	})
}

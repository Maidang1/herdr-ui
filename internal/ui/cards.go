package ui

import (
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/Maidang1/herdr-ui/internal/layout"
	native "github.com/egoist/mygo/ui"
)

const cardHeaderHeight float64 = 32

// splitHairlines separates flush split panes with one-pixel lines at the
// authoritative divider centers, as Terminal's own splits do: no gaps, no
// rounded cards, the content panel's corners clipping the outer edges.
func splitHairlines(c *native.Context, splits []layout.SplitRect, p design.Palette) {
	for _, s := range splits {
		r := s.Rect
		line := native.Box(c.Key("split-line:" + s.ID)).Absolute().Background(p.Border).PassThrough()
		if s.Direction == "right" {
			line.Left(float32(r.X+r.W*s.Ratio-.5)).Top(float32(r.Y)).Size(1, float32(r.H))
		} else {
			line.Left(float32(r.X)).Top(float32(r.Y+r.H*s.Ratio-.5)).Size(float32(r.W), 1)
		}
	}
}

// A card is drawable only while its plugin content has positive dimensions.
func hasCardContent(r layout.Rect, multi bool) bool {
	// A single pane has no header: its controls live in the toolbar.
	header := 0.0
	if multi {
		header = cardHeaderHeight
	}
	return r.W > 32 && r.H > 24+header
}

func (w *Window) paneCard(c *native.Context, r layout.Rect, pane app.PaneRow, frameView app.View, selected, multi bool, p design.Palette, body func()) {
	frame := native.Column(c.Key("pane-frame:"+pane.ID)).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.W), float32(r.H)).Clip().AlignItems(native.Stretch)
	if multi {
		frame.Background(p.Content)
	}
	frame.Children(func() {
		// The header names the pane once: its label, else its agent beside
		// the agent's mark. Ordinary terminals show the mark alone.
		caption := pane.Label
		if caption == "" {
			caption = pane.Agent
		}
		focusLabel := caption
		if focusLabel == "" {
			focusLabel = "Terminal"
		}
		captionColor := p.Secondary
		if selected && multi {
			captionColor = p.Text
		}
		if multi {
			native.Row(c.Key("card-toolbar:" + pane.ID)).Height(float32(cardHeaderHeight)).Shrink(0).FillWidth().PaddingX(8).Gap(4).AlignItems(native.Center).Children(func() {
				if r.W >= 180 {
					focus := native.ButtonBase(c.Key("card-heading:"+pane.ID)).Grow(1).Shrink(1).MinWidth(0).Height(26).Radius(13).Padding(0, 6).Label("Focus " + focusLabel).KeepFocus().TextColor(p.Text)
					if focus.FocusVisible() {
						focus.Border(2, p.Focus)
					}
					focus.Children(func() {
						native.Row(c).FillWidth().MinWidth(0).Gap(7).AlignItems(native.Center).Children(func() {
							native.Icon(c, icons.Agent(pane.Agent)).Size(14, 14).TextColor(captionColor)
							if caption != "" {
								native.Text(c, caption).FontSize(12).FontWeight(500).SingleLine().MinWidth(0).TextColor(captionColor)
							}
						})
					})
					// Double-clicking a header zooms its pane, as double-clicking
					// a Mac title bar zooms its window.
					focus.ContextMenu(func(m *native.Menu) { w.paneMenu(m, pane.ID) })
					if focus.DoubleClicked() {
						w.app.SelectPane(pane.ID)
						w.dispatchCardAction("zoom", pane, frameView)
					} else if focus.Clicked() {
						w.app.SelectPane(pane.ID)
						w.BlurInput()
					}
				}
				w.cardControls(c, pane, frameView, r.W, p)
			})
			native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
		}
		body()
		if multi && !selected {
			// An unfocused pane steps back behind a translucent shell tint
			// over its content, below the header whose controls stay live.
			// The layer is decorative: it passes the pointer through.
			native.Box(c.Key("pane-dim:" + pane.ID)).Absolute().Left(0).Top(float32(cardHeaderHeight + 1)).Right(0).Bottom(0).Background(p.Shell.Alpha(.35)).PassThrough()
		}
	})
}

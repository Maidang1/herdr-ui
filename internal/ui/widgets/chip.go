package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// Chip is a small label that says a state: Blocked, Working, Idle.
//
// The design file's rule is that a chip is the quietest of the colored
// things in the window: a wash of color with the text of the same family
// over it.
func Chip(c *ui.Context, t design.Tokens, label string, tone Tone) ui.Element {
	fill, ink := tone.color(t)
	return ui.Row(c).
		Background(fill).
		Radius(design.RadiusSM).
		Padding(design.SpaceXS, design.SpaceMD).
		AlignItems(ui.Center).
		Children(func() {
			Label(c, t, label, LabelOptions{
				Size:   design.FontSmall,
				Color:  ink,
				Weight: design.WeightMedium,
			})
		})
}

// Badge is the count on a button of the activity bar.
//
// The design file's rule is that a badge sits at the top right of its
// button, overhanging it by about three pixels, and that it grows when
// the count has two digits.
func Badge(c *ui.Context, t design.Tokens, count int, tone Tone) ui.Element {
	label := ""
	if count > 0 {
		label = itoa(count)
	}
	width := float32(design.BadgeHeight)
	if count > 9 {
		width += 6
	}
	return ui.Row(c).
		Background(tone.strong(t)).
		Radius(design.BadgeRadius).
		Size(width, design.BadgeHeight).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Children(func() {
			Label(c, t, label, LabelOptions{
				Size:  design.FontMicro,
				Color: ui.Hex("#FFFFFF"),
			})
		})
}

// Meter is one of the little meters of the row of meters: a bar and the
// text that says how much of it is used.
//
// The design file's rule is that the bar is four pixels high, that it
// wears the color of the state it is in, and that it never says more than
// the space it has.
func Meter(c *ui.Context, t design.Tokens, icon string, value float32, label string, tone Tone) ui.Element {
	fill, _ := tone.color(t)
	return ui.Row(c).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			if icon != "" {
				Icon(c, t, icon, 12, t.TextTertiary)
			}
			ui.Row(c).
				Background(t.TrackOff).
				Radius(design.RadiusXS).
				Size(design.MeterBarWidth, design.MeterBarHeight).
				Children(func() {
					if value <= 0 {
						return
					}
					width := design.MeterBarWidth * value
					if width > design.MeterBarWidth {
						width = design.MeterBarWidth
					}
					ui.Row(c).
						Background(fill).
						Radius(design.RadiusXS).
						Size(width, design.MeterBarHeight)
				})
			Label(c, t, label, text(t, design.FontMicro, t.TextSecondary))
		})
}

// itoa writes a number the way the window writes it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

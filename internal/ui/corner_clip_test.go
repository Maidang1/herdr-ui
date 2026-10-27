package ui

import (
	"image/color"
	"testing"

	native "github.com/egoist/mygo/ui"
)

func TestRoundedPanelClipsASquareChild(t *testing.T) {
	const size = 200
	tester := native.NewTester(func(c *native.Context) {
		native.Box(c).Fill().Background(native.Hex("#FFFFFF")).Children(func() {
			native.Column(c).Fill().Padding(8).Children(func() {
				panel := native.Column(c).Grow(1).FillWidth().Radius(18).Background(native.Hex("#FFFFFF")).Clip()
				panel.Children(func() {
					native.Box(c).Grow(1).FillWidth().Draw(func(p *native.Painter, r native.Rect) {
						p.Fill(r, native.Hex("#000000"), 0)
					})
				})
			})
		})
	}, size, size)
	tester.Frame()
	img := tester.Image()
	// One pixel in from the panel's square corner is outside an 18pt curve.
	c := color.RGBAModel.Convert(img.At(size-8-1, size-8-1)).(color.RGBA)
	if c.R < 200 || c.G < 200 || c.B < 200 {
		t.Fatalf("square child covers the rounded corner: #%02x%02x%02x", c.R, c.G, c.B)
	}
}

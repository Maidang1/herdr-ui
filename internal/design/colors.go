package design

import "github.com/egoist/mygo/ui"

// mix blends two colors, t of the second in the first.
//
// It is how a color the design file does not give is made: the window's
// hover color is its own color mixed with white, never a color picked out
// of the air.
func mix(a, b ui.Color, t float32) ui.Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	blend := func(x, y uint8) uint8 {
		v := float64(x)*(1-float64(t)) + float64(y)*float64(t)
		return uint8(v + 0.5)
	}
	return ui.Color{
		R: blend(a.R, b.R),
		G: blend(a.G, b.G),
		B: blend(a.B, b.B),
		A: blend(a.A, b.A),
	}
}

// white and black are the colors the window mixes with.
var (
	white = ui.Hex("#FFFFFF")
	black = ui.Hex("#000000")
)

// lighten returns the color a little lighter than the one given.
func lighten(c ui.Color, t float32) ui.Color { return mix(c, white, t) }

// darken returns the color a little darker than the one given.
func darken(c ui.Color, t float32) ui.Color { return mix(c, black, t) }

// withAlpha returns the color with its alpha set.
func withAlpha(c ui.Color, alpha float32) ui.Color {
	c.A = uint8(alpha*255 + 0.5)
	return c
}

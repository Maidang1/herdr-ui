package surfaceview

import (
	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/egoist/mygo/ui"
	"math"
)

// Herdr packs a color kind in the high byte: named=0, indexed=1, RGB=2.
func (v *View) color(packed uint32, fallback ui.Color) ui.Color {
	switch packed >> 24 {
	case 0:
		n := int(packed & 255)
		if n == 0 || n > 16 {
			return fallback
		}
		return v.theme.Palette[n-1]
	case 1:
		n := int(packed & 255)
		if n < 16 {
			return v.theme.Palette[n]
		}
		if n >= 232 {
			g := uint8(8 + (n-232)*10)
			return ui.RGB(g, g, g)
		}
		n -= 16
		levels := [6]uint8{0, 95, 135, 175, 215, 255}
		return ui.RGB(levels[n/36], levels[n/6%6], levels[n%6])
	case 2:
		return ui.RGB(uint8(packed>>16), uint8(packed>>8), uint8(packed))
	}
	return fallback
}
func luminance(c ui.Color) float64 {
	f := func(b uint8) float64 {
		x := float64(b) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*f(c.R) + .7152*f(c.G) + .0722*f(c.B)
}
func contrast(a, b ui.Color) float64 {
	x, y := luminance(a), luminance(b)
	return (max(x, y) + .05) / (min(x, y) + .05)
}
func (v *View) colors(cell endpoint.SurfaceCell) (ui.Color, ui.Color) {
	fg, bg := v.color(cell.FG, v.theme.Foreground), v.color(cell.BG, v.theme.Background)
	if cell.Modifier&64 != 0 {
		fg, bg = bg, fg
	}
	if cell.Modifier&2 != 0 {
		fg = ui.RGB(uint8((uint16(fg.R)+uint16(bg.R))/2), uint8((uint16(fg.G)+uint16(bg.G))/2), uint8((uint16(fg.B)+uint16(bg.B))/2))
	}
	ratio := float64(v.cfg.MinimumContrast)
	if ratio > 1 && contrast(fg, bg) < ratio {
		target := ui.RGB(255, 255, 255)
		if contrast(ui.RGB(0, 0, 0), bg) > contrast(target, bg) {
			target = ui.RGB(0, 0, 0)
		}
		original := fg
		for step := 1; step <= 20; step++ {
			t := float64(step) / 20
			mix := func(a, b uint8) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }
			fg = ui.RGB(mix(original.R, target.R), mix(original.G, target.G), mix(original.B, target.B))
			if contrast(fg, bg) >= ratio {
				break
			}
		}
	}
	return fg, bg
}

package design

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestWindowPaletteInactiveAndContrast(t *testing.T) {
	for _, dark := range []bool{false, true} {
		normal := WindowColors(dark, false, false)
		inactive := WindowColors(dark, false, true)
		if inactive.Selected == normal.Selected {
			t.Fatal("inactive selection has no distinct surface")
		}
		if inactive.Text != normal.Text || inactive.Secondary != normal.Secondary {
			t.Fatal("inactive window dims text")
		}
		high := WindowColors(dark, true, false)
		if high.Border != high.ControlBorder || high.Border == normal.Border {
			t.Fatal("increased contrast retains decorative border")
		}
		if high.Focus.A != 255 {
			t.Fatal("focus ring must be opaque")
		}
		theme := NativeTheme(dark, true)
		if theme.Focus != high.Focus || theme.Border != high.ControlBorder || theme.Font != "" || theme.FontSize != 13 {
			t.Fatal("native controls ignore design/system font")
		}
	}
}

func TestShellTintDoesNotFadeContentTokens(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := Colors(dark)
		if p.Content.A != 255 || p.Elevated.A != 255 || p.Grouped.A != 255 {
			t.Fatal("content surfaces lost opacity")
		}
		if p.SidebarTint.A == 255 || p.Sidebar.A != 255 {
			t.Fatal("sidebar tint/fallback not distinguished")
		}
		if p.Focus != p.Accent {
			t.Fatal("focus must use action accent")
		}
	}
}

// Labels are translucent, as AppKit's: on every opaque surface they still
// read at WCAG AA, and each level steps back from the one before.
func TestLabelsReadOnEverySurface(t *testing.T) {
	for _, dark := range []bool{false, true} {
		p := Colors(dark)
		for _, bg := range []ui.Color{p.Content, p.Elevated, p.Grouped, p.Shell} {
			previous := 22.0
			for _, label := range []ui.Color{p.Text, p.Secondary, p.Tertiary} {
				ratio := contrast(Over(label, bg), bg)
				if ratio < 4.5 || ratio >= previous {
					t.Fatalf("dark=%t label %v on %v: contrast %.2f", dark, label, bg, ratio)
				}
				previous = ratio
			}
		}
	}
}

func TestOverCompositesOpaque(t *testing.T) {
	got := Over(ui.Hex("#FFFFFF80"), ui.Hex("#000000"))
	if got.A != 255 || got.R < 127 || got.R > 129 {
		t.Fatalf("Over = %+v", got)
	}
	if solid := ui.Hex("#123456"); Over(solid, ui.Hex("#FFFFFF")) != solid {
		t.Fatal("an opaque label changed over its background")
	}
}

func contrast(a, b ui.Color) float64 {
	la, lb := luminance(a), luminance(b)
	return (max(la, lb) + .05) / (min(la, lb) + .05)
}

func luminance(c ui.Color) float64 {
	channel := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= .04045 {
			return s / 12.92
		}
		return math.Pow((s+.055)/1.055, 2.4)
	}
	return .2126*channel(c.R) + .7152*channel(c.G) + .0722*channel(c.B)
}

func TestShapesAreConcentric(t *testing.T) {
	for _, window := range []float32{10, 26} {
		s := Shapes(window)
		if window == 26 && (s.Panel != window-s.Gap || s.Row != s.Panel-s.Gap) {
			t.Fatalf("nested radii are not concentric: %+v", s)
		}
		if s.MenuItem != s.Popover-MenuPadding {
			t.Fatalf("menu items not concentric with their popover: %+v", s)
		}
		if s.Row <= 0 || s.Panel < s.Row || s.Card <= 0 {
			t.Fatalf("degenerate radii: %+v", s)
		}
	}
}

package design

import (
	"testing"
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
		if p.Content.A != 255 || p.Text.A != 255 || p.Secondary.A != 255 {
			t.Fatal("content/typography lost opacity")
		}
		if p.SidebarTint.A == 255 || p.Sidebar.A != 255 {
			t.Fatal("sidebar tint/fallback not distinguished")
		}
		if p.Focus != p.Accent {
			t.Fatal("focus must use action accent")
		}
	}
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

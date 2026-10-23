package ui

import (
	"github.com/bytedance/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

type windowAppearance struct {
	active              bool
	reducedTransparency bool
	theme               *native.Theme
	dark, highContrast  bool
}

// These setters are called on the UI thread by the native window assembly.
func (w *Window) SetReducedTransparency(reduced bool) { w.appearance.reducedTransparency = reduced }
func (w *Window) SetWindowActive(active bool) {
	w.appearance.active = active
	if !active {
		w.BlurInput()
	}
}

func (w *Window) appearancePalette(c *native.Context) design.Palette {
	dark, high := c.Theme().Dark, c.Preferences().HighContrast
	a := &w.appearance
	if a.theme == nil || a.dark != dark || a.highContrast != high {
		a.theme = design.NativeTheme(dark, high)
		a.dark = dark
		a.highContrast = high
	}
	c.SetTheme(a.theme)
	p := design.WindowColors(dark, high, !a.active)
	if c.Vibrancy() && !a.reducedTransparency && !high {
		p.Sidebar = p.SidebarTint
	}
	return p
}

func (w *Window) styleShell(c *native.Context, root native.Element, p design.Palette) {
	material := c.Vibrancy() && !w.appearance.reducedTransparency && !c.Preferences().HighContrast
	alpha := float32(1)
	if material {
		// Keep the shell wash light enough for the behind-window blur to
		// read; the sidebar tint layers its own frost over it.
		c.Root().Background(native.Transparent)
		alpha = .8
	} else {
		c.Root().Background(p.Shell)
	}
	root.LinearGradient(native.LinearGradient{Angle: 180, Stops: []native.GradientStop{{Color: p.Top.Alpha(alpha), At: 0}, {Color: p.Middle.Alpha(alpha), At: .52}, {Color: p.Bottom.Alpha(alpha), At: 1}}})
}

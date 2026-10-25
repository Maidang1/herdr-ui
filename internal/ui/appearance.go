package ui

import (
	"github.com/Maidang1/herdr-ui/internal/design"
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
	if w.translucent(c) {
		p.Sidebar = p.SidebarTint
	}
	return p
}

// translucent tells whether the window may show materials: the system
// draws its behind-window material, and neither Reduce Transparency nor
// Increase Contrast asks for opaque surfaces.
func (w *Window) translucent(c *native.Context) bool {
	return c.Vibrancy() && !w.appearance.reducedTransparency && !c.Preferences().HighContrast
}

// glassy tells whether floating surfaces may be Liquid Glass. Glass
// refracts what the window itself painted, so it needs no window material,
// only the user's leave to be translucent.
func (w *Window) glassy(c *native.Context) bool {
	return !w.appearance.reducedTransparency && !c.Preferences().HighContrast
}

func (w *Window) styleShell(c *native.Context, root native.Element, p design.Palette) {
	if w.translucent(c) {
		// The system's sidebar material is the shell: its blur and the
		// wallpaper's tint show through, under only a faint neutral wash
		// that steadies the labels on it.
		c.Root().Background(native.Transparent)
		root.Background(p.SidebarTint)
		return
	}
	c.Root().Background(p.Shell)
	root.LinearGradient(native.LinearGradient{Angle: 180, Stops: []native.GradientStop{{Color: p.Top, At: 0}, {Color: p.Middle, At: .52}, {Color: p.Bottom, At: 1}}})
}

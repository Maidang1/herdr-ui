package ui

import (
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
	"testing"
)

func TestShellFollowsMaterialAndAccessibilityPreferences(t *testing.T) {
	w := New(app.New(), nil)
	var p design.Palette
	tester := native.NewTester(func(c *native.Context) {
		p = w.appearancePalette(c)
		shell := native.Row(c).Fill()
		w.styleShell(c, shell, p)
	}, 100, 100)
	if p.Sidebar.A != 255 {
		t.Fatal("material-unavailable sidebar must be opaque")
	}
	tester.SetVibrancy(true)
	tester.Frame()
	if p.Sidebar.A == 255 {
		t.Fatal("material tint is opaque")
	}
	_, _, _, alpha := tester.Image().At(50, 0).RGBA()
	if alpha >= 65535 {
		t.Fatal("material shell did not expose native material")
	}
	w.SetReducedTransparency(true)
	tester.Frame()
	if p.Sidebar.A != 255 {
		t.Fatal("reduced transparency retains tint")
	}
	_, _, _, alpha = tester.Image().At(50, 0).RGBA()
	if alpha != 65535 {
		t.Fatal("reduced transparency shell not opaque")
	}
	w.SetReducedTransparency(false)
	tester.SetPreferences(native.Preferences{HighContrast: true})
	tester.Frame()
	if p.Border != p.ControlBorder || p.Sidebar.A != 255 {
		t.Fatal("high contrast retains subtle borders or translucent sidebar")
	}
	_, _, _, alpha = tester.Image().At(50, 0).RGBA()
	if alpha != 65535 {
		t.Fatal("high contrast shell not opaque")
	}
	tester.SetDark(true)
	tester.Frame()
	if p.Content != design.Colors(true).Content {
		t.Fatal("custom native theme stopped following dark mode")
	}
}

func TestWindowInactiveStateKeepsReadableTextAndCancelsInput(t *testing.T) {
	w := New(app.New(), nil)
	w.controller = &actionController{}
	var p design.Palette
	tester := native.NewTester(func(c *native.Context) { p = w.appearancePalette(c) }, 100, 100)
	active := p
	w.controller.machine.Awaiting = true
	w.controller.machine.Resize = true
	w.SetWindowActive(false)
	tester.Frame()
	if p.Selected == active.Selected || p.Text != active.Text || p.Secondary != active.Secondary {
		t.Fatal("inactive selection or text does not follow design")
	}
	if w.controller.machine.Awaiting || w.controller.machine.Resize {
		t.Fatal("window blur left prefix active")
	}
	w.SetWindowActive(true)
	tester.Frame()
	if p.Selected != active.Selected {
		t.Fatal("focus regain did not restore selection")
	}
}

// Floating surfaces are glass only while the user allows transparency;
// the settings window shows its material only where the system draws it,
// and paints itself opaque under Reduce Transparency or Increase Contrast.
func TestMaterialsFallBackToOpaqueSurfaces(t *testing.T) {
	w := New(app.New(), nil)
	var glassy, translucent bool
	tester := native.NewTester(func(c *native.Context) {
		w.SettingsView(c)
		glassy, translucent = w.glassy(c), w.translucent(c)
	}, 720, 520)
	navAlpha := func() uint32 {
		_, _, _, a := tester.Image().At(20, 500).RGBA()
		return a
	}
	if !glassy || translucent || navAlpha() != 65535 {
		t.Fatal("without the window material the settings sidebar must be opaque")
	}
	tester.SetVibrancy(true)
	tester.Frame()
	if !glassy || !translucent || navAlpha() == 65535 {
		t.Fatal("settings sidebar hides the window material")
	}
	w.SetReducedTransparency(true)
	tester.Frame()
	if glassy || translucent || navAlpha() != 65535 {
		t.Fatal("reduced transparency kept glass or material")
	}
	w.SetReducedTransparency(false)
	tester.SetPreferences(native.Preferences{HighContrast: true})
	tester.Frame()
	if glassy || translucent || navAlpha() != 65535 {
		t.Fatal("increased contrast kept glass or material")
	}
}

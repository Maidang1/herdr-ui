package terminalhost

import (
	"fmt"
	"io"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestAttachRGBBackgroundFollowsWindowAppearance(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	light, dark := terminalThemes()
	term, err := terminal.New(terminal.Options{Conn: appearanceConn{reader}, Theme: light, DarkTheme: dark, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	plugin := &pluginInstance{term}
	tester := ui.NewTester(func(c *ui.Context) { plugin.View(c).Fill() }, 320, 180)
	// Herdr's frame encoder emits resolved RGB backgrounds, including on
	// blank cells; there need not be any OSC 11 command in the stream.
	cols, rows := term.Size()
	frame := "\x1b[?2026h\x1b[?25l\x1b[2J"
	for y := 1; y <= rows; y++ {
		frame += fmt.Sprintf("\x1b[%d;1H\x1b[0;48;2;41;39;41m%*s", y, cols, "")
	}
	frame += "\x1b[1;1Hbase\x1b[2;1H\x1b[48;2;255;0;0m    \x1b[3;1H\x1b[48;2;80;80;80m    \x1b[0m\x1b[?2026l"
	term.Feed([]byte(frame))
	for _, darkMode := range []bool{false, true, false} {
		tester.SetDark(darkMode)
		tester.Frame()
		want := light.Background
		if darkMode {
			want = dark.Background
		}
		img := tester.Image()
		bg := img.RGBAAt(img.Bounds().Dx()/2, img.Bounds().Dy()/2)
		if bg.R != want.R || bg.G != want.G || bg.B != want.B {
			t.Fatalf("RGB canvas background = %v, want %v (dark=%v)", bg, want, darkMode)
		}
		red, gray := false, false
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				p := img.RGBAAt(x, y)
				red = red || p.R == 255 && p.G == 0 && p.B == 0
				gray = gray || p.R == 80 && p.G == 80 && p.B == 80
			}
		}
		if !red || !gray {
			t.Fatalf("highlight backgrounds lost: red=%v gray=%v", red, gray)
		}
	}
}

// Keep the reader alive while Feed supplies deterministic attach output.
type appearanceConn struct{ *io.PipeReader }

func (appearanceConn) Write(p []byte) (int, error) { return len(p), nil }

func TestAttachColorsFollowWindowAppearance(t *testing.T) {
	reader, writer := io.Pipe()
	defer writer.Close()
	light, dark := terminalThemes()
	term, err := terminal.New(terminal.Options{Conn: appearanceConn{reader}, Theme: light, DarkTheme: dark, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	plugin := &pluginInstance{term}
	tester := ui.NewTester(func(c *ui.Context) { plugin.View(c).Fill() }, 320, 180)
	check := func(want *terminal.Theme) {
		t.Helper()
		// Replayed default colors must not override the window, but explicit
		// per-cell backgrounds used by terminal programs must still render.
		term.Feed([]byte("\x1b]10;#eeeeee\x07\x1b]11;#292729\x07\x1b[2;1H\x1b[48;2;255;0;0m    \x1b[0m\x1b[3;1H████"))
		tester.Frame()
		img := tester.Image()
		bg := img.RGBAAt(img.Bounds().Dx()-20, img.Bounds().Dy()-20)
		if bg.R != want.Background.R || bg.G != want.Background.G || bg.B != want.Background.B {
			t.Fatalf("attach background = %v, want window background %v", bg, want.Background)
		}
		red, foreground := false, false
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				p := img.RGBAAt(x, y)
				if p.R == 255 && p.G == 0 && p.B == 0 {
					red = true
				}
				if p.R == want.Foreground.R && p.G == want.Foreground.G && p.B == want.Foreground.B {
					foreground = true
				}
			}
		}
		if !red {
			t.Fatal("explicit cell background was lost")
		}
		if !foreground {
			t.Fatalf("default text did not use window foreground %v", want.Foreground)
		}
	}
	check(light)
	tester.SetDark(true)
	check(dark)
	tester.SetDark(false)
	check(light)
	light, dark = terminal.LightTheme(), terminal.DarkTheme()
	light.Background, light.Foreground = ui.RGB(230, 220, 210), ui.RGB(30, 40, 50)
	dark.Background, dark.Foreground = ui.RGB(20, 30, 40), ui.RGB(220, 230, 240)
	plugin.SetTheme(light, dark)
	check(light)
	tester.SetDark(true)
	check(dark)
}

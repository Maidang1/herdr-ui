package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheLadderOfSurfaces checks the eight surfaces of the token board
// against the tokens they are made of.
func TestTheLadderOfSurfaces(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
		dark bool
	}{
		{"light", Light(), false},
		{"dark", Dark(), true},
	} {
		surfaces := mode.toks.Surfaces()
		want := []struct {
			level string
			base  ui.Color
		}{
			{"window", mode.toks.WinBg},
			{"titlebar", mode.toks.Titlebar},
			{"panel", mode.toks.Panel},
			{"solid", mode.toks.Surface},
			{"elevated", mode.toks.Elevated},
			{"overlay", mode.toks.Overlay},
			{"terminal", mode.toks.TerminalBg},
			{"rail", mode.toks.RailBg},
		}
		got := []struct {
			level string
			base  ui.Color
		}{
			{"window", surfaces.Window.Base},
			{"titlebar", surfaces.Titlebar.Base},
			{"panel", surfaces.Panel.Base},
			{"solid", surfaces.Solid.Base},
			{"elevated", surfaces.Elevated.Base},
			{"overlay", surfaces.Overlay.Base},
			{"terminal", surfaces.Terminal.Base},
			{"rail", surfaces.Rail.Base},
		}
		for i := range want {
			if want[i].level != got[i].level {
				t.Fatalf("%s: the ladder is in the wrong order at %d", mode.name, i)
			}
			if want[i].base != got[i].base {
				t.Errorf("%s: the %s surface is %v, the design file says %v", mode.name, want[i].level, got[i].base, want[i].base)
			}
		}
	}
}

// TestEverySurfaceHasItsStates checks that every step of the ladder has a
// color for every state, and that the step that lies on it is the one the
// design file says: the panel lies on the title bar, the terminal on the
// solid surface, the glass of the activity bar on the selection.
func TestEverySurfaceHasItsStates(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		surfaces := mode.toks.Surfaces()
		for _, surface := range []struct {
			level  string
			s      Surface
			raised string
		}{
			{"window", surfaces.Window, "titlebar"},
			{"titlebar", surfaces.Titlebar, "panel"},
			{"panel", surfaces.Panel, "elevated"},
			{"solid", surfaces.Solid, "elevated"},
			{"terminal", surfaces.Terminal, "surface"},
			{"rail", surfaces.Rail, "selection"},
		} {
			want, ok := mode.toks.Token(surface.raised)
			if !ok {
				t.Fatalf("%s: no token named %q", mode.name, surface.raised)
			}
			if surface.s.Raised != want {
				t.Errorf("%s: the %s surface is raised by %v, the design file says %v", mode.name, surface.level, surface.s.Raised, want)
			}
			if surface.s.Hover == surface.s.Base {
				t.Errorf("%s: the %s surface does not change under the pointer", mode.name, surface.level)
			}
			if surface.s.Pressed == surface.s.Hover {
				t.Errorf("%s: the %s surface does not change when it is pressed", mode.name, surface.level)
			}
		}
		// The two steps at the top of the ladder have no step above
		// them, so they take their own color mixed with white. In the
		// light mode the elevated surface is already white, so there is
		// nothing for it to take.
		if mode.toks.Dark && surfaces.Elevated.Raised == surfaces.Elevated.Base {
			t.Errorf("%s: the elevated surface is not raised above itself", mode.name)
		}
		if surfaces.Overlay.Raised == surfaces.Overlay.Base {
			t.Errorf("%s: the overlay is not raised above itself", mode.name)
		}
	}
}

// TestTheWindowGradient checks the gradient the window sits on.
func TestTheWindowGradient(t *testing.T) {
	cases := []struct {
		name  string
		toks  Tokens
		stops []string
	}{
		{"light", Light(), []string{"#DCCFF9", "#F7CDE3", "#FFE7CE"}},
		{"dark", Dark(), []string{"#1E1638", "#2E1836", "#35201A"}},
	}
	for _, c := range cases {
		gradient := c.toks.WindowGradient()
		if len(gradient.Stops) != len(c.stops) {
			t.Fatalf("%s: the gradient has %d stops, the design file has %d", c.name, len(gradient.Stops), len(c.stops))
		}
		if gradient.Angle != GradientAngle {
			t.Errorf("%s: the gradient turns %v degrees, the design file says %d", c.name, gradient.Angle, GradientAngle)
		}
		for i, want := range c.stops {
			if gradient.Stops[i].Color != ui.Hex(want) {
				t.Errorf("%s: the stop %d is %v, the design file says %s", c.name, i, gradient.Stops[i].Color, want)
			}
		}
	}
}

// TestTheFrameOfTheWindow checks the lines around the window and around the
// card in the middle of it.
func TestTheFrameOfTheWindow(t *testing.T) {
	if got := Light().WindowFrameBorder(); got != ui.Hex("#FFFFFF59") {
		t.Errorf("the light frame is %v, the design file says #FFFFFF59", got)
	}
	if got := Dark().WindowFrameBorder(); got != ui.Hex("#FFFFFF21") {
		t.Errorf("the dark frame is %v, the design file says #FFFFFF21", got)
	}
	if got := Light().MainCardBorder(); got != ui.Hex("#FFFFFF66") {
		t.Errorf("the light card is outlined in %v, the design file says #FFFFFF66", got)
	}
	if got := Dark().MainCardBorder(); got != ui.Hex("#FFFFFF1A") {
		t.Errorf("the dark card is outlined in %v, the design file says #FFFFFF1A", got)
	}
}

// TestTheTrackOfAControl checks the rule of the controls: a control that is
// on wears the accent, a control that is off wears a gray track.
func TestTheTrackOfAControl(t *testing.T) {
	light := Light().Track()
	if light.On != Light().AccentTrack {
		t.Error("the light track that is on is not the accent track")
	}
	if light.Off != Light().TrackOff {
		t.Error("the light track that is off is not the track that is off")
	}
	if light.KnobOn != ui.Hex("#FFFFFF") {
		t.Error("the knob of a control that is on is not white")
	}
	if light.KnobOff != Light().KnobOff {
		t.Error("the knob of a control that is off is not the knob that is off")
	}
}

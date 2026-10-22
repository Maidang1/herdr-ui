package design

import "github.com/egoist/mygo/ui"

// Surface is one step of the ladder of surfaces.
//
// The design file gives each step a base color; the rest of the step —
// what the step looks like when it is raised, hovered, pressed or outlined
// — is derived from that base, the way the material rules of the design
// file say.
type Surface struct {
	// Base is the color of the surface itself.
	Base ui.Color
	// Raised is the color of a panel that lies on the surface.
	Raised ui.Color
	// Hover is the color of the surface under the pointer.
	Hover ui.Color
	// Pressed is the color of the surface that is being pressed.
	Pressed ui.Color
	// Border is the color of the line that outlines the surface.
	Border ui.Color
}

// Surfaces is the ladder of the surfaces of the window.
//
// The design file's token board has eight of them, from the base of the
// window to the glass of the activity bar.
type Surfaces struct {
	// Window is the base of the window, the gradient's own ground.
	Window Surface
	// Titlebar is the bar at the top of the window.
	Titlebar Surface
	// Panel is the frosted panel the body of the window sits on.
	Panel Surface
	// Solid is the solid surface, the one that holds a card.
	Solid Surface
	// Elevated is the surface that floats above the window: a tip, a
	// menu.
	Elevated Surface
	// Overlay is the scrim over the window, under a modal.
	Overlay Surface
	// Terminal is the ground of the terminal.
	Terminal Surface
	// Rail is the glass of the activity bar.
	Rail Surface
}

// Surfaces returns the ladder of surfaces of one mode.
func (t Tokens) Surfaces() Surfaces {
	// The elevated surface is the top of the ladder, and nothing lies
	// on it. In the dark mode it takes a little more white; in the light
	// mode it is already white, which is the brightest color there is,
	// so it stays as it is.
	elevated := t.Elevated
	if t.Dark {
		elevated = lighten(t.Elevated, 0.07)
	}
	return Surfaces{
		Window:   t.surface(t.WinBg, t.Titlebar),
		Titlebar: t.surface(t.Titlebar, t.Panel),
		Panel:    t.surface(t.Panel, t.Elevated),
		Solid:    t.surface(t.Surface, t.Elevated),
		Elevated: t.surface(t.Elevated, elevated),
		Overlay:  t.surface(t.Overlay, lighten(t.Overlay, 0.04)),
		Terminal: t.surface(t.TerminalBg, t.Surface),
		Rail:     t.surface(t.RailBg, t.Selection),
	}
}

// surface derives one step of the ladder from its base and from the step
// that lies on it.
//
// The design file's rule is that a surface that is raised is brighter than
// the one it lies on, and that it is not lifted by a shadow but by the
// color it has once it is mixed with what is behind it. So the raised
// color is the next step of the ladder, and the step above the last one is
// its own base mixed with white.
func (t Tokens) surface(base, raised ui.Color) Surface {
	s := Surface{Base: base, Raised: raised, Border: t.Separator}
	if t.Dark {
		s.Hover = lighten(base, 0.05)
		s.Pressed = darken(base, 0.16)
		return s
	}
	s.Hover = darken(base, 0.04)
	s.Pressed = darken(base, 0.08)
	return s
}

// The six material rules of the design file.
const (
	// The window sits on a gradient of 135 degrees, of three stops.
	GradientAngle = 135
)

// Gradient is the wash of color the window sits on.
//
// The design file calls for a gradient that turns from a cold violet
// through a rose to a warm sand in the light mode, and from a deep violet
// through a plum to a warm brown in the dark mode: a shift of temperature
// rather than a plain gray.
type Gradient struct {
	// Stops are the colors of the gradient, in order.
	Stops []GradientStop
	// Angle is the direction of the gradient, in degrees.
	Angle float32
}

// GradientStop is one color of a gradient, at its place along it.
type GradientStop struct {
	// Color is the color of the stop.
	Color ui.Color
	// At is where the stop lies along the gradient, from 0 to 1.
	At float32
}

// WindowGradient returns the gradient the window sits on.
func (t Tokens) WindowGradient() Gradient {
	if t.Dark {
		return Gradient{
			Angle: GradientAngle,
			Stops: []GradientStop{
				{Color: ui.Hex("#1E1638"), At: 0},
				{Color: ui.Hex("#2E1836"), At: 0.5},
				{Color: ui.Hex("#35201A"), At: 1},
			},
		}
	}
	return Gradient{
		Angle: GradientAngle,
		Stops: []GradientStop{
			{Color: ui.Hex("#DCCFF9"), At: 0},
			{Color: ui.Hex("#F7CDE3"), At: 0.5},
			{Color: ui.Hex("#FFE7CE"), At: 1},
		},
	}
}

// WindowFrameBorder returns the color of the line around the window.
//
// The design file draws the window as a card of its own, with a hairline of
// white over the gradient.
func (t Tokens) WindowFrameBorder() ui.Color {
	if t.Dark {
		return ui.Hex("#FFFFFF21")
	}
	return ui.Hex("#FFFFFF59")
}

// MainCardBorder returns the color of the line around the card in the
// middle of the window.
func (t Tokens) MainCardBorder() ui.Color {
	if t.Dark {
		return ui.Hex("#FFFFFF1A")
	}
	return ui.Hex("#FFFFFF66")
}

// Track is the switch of a control: the track it runs on and the knob that
// rides it.
type Track struct {
	// On is the color of the track of a control that is on.
	On ui.Color
	// Off is the color of the track of a control that is off.
	Off ui.Color
	// KnobOn is the color of the knob of a control that is on.
	KnobOn ui.Color
	// KnobOff is the color of the knob of a control that is off.
	KnobOff ui.Color
}

// Track returns the colors of the controls of one mode.
//
// The design file's rule is that a control that is on wears the accent and
// a knob of white, and a control that is off wears a gray track and a gray
// knob: the knob is always the same color as the track's own family, so
// that the two states read as one control.
func (t Tokens) Track() Track {
	return Track{
		On:      t.AccentTrack,
		Off:     t.TrackOff,
		KnobOn:  white,
		KnobOff: t.KnobOff,
	}
}

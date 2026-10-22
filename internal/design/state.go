package design

import "github.com/egoist/mygo/ui"

// Interaction is the state a control is in.
//
// The design file gives every control in every board the same five states,
// so the window holds them in one place rather than in each widget.
type Interaction int

const (
	// StateNormal is the state of a control that is at rest.
	StateNormal Interaction = iota
	// StateHover is the state of a control under the pointer.
	StateHover
	// StatePress is the state of a control that is being pressed.
	StatePress
	// StateFocus is the state of a control that has the keyboard.
	StateFocus
	// StateDisabled is the state of a control that cannot be used.
	StateDisabled
	// StateSelected is the state of a control that is chosen.
	StateSelected
)

// String names the state.
func (i Interaction) String() string {
	switch i {
	case StateNormal:
		return "normal"
	case StateHover:
		return "hover"
	case StatePress:
		return "press"
	case StateFocus:
		return "focus"
	case StateDisabled:
		return "disabled"
	case StateSelected:
		return "selected"
	}
	return "unknown"
}

// The rules of the states, as the design file's board of them says.
const (
	// FocusRingWidth is the width of the ring around a control that has
	// the keyboard.
	FocusRingWidth = 2
	// FocusRingOffset is the distance between the ring and the control.
	FocusRingOffset = 2
	// PressDarken is how much darker the fill of a pressed control is
	// than the fill of a hovered one.
	PressDarken = 0.06
	// HitArea is the smallest a control the pointer may hit may be.
	HitArea = 24
)

// Icon returns the color of the icon of a control in a state.
//
// The design file's rule is that an icon that is at rest is the third
// grade of text, and that hovering brings it up to the first grade. It
// never changes its size.
func (t Tokens) Icon(state Interaction) ui.Color {
	switch state {
	case StateDisabled:
		return t.disabledText()
	case StateHover, StatePress, StateFocus:
		return t.TextPrimary
	}
	return t.TextTertiary
}

// Fill returns the color to fill a control with in a state.
//
// The design file's rule is that a pressed control is six percent darker
// than a hovered one, and that no state changes the size of anything, so
// that a row never moves under the pointer.
func (t Tokens) Fill(base ui.Color, state Interaction) ui.Color {
	switch state {
	case StateDisabled:
		return t.disabledFill(base)
	case StateHover:
		return t.hoverFill(base)
	case StatePress:
		return darken(t.hoverFill(base), PressDarken)
	case StateFocus:
		return t.hoverFill(base)
	case StateSelected:
		return t.Selection
	}
	return base
}

// Label returns the color of the text of a control in a state.
func (t Tokens) Label(base ui.Color, state Interaction) ui.Color {
	switch state {
	case StateDisabled:
		return t.disabledText()
	case StateHover, StatePress, StateFocus:
		return t.TextPrimary
	case StateSelected:
		return t.TextPrimary
	}
	return base
}

// FocusRing returns the color of the ring around a control that has the
// keyboard.
//
// The design file's rule is that the ring is the accent, two pixels wide
// and two pixels away from the control, and that hover and press never
// take it away.
func (t Tokens) FocusRing() ui.Color {
	return t.Accent
}

// chipHover is the color of a chip under the pointer.
//
// The design file gives the light mode's chip the color of a warm paper,
// #FFEFE2. The dark mode's chip is white over the window, so it takes
// more white under the pointer.
var chipHover = ui.Hex("#FFEFE2")

// hoverFill returns the fill of a control under the pointer.
//
// A chip takes the color of a warm paper; everything else takes its own
// color, a little brighter in the dark mode and a little darker in the
// light one.
func (t Tokens) hoverFill(base ui.Color) ui.Color {
	if base == t.Chip {
		if t.Dark {
			return lighten(base, 0.08)
		}
		return chipHover
	}
	return t.Surfaces().Solid.Hover
}

// disabledFill returns the fill of a control that cannot be used.
//
// The design file keeps the geometry of a disabled control, so that a row
// never reflows; only its colors change.
func (t Tokens) disabledFill(base ui.Color) ui.Color {
	if t.Dark {
		return mix(base, t.WinBg, 0.55)
	}
	return mix(base, ui.Hex("#E7D9D7"), 0.6)
}

// disabledText returns the color of the text of a control that cannot be
// used.
//
// The design file gives the light mode the color of a faded ink, #6B5C58.
// The dark mode's own third grade of text is too bright to read as
// disabled, so it is mixed back into the window.
func (t Tokens) disabledText() ui.Color {
	if t.Dark {
		return mix(t.TextTertiary, t.WinBg, 0.35)
	}
	return ui.Hex("#6B5C58")
}

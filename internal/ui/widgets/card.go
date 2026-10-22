package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// Card is a part of the window that stands on its own: the panel the body
// of the window sits on, a tip, a card of the overview.
//
// The design file's rule for a card is a translucent fill with a hairline
// around it. The card in the middle of the window is the roundest of them
// all, with corners of sixteen.
func Card(c *ui.Context, t design.Tokens, o CardOptions, children func()) ui.Element {
	surface := o.Surface
	if surface.Base == (ui.Color{}) {
		surface = t.Surfaces().Solid
	}
	padding := o.Padding
	if padding == [4]float32{} {
		padding = [4]float32{design.SpaceXL, design.SpaceXL, design.SpaceXL, design.SpaceXL}
	}
	radius := o.Radius
	if radius == 0 {
		radius = design.RadiusLG
	}
	element := ui.Column(c).
		Background(surface.Base).
		Radius(radius).
		Padding(padding[0], padding[1], padding[2], padding[3])
	if o.Border {
		border := o.BorderColor
		if border == (ui.Color{}) {
			border = surface.Border
		}
		element = element.Border(design.StrokeNormal, border)
	}
	return element.Children(children)
}

// CardOptions are the ways one card differs from another.
type CardOptions struct {
	// Surface is the step of the ladder the card lies on. When it is
	// left out, the card is solid.
	Surface design.Surface
	// Padding is the room inside the card: above, at the right, below
	// and at the left.
	Padding [4]float32
	// Radius rounds the corners of the card.
	Radius float32
	// Border draws a hairline around the card.
	Border bool
	// BorderColor is the color of the hairline. When it is left out, the
	// border of the surface is used.
	BorderColor ui.Color
}

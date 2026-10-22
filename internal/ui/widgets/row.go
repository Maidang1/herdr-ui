package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// Mark is the little dot at the left of a row of the tree.
//
// The design file's rule is that a row that is chosen wears a filled dot
// of the accent, and a row that is not wears an empty ring of the third
// grade of text.
type Mark int

const (
	// MarkNone is a row without a mark.
	MarkNone Mark = iota
	// MarkAccent is a filled dot of the accent.
	MarkAccent
	// MarkOutline is an empty ring.
	MarkOutline
)

// RowOptions are the ways one row of the tree differs from another.
type RowOptions struct {
	// Mark is the mark at the left of the row.
	Mark Mark
	// Selected tells that the row is the one that is chosen.
	Selected bool
	// Icon is the name of the icon of the thing the row is about.
	Icon string
	// Name is the name of the thing the row is about.
	Name string
	// Trailing is what the row says at its right.
	Trailing string
	// TrailingColor is the color of that.
	TrailingColor ui.Color
	// Indent is the room the row keeps at its left.
	Indent float32
	// Height is the height of the row.
	Height float32
	// Mono writes the name in the family of the terminal.
	Mono bool
	// Size is the size of the name.
	Size float32
	// OnClick is what happens when the row is clicked.
	OnClick func()
}

// TreeRow is a row of the tree of the sidebar: a mark, a name, and what
// else is known about the thing it is about.
//
// The design file's rule is that a row that is chosen wears the color of
// the selection, and that the name of it is in bold.
func TreeRow(c *ui.Context, t design.Tokens, o RowOptions) ui.Element {
	height := o.Height
	if height == 0 {
		height = design.WorkspaceRowHeight
	}
	size := o.Size
	if size == 0 {
		size = design.FontUI
	}
	trailingColor := o.TrailingColor
	if trailingColor == (ui.Color{}) {
		trailingColor = t.TextTertiary
	}
	element := ui.Row(c).
		Height(height).
		Padding(0, design.SpaceMD).
		Radius(design.RadiusMD).
		Gap(design.SpaceLG).
		AlignItems(ui.Center)
	if o.Selected {
		element = element.Background(t.Selection)
	}
	if o.Indent > 0 {
		element = element.PaddingStart(o.Indent)
	}
	if o.OnClick != nil {
		element = element.OnClick(o.OnClick)
	}
	return element.Children(func() {
		mark(c, t, o.Mark)
		if o.Icon != "" {
			Icon(c, t, o.Icon, design.IconSmall, t.TextTertiary)
		}
		nameWeight := design.WeightNormal
		if o.Selected {
			nameWeight = design.WeightMedium
		}
		Label(c, t, o.Name, LabelOptions{
			Size:   size,
			Color:  t.TextPrimary,
			Weight: nameWeight,
			Mono:   o.Mono,
		})
		if o.Trailing != "" {
			ui.Row(c).Grow(1)
			Label(c, t, o.Trailing, LabelOptions{
				Size:  design.FontBody,
				Color: trailingColor,
			})
		}
	})
}

// ChildRow is a row that stands under a row of the tree: the branch of a
// workspace, the ports it listens on, an agent that works in it.
type ChildRowOptions struct {
	// Icon is the name of the icon of the thing the row is about.
	Icon string
	// Text is what the row says.
	Text string
	// Color is the color of that.
	Color ui.Color
	// Indent is the room the row keeps at its left.
	Indent float32
	// Height is the height of the row.
	Height float32
	// Size is the size of the text.
	Size float32
	// Mono writes the text in the family of the terminal.
	Mono bool
}

// ChildRow draws a row that stands under a row of the tree.
func ChildRow(c *ui.Context, t design.Tokens, o ChildRowOptions) ui.Element {
	height := o.Height
	if height == 0 {
		height = design.AgentRowHeight
	}
	size := o.Size
	if size == 0 {
		size = design.FontBody
	}
	color := o.Color
	if color == (ui.Color{}) {
		color = t.TextTertiary
	}
	return ui.Row(c).
		Height(height).
		Padding(0, design.SpaceMD).
		PaddingStart(o.Indent).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Children(func() {
			if o.Icon != "" {
				Icon(c, t, o.Icon, design.IconSmall, color)
			}
			Label(c, t, o.Text, LabelOptions{
				Size:  size,
				Color: color,
				Mono:  o.Mono,
			})
		})
}

// mark draws the dot at the left of a row.
func mark(c *ui.Context, t design.Tokens, kind Mark) {
	switch kind {
	case MarkAccent:
		ui.Row(c).
			Size(7, 7).
			Radius(4).
			Background(t.Accent)
	case MarkOutline:
		ui.Row(c).
			Size(7, 7).
			Radius(4).
			Border(1.5, t.TextTertiary)
	}
}

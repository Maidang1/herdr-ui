// Package widgets holds the parts the window is made of.
//
// Every widget here reads the tokens of the design and nothing else: a
// widget that reaches for a color of its own is a widget that will not
// look right in the other mode.
package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// Tone is the sense of a color: the states a chip or a meter can mean.
type Tone int

const (
	// ToneNeutral is the sense of a thing that says nothing in
	// particular.
	ToneNeutral Tone = iota
	// ToneAccent is the sense of the thing the window is about.
	ToneAccent
	// ToneSuccess is the sense of what went well.
	ToneSuccess
	// ToneWarning is the sense of what wants attention.
	ToneWarning
	// ToneDanger is the sense of what went wrong.
	ToneDanger
	// ToneInfo is the sense of what is worth reading.
	ToneInfo
)

// color returns the color of a tone, and the color of the text that goes
// with it.
func (tone Tone) color(t design.Tokens) (fill, label ui.Color) {
	switch tone {
	case ToneAccent:
		return t.AccentWash, t.AccentText
	case ToneSuccess:
		return t.GreenWash, t.GreenText
	case ToneWarning:
		return t.YellowWash, t.YellowText
	case ToneDanger:
		return t.RedWash, t.RedText
	case ToneInfo:
		return t.BlueWash, t.Blue
	}
	return t.Chip, t.ChipText
}

// strong returns the color of a tone that is solid enough to carry white
// text: the color of a badge of the activity bar.
func (tone Tone) strong(t design.Tokens) ui.Color {
	switch tone {
	case ToneAccent:
		return t.AccentDeep
	case ToneSuccess:
		return t.GreenDeep
	case ToneWarning:
		return t.Yellow
	case ToneDanger:
		return t.Red
	case ToneInfo:
		return t.Blue
	}
	return t.KnobOff
}

// Label is a piece of text of the window.
//
// It is the one way the window writes text, so that every word of it has
// the family, the size and the color the design file says.
func Label(c *ui.Context, t design.Tokens, s string, o LabelOptions) ui.Element {
	element := ui.Text(c, s).
		Font(design.FamilyUI).
		FontSize(o.Size).
		TextColor(o.Color)
	if o.Weight != 0 {
		element = element.FontWeight(o.Weight)
	}
	if o.Spacing != 0 {
		element = element.LetterSpacing(o.Spacing)
	}
	if o.Mono {
		element = element.Font(design.FamilyMono)
	}
	return element
}

// LabelOptions are the ways a piece of text can differ from another.
type LabelOptions struct {
	// Size is the size of the text.
	Size float32
	// Color is the color of the text.
	Color ui.Color
	// Weight is the weight of the text.
	Weight int
	// Spacing is the space between the letters.
	Spacing float32
	// Mono writes the text in the family of the terminal.
	Mono bool
}

// text returns the options of the text of the window, with the size and
// the color the design file gives it.
func text(t design.Tokens, size float32, color ui.Color) LabelOptions {
	return LabelOptions{Size: size, Color: color}
}

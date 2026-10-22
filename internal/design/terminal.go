package design

import "github.com/egoist/mygo/ui"

// The palette of the terminal.
//
// The daemon says the color of a cell in one of three ways: the name of
// one of the sixteen colors, the number of one of the two hundred and
// fifty six, or the color itself. The sixteen are the ones of the design:
// the colors of the window that the terminal borrows, so that a program
// that paints itself green paints itself the green of the window.
//
// The order is the order of the terminal: black, red, green, yellow, blue,
// magenta, cyan, white, and then the same eight, brighter.
func (t Tokens) TerminalPalette() [16]ui.Color {
	return [16]ui.Color{
		// The eight ordinary colors.
		t.TextPrimary,
		t.Red,
		t.Green,
		t.Yellow,
		t.Blue,
		t.Mauve,
		t.Cyan,
		t.TextPrimary,
		// The eight brighter ones.
		t.TextTertiary,
		t.RedText,
		t.GreenText,
		t.YellowText,
		t.Blue,
		t.Mauve,
		t.Cyan,
		t.TextSecondary,
	}
}

// TerminalColor returns the color of a cell of the terminal.
//
// The daemon writes the color in one of three ways, told by the byte at
// the top of the number:
//
//	0 — the color is one of the sixteen, or the default
//	1 — the color is one of the two hundred and fifty six
//	2 — the color is itself
//
// The default is the color the terminal paints when the program says
// nothing.
func (t Tokens) TerminalColor(value uint32, defaultColor ui.Color) ui.Color {
	switch value >> 24 {
	case 0:
		if index := value & 0xFF; index >= 1 && index <= 16 {
			return t.TerminalPalette()[index-1]
		}
		return defaultColor
	case 1:
		palette := t.TerminalPalette()
		index := int(value & 0xFF)
		if index < len(palette) {
			return palette[index]
		}
		return defaultColor
	case 2:
		return ui.Color{
			R: uint8(value >> 16 & 0xFF),
			G: uint8(value >> 8 & 0xFF),
			B: uint8(value & 0xFF),
			A: 0xFF,
		}
	}
	return defaultColor
}

// The modifiers of a cell of the terminal.
//
// The daemon says them in the bits of a number.
const (
	// CellBold makes the character heavier.
	CellBold = 1 << 0
	// CellDim makes the character fainter.
	CellDim = 1 << 1
	// CellItalic makes the character lean.
	CellItalic = 1 << 2
	// CellUnderline underlines the character.
	CellUnderline = 1 << 3
	// CellReversed swaps the colors of the character and its ground.
	CellReversed = 1 << 6
	// CellHidden hides the character.
	CellHidden = 1 << 7
	// CellStrikeThrough strikes the character through.
	CellStrikeThrough = 1 << 8
)

// TerminalCellColors returns the color of the character of a cell and the
// color behind it.
//
// The rules are the rules of a terminal: a reversed cell swaps its two
// colors, a dim cell is half as bright, and a hidden cell takes the color
// of its ground.
func (t Tokens) TerminalCellColors(fg, bg uint32, modifier uint16) (ui.Color, ui.Color) {
	foreground := t.TerminalColor(fg, t.TextPrimary)
	background := t.TerminalColor(bg, t.TerminalBg)
	if modifier&CellReversed != 0 {
		foreground, background = background, foreground
	}
	if modifier&CellDim != 0 {
		foreground = mix(foreground, background, 0.5)
	}
	if modifier&CellHidden != 0 {
		foreground = background
	}
	return foreground, background
}

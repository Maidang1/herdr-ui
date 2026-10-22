package ui

import (
	"strings"

	"herdr-ui/internal/herdr"
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo/ui"
)

// The daemon paints the terminals and sends the cells. A cell carries the
// symbol and the colors the program asked for; the client draws them.

// The bits of a cell's modifier.
const (
	modifierBold          = 1
	modifierDim           = 1 << 1
	modifierItalic        = 1 << 2
	modifierUnderline     = 1 << 3
	modifierReversed      = 1 << 6
	modifierHidden        = 1 << 7
	modifierStrikethrough = 1 << 8
)

// The size of one cell, in DIPs.
const (
	// defaultFontSize is the size of the characters of the terminal when
	// the settings say nothing.
	defaultFontSize = 13
	// defaultRowHeight is the height of a row of the terminal when the
	// settings say nothing.
	defaultRowHeight = 18
	monoFont         = "Menlo"
)

// drawTerminal shows the terminals the daemon painted.
//
// Aiming at a pane focuses it, the way the multiplexer does: the pane under
// the pointer takes the keyboard.
func (a *App) drawTerminal(c *ui.Context, snapshot *herdr.Snapshot) {
	theme := c.Theme()
	// The terminal follows the theme the settings name, not the theme of
	// the window.
	colors := a.terminalColors(theme.Dark)
	box := ui.Box(c).Grow(1).Fill().Background(colors.Background)
	// The terminal takes the keyboard, the way a terminal does: the keys
	// the user presses go to the pane that has the focus.
	box.Focusable().FocusRing(false)
	// The keys of the keyboard type their characters, without an input
	// method in between.
	box.InputMethod(false)
	box.TextCaret(a.terminalCaret())
	box.HandleInput(func(event ui.InputEvent) bool {
		// The copy mode holds the keyboard while it is on: every key is
		// its own, and none reaches the pane.
		if a.copyModeActive() {
			switch event.Kind {
			case ui.InputKeyDown:
				return a.copyModeKey(event.Key, event.Mods)
			case ui.InputText:
				return a.copyModeText(event.Text)
			}
		}
		switch event.Kind {
		case ui.InputKeyDown:
			return a.terminalKey(event)
		case ui.InputText:
			return a.terminalText(event.Text)
		case ui.InputCommand:
			switch event.Text {
			case "copy":
				a.copySelection()
				return true
			case "paste":
				a.pasteIntoPane()
				return true
			}
			return false
		case ui.InputPointerDown:
			if event.Button != 0 {
				return false
			}
			// The command held opens the link under the pointer.
			if event.Mods&ui.Cmd != 0 {
				a.openLinkAt(event.X, event.Y)
				return true
			}
			// The button that tries the daemon again takes the click.
			// The pointer is counted from the terminal's own corner.
			box := a.terminalBounds()
			local := ui.Rect{X: 0, Y: 0, W: box.W, H: box.H}
			if rect, ok := a.reconnectButtonRect(local); ok &&
				event.X >= rect.X && event.X <= rect.X+rect.W &&
				event.Y >= rect.Y && event.Y <= rect.Y+rect.H {
				a.reconnectNow()
				return true
			}
			if rect, ok := a.annotationOfferRect(local, a.measuredCellWidth()); ok &&
				event.X >= rect.X && event.X <= rect.X+rect.W &&
				event.Y >= rect.Y && event.Y <= rect.Y+rect.H {
				a.openAnnotation()
				return true
			}
			a.mu.Lock()
			frame := a.surface
			cellWidth := a.cellWidth
			a.mu.Unlock()
			if cellWidth <= 0 {
				return false
			}
			paneID, ok := paneAt(frame, event.X, event.Y, cellWidth, a.rowHeightPx())
			if !ok {
				return false
			}
			point, ok := cellAt(frame, event.X, event.Y, cellWidth, a.rowHeightPx())
			if !ok {
				return false
			}
			a.focusPane(paneID)
			a.beginSelection(paneID, point)
			return true
		case ui.InputPointerMove:
			a.mu.Lock()
			frame := a.surface
			cellWidth := a.cellWidth
			dragging := a.selection.dragging
			a.mu.Unlock()
			if cellWidth <= 0 {
				return false
			}
			if !dragging {
				a.hoverLinkAt(event.X, event.Y)
				return false
			}
			point, ok := cellAt(frame, event.X, event.Y, cellWidth, a.rowHeightPx())
			if !ok {
				return true
			}
			a.extendSelection(point)
			return true
		case ui.InputPointerUp:
			a.mu.Lock()
			frame := a.surface
			cellWidth := a.cellWidth
			dragging := a.selection.dragging
			a.mu.Unlock()
			if !dragging || cellWidth <= 0 {
				return false
			}
			point, ok := cellAt(frame, event.X, event.Y, cellWidth, a.rowHeightPx())
			if !ok {
				a.releaseSelection(a.selectionOf().cursor)
				return true
			}
			a.releaseSelection(point)
			return true
		}
		return false
	})
	box.ContextMenu(a.paneMenu)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		a.mu.Lock()
		a.terminalRect = r
		a.mu.Unlock()
		a.drawGrid(p, r, colors)
		a.drawPaneHeaders(p, r, a.surface, snapshot, a.measuredCellWidth())
		a.drawFindHighlight(p, r, a.measuredCellWidth())
		a.drawSelection(p, r, a.measuredCellWidth())
		a.drawCopyMode(p, r, a.measuredCellWidth())
		a.drawLink(p, r, a.measuredCellWidth())
		a.drawAnnotationOffer(p, r, a.measuredCellWidth())
		a.drawReconnecting(p, r)
		a.drawToasts(p, r)
	})
}

// fontSizePx returns the size of the characters of the terminal.
func (a *App) fontSizePx() float32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.fontSize <= 0 {
		return defaultFontSize
	}
	return a.fontSize
}

// rowHeightPx returns the height of a row of the terminal.
func (a *App) rowHeightPx() float32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rowHeight <= 0 {
		return defaultRowHeight
	}
	return a.rowHeight
}

// uiFontSizePx returns the size of the characters of the rest of the
// window.
func (a *App) uiFontSizePx() float32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.uiFontSize <= 0 {
		return 12
	}
	return a.uiFontSize
}

// measuredCellWidth returns the width of one cell, as the last frame
// measured it.
func (a *App) measuredCellWidth() float32 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cellWidth
}

// terminalBounds says where the terminal stands in the window.
func (a *App) terminalBounds() ui.Rect {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.terminalRect
}

// terminalCaret says where the cursor of the terminal stands, in the box of
// the terminal.
//
// The caret is where the system puts the candidate window of an input
// method, and where the window shows the cursor of the pane.
func (a *App) terminalCaret() ui.Rect {
	a.mu.Lock()
	frame := a.surface
	cellWidth := a.cellWidth
	a.mu.Unlock()
	if frame == nil || frame.Grid.Cursor == nil {
		return ui.Rect{X: 0, Y: 0, W: estimatedCellWidth, H: a.rowHeightPx()}
	}
	if cellWidth <= 0 {
		cellWidth = estimatedCellWidth
	}
	return ui.Rect{
		X: float32(frame.Grid.Cursor.X) * cellWidth,
		Y: float32(frame.Grid.Cursor.Y) * a.rowHeightPx(),
		W: cellWidth,
		H: a.rowHeightPx(),
	}
}

// drawGrid draws the cells of the surface frame.
func (a *App) drawGrid(p *ui.Painter, r ui.Rect, colors terminalColors) {
	a.mu.Lock()
	frame := a.surface
	a.mu.Unlock()

	if frame == nil || len(frame.Grid.Cells) == 0 {
		p.Text(r.X+8, r.Y+20, "Waiting for the terminal…", 13, colors.Foreground)
		return
	}

	grid := frame.Grid
	width := int(grid.Width)
	height := int(grid.Height)
	if width <= 0 || height <= 0 {
		return
	}

	advance, _ := p.MeasureText(0, ui.Span{
		Text: strings.Repeat("M", 64),
		Font: monoFont,
		Size: a.fontSizePx(),
	})
	cellWidth := advance / 64
	if cellWidth <= 0 {
		cellWidth = 7
	}
	a.mu.Lock()
	a.cellWidth = cellWidth
	a.mu.Unlock()

	// The pane the daemon focuses shows a border, so the user sees where the
	// keys go.
	for _, pane := range frame.Panes {
		x := r.X + float32(pane.Rect.X)*cellWidth
		y := r.Y + float32(pane.Rect.Y)*a.rowHeightPx()
		w := float32(pane.Rect.Width) * cellWidth
		h := float32(pane.Rect.Height) * a.rowHeightPx()
		if pane.Focused {
			p.Fill(ui.Rect{X: x, Y: y, W: w, H: h}, ui.RGBA(120, 170, 255, 0.10), 0)
		}
	}

	for row := 0; row < height; row++ {
		y := r.Y + float32(row)*a.rowHeightPx()
		if y > r.Y+r.H {
			break
		}
		a.drawRow(p, r, grid, row, width, cellWidth, y, colors)
	}

	a.drawCursor(p, r, grid, cellWidth, colors)
}

// drawRow draws one line of cells.
func (a *App) drawRow(p *ui.Painter, r ui.Rect, grid herdr.Grid, row, width int, cellWidth, y float32, colors terminalColors) {
	spans, backgrounds := rowSpans(grid, row, width, a.fontSizePx(), colors)
	for _, background := range backgrounds {
		x := r.X + float32(background.Column)*cellWidth
		p.Fill(ui.Rect{X: x, Y: y, W: cellWidth, H: a.rowHeightPx()}, background.Color, 0)
	}
	if len(spans) == 0 {
		return
	}
	p.RichText(r.X, y+a.fontSizePx(), 0, spans...)
}

// background is one cell that the program gave a background of its own.
type background struct {
	Column int
	Color  ui.Color
}

// rowSpans turns one line of cells into the spans the window paints.
//
// Cells that share a color and a style become one span, so that a line of
// text is drawn in a few calls rather than one per cell.
func rowSpans(grid herdr.Grid, row, width int, fontSize float32, colors terminalColors) ([]ui.Span, []background) {
	var spans []ui.Span
	var backgrounds []background
	var runText strings.Builder
	var runColor ui.Color
	var runModifier uint16
	runValid := false

	flush := func() {
		if !runValid || runText.Len() == 0 {
			return
		}
		span := ui.Span{
			Text:  runText.String(),
			Font:  monoFont,
			Size:  fontSize,
			Color: runColor,
		}
		if runModifier&modifierBold != 0 {
			span.Weight = 700
		}
		if runModifier&modifierItalic != 0 {
			span.Italic = true
		}
		if runModifier&modifierUnderline != 0 {
			span.Underline = true
		}
		if runModifier&modifierStrikethrough != 0 {
			span.Strikethrough = true
		}
		spans = append(spans, span)
		runText.Reset()
		runValid = false
	}

	for column := 0; column < width; column++ {
		index := row*int(grid.Width) + column
		if index >= len(grid.Cells) {
			break
		}
		cell := grid.Cells[index]
		if cell.Skip {
			continue
		}
		fg, bg := cellColors(cell, colors.Foreground, colors.Background, &colors.Palette)
		style := cell.Modifier & styleBits
		if cell.Symbol != "" && cell.Symbol != " " {
			if !runValid || runColor != fg || runModifier != style {
				flush()
				runColor = fg
				runModifier = style
				runValid = true
			}
			runText.WriteString(cell.Symbol)
		} else {
			flush()
		}
		if bg != colors.Background {
			backgrounds = append(backgrounds, background{Column: column, Color: bg})
		}
	}
	flush()
	return spans, backgrounds
}

// drawCursor marks the cell the cursor sits in.
func (a *App) drawCursor(p *ui.Painter, r ui.Rect, grid herdr.Grid, cellWidth float32, colors terminalColors) {
	if grid.Cursor == nil || !grid.Cursor.Visible {
		return
	}
	index := int(grid.Cursor.Y)*int(grid.Width) + int(grid.Cursor.X)
	if index < 0 || index >= len(grid.Cells) {
		return
	}
	fg, _ := cellColors(grid.Cells[index], colors.Foreground, colors.Background, &colors.Palette)
	x := r.X + float32(grid.Cursor.X)*cellWidth
	y := r.Y + float32(grid.Cursor.Y)*a.rowHeightPx()
	p.Fill(ui.Rect{X: x, Y: y, W: cellWidth, H: a.rowHeightPx()}, ui.RGBA(fg.R, fg.G, fg.B, 0.55), 0)
}

// The colors of the terminal, when the daemon does not choose one.
//
// They come from the default theme, the way the Rust client does, and the
// theme the settings name replaces them.
var (
	defaultForeground = rgbOf(theme.Default().Foreground)
	defaultBackground = rgbOf(theme.Default().Background)
	palette16         = paletteOf16(theme.Default())
	palette256        = paletteOf256(theme.Default())
)

// defaultColors are the colors of the terminal when no theme is named.
func defaultColors() terminalColors {
	return terminalColors{
		Foreground: defaultForeground,
		Background: defaultBackground,
		Palette:    palette256,
	}
}

// paletteOf16 returns the sixteen named colors of a theme.
func paletteOf16(found theme.Theme) [16]ui.Color {
	var palette [16]ui.Color
	for index := 0; index < 16; index++ {
		palette[index] = rgbOf(found.Palette[index])
	}
	return palette
}

// paletteOf256 returns the palette of a theme.
func paletteOf256(found theme.Theme) [256]ui.Color {
	var palette [256]ui.Color
	for index, value := range found.Palette {
		palette[index] = rgbOf(value)
	}
	return palette
}

// cellColors works out the colors of one cell.
func cellColors(cell herdr.Cell, defaultFg, defaultBg ui.Color, palette *[256]ui.Color) (ui.Color, ui.Color) {
	fg := resolveColor(cell.FG, defaultFg, palette)
	bg := resolveColor(cell.BG, defaultBg, palette)
	if cell.Modifier&modifierReversed != 0 {
		fg, bg = bg, fg
	}
	if cell.Modifier&modifierDim != 0 {
		fg = ui.RGB(
			(fg.R+bg.R)/2,
			(fg.G+bg.G)/2,
			(fg.B+bg.B)/2,
		)
	}
	if cell.Modifier&modifierHidden != 0 {
		fg = bg
	}
	return fg, bg
}

// resolveColor turns the color the daemon sends into one the window paints.
//
// The high byte says what the color is: zero is a name from the palette of
// sixteen, one is an index of the palette of 256, two is a red, green and
// blue triple, and anything else is the color of the terminal itself.
func resolveColor(value uint32, fallback ui.Color, palette *[256]ui.Color) ui.Color {
	switch value >> 24 {
	case 0:
		index := value & 255
		if index >= 1 && index <= 16 {
			return palette[index-1]
		}
		return fallback
	case 1:
		return palette[value&255]
	case 2:
		return ui.RGB(uint8(value>>16), uint8(value>>8), uint8(value))
	default:
		return fallback
	}
}

// styleBits are the modifiers that change how a symbol looks.
const styleBits = modifierBold | modifierItalic | modifierUnderline | modifierStrikethrough

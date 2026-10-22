package ui

import (
	"strings"

	"herdr-ui/internal/design"
	"herdr-ui/internal/protocol"
	"herdr-ui/internal/ui/widgets"

	"github.com/egoist/mygo/ui"
)

// terminal draws the pane in front.
//
// The daemon sends the surface of the shell as a grid of cells: what
// character is in each cell, what color it is, and what the modifiers of
// the text are. The window draws that grid, and sends back what the user
// does in it.
func (a *App) terminal(c *ui.Context, t design.Tokens, live *Live) {
	surface := live.Surface()
	if surface == nil {
		a.sampleTerminal(c, t)
		return
	}
	rect, cells := focusedPane(surface)
	if cells == nil {
		a.sampleTerminal(c, t)
		return
	}

	element := ui.Column(c).
		Grow(1).
		Background(t.TerminalBg).
		Padding(design.TerminalPadY, design.TerminalPadX).
		Focusable().
		HandleInput(a.terminalInput)
	if live.Connected() {
		element = element.AutoFocus()
	}
	element.Children(func() {
		for y := 0; y < int(rect.Height); y++ {
			a.terminalRow(c, t, surface, rect, y)
		}
	})
}

// focusedPane returns the place of the pane in front and the cells of it.
func focusedPane(surface *protocol.SurfaceFrame) (protocol.Rect, []protocol.Cell) {
	for _, pane := range surface.Panes {
		if pane.Focused {
			return pane.InnerRect, cellsOf(surface, pane.InnerRect)
		}
	}
	if len(surface.Panes) > 0 {
		pane := surface.Panes[0]
		return pane.InnerRect, cellsOf(surface, pane.InnerRect)
	}
	return protocol.Rect{Width: surface.Width, Height: surface.Height}, surface.Cells
}

// cellsOf returns the cells of a place of the surface.
func cellsOf(surface *protocol.SurfaceFrame, rect protocol.Rect) []protocol.Cell {
	if surface.Width == 0 {
		return nil
	}
	cells := make([]protocol.Cell, 0, rect.Width*rect.Height)
	for y := 0; y < int(rect.Height); y++ {
		row := int(rect.Y) + y
		if row < 0 || row >= int(surface.Height) {
			continue
		}
		start := row * int(surface.Width)
		if start >= len(surface.Cells) {
			continue
		}
		end := start + int(surface.Width)
		if end > len(surface.Cells) {
			end = len(surface.Cells)
		}
		line := surface.Cells[start:end]
		from := int(rect.X)
		if from > len(line) {
			continue
		}
		to := from + int(rect.Width)
		if to > len(line) {
			to = len(line)
		}
		cells = append(cells, line[from:to]...)
	}
	return cells
}

// terminalRow draws one row of the terminal.
//
// The cells that are painted the same are drawn together, for a row of a
// terminal is a few pieces of text and not a thousand.
func (a *App) terminalRow(c *ui.Context, t design.Tokens, surface *protocol.SurfaceFrame, rect protocol.Rect, y int) {
	row := cellsOf(surface, protocol.Rect{X: rect.X, Y: rect.Y + uint16(y), Width: rect.Width, Height: 1})
	if len(row) == 0 {
		ui.Row(c).Height(design.TerminalLineHeight)
		return
	}
	cursorHere := surface.Cursor != nil && surface.Cursor.Visible &&
		int(surface.Cursor.Y) == int(rect.Y)+y

	ui.Row(c).
		Height(design.TerminalLineHeight).
		Font(design.FamilyMono).
		FontSize(design.TerminalFontSize).
		AlignItems(ui.Center).
		Children(func() {
			start := 0
			for start < len(row) {
				foreground, background := t.TerminalCellColors(row[start].Fg, row[start].Bg, row[start].Modifier)
				if cursorHere && start == int(surface.Cursor.X)-int(rect.X) {
					background = t.TextTertiary
					foreground = t.TerminalBg
				}
				end := start
				var text strings.Builder
				for end < len(row) {
					other, otherBackground := t.TerminalCellColors(row[end].Fg, row[end].Bg, row[end].Modifier)
					if other != foreground || otherBackground != background {
						break
					}
					if row[end].Skip {
						end++
						continue
					}
					text.WriteString(row[end].Symbol)
					end++
				}
				if text.Len() > 0 {
					ui.Row(c).
						Background(background).
						Children(func() {
							ui.Text(c, text.String()).
								Font(design.FamilyMono).
								FontSize(design.TerminalFontSize).
								TextColor(foreground)
						})
				}
				if end == start {
					end++
				}
				start = end
			}
		})
}

// sampleTerminal draws the sample session, when the daemon is not there.
func (a *App) sampleTerminal(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Grow(1).
		Background(t.TerminalBg).
		Padding(design.TerminalPadY, design.TerminalPadX).
		Children(func() {
			for _, line := range a.session.Lines {
				ui.Row(c).
					Height(design.TerminalLineHeight).
					AlignItems(ui.Center).
					Children(func() {
						for _, segment := range line {
							widgets.Label(c, t, segment.Text, widgets.LabelOptions{
								Size:   design.TerminalFontSize,
								Color:  roleColor(t, segment.Role),
								Weight: segment.Weight,
								Mono:   true,
							})
						}
					})
			}
		})
}

// terminalInput takes what the user does in the terminal and gives it to
// the daemon.
//
// The rules are the rules of a terminal: the keys that have a name are
// sent as keys, and the letters are sent as text, for a letter belongs to
// the keyboard's language and not to the window.
func (a *App) terminalInput(ev ui.InputEvent) bool {
	c := a.live.Client()
	if c == nil {
		return false
	}
	paneID := a.live.FocusedPaneID()
	if paneID == "" {
		return false
	}
	switch ev.Kind {
	case ui.InputKeyDown:
		code, ok := keyCode(ev.Key)
		if !ok {
			return false
		}
		kind := uint8(protocol.KeyPress)
		if ev.Repeat {
			kind = protocol.KeyRepeat
		}
		c.Input(paneID, &protocol.KeyEvent{
			Code:       code,
			Modifiers:  modifierBits(ev.Mods),
			Kind:       kind,
			RepeatCount: 1,
		})
		return true
	case ui.InputText:
		if ev.Text == "" {
			return false
		}
		c.Input(paneID, &protocol.TextCommit{Text: ev.Text})
		return true
	}
	return false
}

// modifierBits returns the modifiers of a key, as the daemon counts them.
func modifierBits(mods ui.Modifiers) uint8 {
	var bits uint8
	if mods&ui.Shift != 0 {
		bits |= protocol.ModShift
	}
	if mods&ui.Ctrl != 0 {
		bits |= protocol.ModCtrl
	}
	if mods&ui.Alt != 0 {
		bits |= protocol.ModAlt
	}
	return bits
}

// keyCode returns the key of a key that has a name.
//
// A letter has no name here: it is text, and the daemon wants the text.
func keyCode(key ui.Key) (protocol.KeyCode, bool) {
	switch key {
	case ui.KeyEnter:
		return protocol.KeyEnter, true
	case ui.KeyBackspace:
		return protocol.KeyBackspace, true
	case ui.KeyTab:
		return protocol.KeyTab, true
	case ui.KeyEscape:
		return protocol.KeyEsc, true
	case ui.KeyLeft:
		return protocol.KeyLeft, true
	case ui.KeyRight:
		return protocol.KeyRight, true
	case ui.KeyUp:
		return protocol.KeyUp, true
	case ui.KeyDown:
		return protocol.KeyDown, true
	case ui.KeyHome:
		return protocol.KeyHome, true
	case ui.KeyEnd:
		return protocol.KeyEnd, true
	case ui.KeyPageUp:
		return protocol.KeyPageUp, true
	case ui.KeyPageDown:
		return protocol.KeyPageDown, true
	case ui.KeyDelete:
		return protocol.KeyDelete, true
	case ui.KeyInsert:
		return protocol.KeyInsert, true
	case ui.KeyF1:
		return protocol.FunctionKey(1), true
	case ui.KeyF2:
		return protocol.FunctionKey(2), true
	case ui.KeyF3:
		return protocol.FunctionKey(3), true
	case ui.KeyF4:
		return protocol.FunctionKey(4), true
	case ui.KeyF5:
		return protocol.FunctionKey(5), true
	case ui.KeyF6:
		return protocol.FunctionKey(6), true
	case ui.KeyF7:
		return protocol.FunctionKey(7), true
	case ui.KeyF8:
		return protocol.FunctionKey(8), true
	case ui.KeyF9:
		return protocol.FunctionKey(9), true
	case ui.KeyF10:
		return protocol.FunctionKey(10), true
	case ui.KeyF11:
		return protocol.FunctionKey(11), true
	case ui.KeyF12:
		return protocol.FunctionKey(12), true
	}
	return protocol.KeyCode{}, false
}

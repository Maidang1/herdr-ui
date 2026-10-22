package ui

import (
	"fmt"
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// The user chooses cells of a pane with the pointer and copies them.
//
// The daemon paints every pane of the tab into one grid of cells, so a
// selection is a stretch of that grid. What the selection reads is taken
// from the cells the window has painted; when the daemon has moved on and
// the painted cells no longer hold the text, the window asks the daemon to
// read the stretch of the scrollback instead.

const (
	// modifierShift, modifierControl and modifierAlt are the modifiers the
	// pointer and the keyboard carry.
	modifierShift   = 1 << 0
	modifierControl = 1 << 1
	modifierAlt     = 1 << 2
	modifierSuper   = 1 << 3
)

// gridPoint is a place in the grid of cells the daemon painted.
type gridPoint struct {
	Row int
	Col int
}

// selectionState is the selection the user is making, or the one they made
// and the window keeps highlighted.
type selectionState struct {
	// dragging is true while the pointer is down and choosing cells.
	dragging bool
	// retained is true when the selection was released and stays chosen.
	retained bool
	// paneID is the pane the selection belongs to.
	paneID string
	// anchor and cursor are the two ends of the selection, in the grid.
	anchor gridPoint
	cursor gridPoint
	// text is what the selection read.
	text string
}

// cellAt says which cell of the grid a point of the window falls on.
func cellAt(frame *herdr.SurfaceFrame, x, y, cellWidth, rowHeight float32) (gridPoint, bool) {
	if frame == nil || cellWidth <= 0 {
		return gridPoint{}, false
	}
	column := int(x / cellWidth)
	row := int(y / rowHeight)
	if row < 0 || row >= int(frame.Grid.Height) {
		return gridPoint{}, false
	}
	if column < 0 || column >= int(frame.Grid.Width) {
		return gridPoint{}, false
	}
	return gridPoint{Row: row, Col: column}, true
}

// paneRegion says which cells of the grid a pane holds.
func paneRegion(pane herdr.PaneSurface) (left, top, right, bottom int) {
	inner := pane.InnerRect
	left = int(inner.X)
	top = int(inner.Y)
	right = left + int(inner.Width) - 1
	bottom = top + int(inner.Height) - 1
	return left, top, right, bottom
}

// paneHolds says whether a cell of the grid belongs to a pane.
func paneHolds(pane herdr.PaneSurface, point gridPoint) bool {
	left, top, right, bottom := paneRegion(pane)
	return point.Row >= top && point.Row <= bottom && point.Col >= left && point.Col <= right
}

// beginSelection starts a selection in a pane.
//
// A press outside the painted cells of a pane clears the selection instead:
// there is nothing there to choose.
func (a *App) beginSelection(paneID string, point gridPoint) {
	frame := a.surfaceOf()
	pane, ok := a.surfacePane(paneID)
	if !ok || frame == nil || !paneHolds(pane, point) {
		a.clearSelection()
		return
	}
	a.mu.Lock()
	a.selection.dragging = true
	a.selection.retained = false
	a.selection.paneID = paneID
	a.selection.anchor = point
	a.selection.cursor = point
	a.selection.text = ""
	a.mu.Unlock()
	a.invalidate()
}

// extendSelection moves the end of the selection.
func (a *App) extendSelection(point gridPoint) {
	a.mu.Lock()
	if !a.selection.dragging {
		a.mu.Unlock()
		return
	}
	a.selection.cursor = point
	a.mu.Unlock()
	a.invalidate()
}

// releaseSelection ends the drag and copies what it chose.
//
// The copy follows the daemon's own habit: what the user selects is copied
// when the pointer comes up, and the highlight stays so that the copy can
// be made again.
func (a *App) releaseSelection(point gridPoint) {
	text := a.endSelection(point)
	if text == "" {
		return
	}
	a.copyText(text)
	a.offerAnnotation()
}

// endSelection ends the drag and keeps what it chose.
//
// The text is returned so that the caller can copy it, and so that a test
// can read the selection without touching the clipboard.
func (a *App) endSelection(point gridPoint) string {
	a.mu.Lock()
	if !a.selection.dragging {
		a.mu.Unlock()
		return ""
	}
	a.selection.dragging = false
	a.selection.cursor = point
	paneID := a.selection.paneID
	anchor := a.selection.anchor
	cursor := a.selection.cursor
	a.mu.Unlock()

	text := a.readSelectionText(paneID, anchor, cursor)
	a.mu.Lock()
	a.selection.text = text
	// The selection stays highlighted after it was copied, unless the
	// settings say it should not.
	a.selection.retained = text != "" && !a.dropSelection
	a.mu.Unlock()
	a.invalidate()
	return text
}

// copyText puts text on the clipboard.
func (a *App) copyText(text string) {
	if err := a.writeToClipboard(text); err != nil {
		a.setStatus("could not copy: %v", err)
		return
	}
	a.setStatus("copied %d characters", len(text))
}

// readSelectionText reads the text of a selection.
//
// The cells the window has painted come first: they are what the user saw.
// When they hold nothing, the daemon is asked for the stretch of the
// scrollback the selection covers, which is what it keeps of the pane.
func (a *App) readSelectionText(paneID string, anchor, cursor gridPoint) string {
	frame := a.surfaceOf()
	if frame != nil {
		if pane, ok := a.surfacePane(paneID); ok {
			if text := selectionText(frame.Grid, pane, anchor, cursor); text != "" {
				return text
			}
		}
	}
	text, err := a.readSelectionFromDaemon(paneID, anchor, cursor)
	if err != nil {
		return ""
	}
	return text
}

// selectionText reads the cells a selection covers.
//
// The selection runs from the earlier of its two ends to the later, and
// every row it crosses is read from the pane's first column to its last,
// except the first and the last row, which start and end where the
// selection does.
func selectionText(grid herdr.Grid, pane herdr.PaneSurface, anchor, cursor gridPoint) string {
	start, end := anchor, cursor
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		start, end = end, start
	}
	left, top, right, bottom := paneRegion(pane)
	if start.Row < top {
		start.Row = top
	}
	if end.Row > bottom {
		end.Row = bottom
	}
	if start.Col < left {
		start.Col = left
	}
	if end.Col > right {
		end.Col = right
	}
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		return ""
	}
	// A press that never left the cell it began in chooses nothing, the
	// way a terminal emulator reads a plain click.
	if start == end {
		return ""
	}

	width := int(grid.Width)
	var lines []string
	for row := start.Row; row <= end.Row; row++ {
		from, to := left, right
		if row == start.Row {
			from = start.Col
		}
		if row == end.Row {
			to = end.Col
		}
		if to < from {
			to = from
		}
		var line strings.Builder
		for column := from; column <= to; column++ {
			index := row*width + column
			if index < 0 || index >= len(grid.Cells) {
				break
			}
			cell := grid.Cells[index]
			if cell.Skip {
				continue
			}
			line.WriteString(cell.Symbol)
		}
		lines = append(lines, strings.TrimRight(line.String(), " "))
	}
	// The rows the selection ends on may hold nothing but blanks.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// readSelectionFromDaemon asks the daemon for the text of a selection.
//
// The daemon counts the rows of a pane from the top of its scrollback, so
// the rows of the grid are turned into the rows of the scrollback before
// the question is asked.
func (a *App) readSelectionFromDaemon(paneID string, anchor, cursor gridPoint) (string, error) {
	if a.endpoint == nil {
		return "", fmt.Errorf("herdr-ui: not connected to the daemon")
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return "", fmt.Errorf("herdr-ui: the pane is not on screen")
	}
	left, top, _, _ := paneRegion(pane)
	base := viewportTop(pane) - top
	absolute := func(point gridPoint) TextPoint {
		return TextPoint{
			Row: base + point.Row,
			Col: point.Col - left,
		}
	}
	start := absolute(anchor)
	end := absolute(cursor)
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		start, end = end, start
	}

	var result struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	ctx, cancel := contextWithTimeout()
	defer cancel()
	err := a.endpoint.Request(ctx, "pane.selection.read", map[string]any{
		"pane_id": paneID,
		"anchor":  map[string]any{"row": start.Row, "col": start.Col},
		"cursor":  map[string]any{"row": end.Row, "col": end.Col},
	}, &result)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// copySelection copies the selection the window kept, and says whether it
// had one.
func (a *App) copySelection() bool {
	a.mu.Lock()
	text := a.selection.text
	retained := a.selection.retained
	a.mu.Unlock()
	if !retained || text == "" {
		return false
	}
	a.copyText(text)
	return true
}

// writeToClipboard puts text on the clipboard.
func (a *App) writeToClipboard(text string) error {
	if a.clipboardWriter != nil {
		return a.clipboardWriter(text)
	}
	return mygo.Clipboard.Write(transfer.TextData(text))
}

// clearSelection drops the selection.
func (a *App) clearSelection() {
	a.mu.Lock()
	a.selection.dragging = false
	a.selection.retained = false
	a.selection.text = ""
	a.mu.Unlock()
	a.invalidate()
}

// selectionOf returns the selection the window holds.
func (a *App) selectionOf() selectionState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.selection
}

// drawSelection marks the cells the user chose.
func (a *App) drawSelection(p *ui.Painter, r ui.Rect, cellWidth float32) {
	selection := a.selectionOf()
	if !selection.dragging && !selection.retained {
		return
	}
	frame := a.surfaceOf()
	if frame == nil {
		return
	}
	pane, ok := a.surfacePane(selection.paneID)
	if !ok {
		return
	}
	start, end := selection.anchor, selection.cursor
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		start, end = end, start
	}
	left, top, right, bottom := paneRegion(pane)
	if start.Row < top {
		start.Row = top
	}
	if end.Row > bottom {
		end.Row = bottom
	}
	if start.Col < left {
		start.Col = left
	}
	if end.Col > right {
		end.Col = right
	}
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		return
	}
	for row := start.Row; row <= end.Row; row++ {
		from, to := left, right
		if row == start.Row {
			from = start.Col
		}
		if row == end.Row {
			to = end.Col
		}
		if to < from {
			to = from
		}
		x := r.X + float32(from)*cellWidth
		y := r.Y + float32(row)*a.rowHeightPx()
		width := float32(to-from+1) * cellWidth
		p.Fill(ui.Rect{X: x, Y: y, W: width, H: a.rowHeightPx()}, ui.RGBA(120, 170, 255, 0.30), 0)
	}
}

// selectedText returns the text the user chose.
func (a *App) selectedText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return strings.TrimRight(a.selection.text, "\n")
}

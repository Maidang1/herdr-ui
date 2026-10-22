package ui

import (
	"os"
	"path/filepath"
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The links the daemon reads out of its own terminal state.
//
// `pane.link.resolve` names the cells a link covers, including the rows a
// soft-wrapped address continues on, and `pane.link.activate` lets a plugin
// claim the click before the window opens the address itself.
//
// Both are optional: a daemon that does not serve them keeps the cells that
// carry a link of their own, which the window reads from the surface.

// linkRegion is a stretch of cells a link covers, in the pane's viewport.
type linkRegion struct {
	row      int
	startCol int
	endCol   int
}

// linkState is the link the pointer is over.
type linkState struct {
	// paneID is the pane the link is in.
	paneID string
	// regions are the cells the link covers.
	regions []linkRegion
	// url is the address the daemon read, when it read one.
	url string
	// row and column are the cell the pointer was over when the window
	// last asked the daemon about it, so that a pointer that moves over
	// the same cell does not ask again.
	row, column int
	// asked says a question is on its way to the daemon.
	asked bool
}

// linkOfCell says whether a cell of the grid carries a link of its own.
func linkOfCell(frame *herdr.SurfaceFrame, row, column int) bool {
	if frame == nil {
		return false
	}
	index := row*int(frame.Grid.Width) + column
	if index < 0 || index >= len(frame.Grid.Cells) {
		return false
	}
	return frame.Grid.Cells[index].Link != nil
}

// resolveLink asks the daemon which cells a link covers.
//
// The pointer moves over many cells a second; the window asks once per cell,
// and not at all while a question is still on its way.
func (a *App) resolveLink(paneID string, row, column int) {
	if a.endpoint == nil {
		return
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return
	}
	a.mu.Lock()
	if a.link.asked || (a.link.paneID == paneID && a.link.row == row && a.link.column == column) {
		a.mu.Unlock()
		return
	}
	a.link.asked = true
	a.link.paneID = paneID
	a.link.row = row
	a.link.column = column
	a.mu.Unlock()
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		var result struct {
			Type    string       `json:"type"`
			Regions []linkRegion `json:"regions"`
		}
		err := a.endpoint.Request(ctx, "pane.link.resolve", map[string]any{
			"pane_id":          paneID,
			"viewport_row":     row,
			"col":              column,
			"content_revision": pane.ContentRevision,
		}, &result)
		a.mu.Lock()
		a.link.asked = false
		if err != nil || len(result.Regions) == 0 {
			a.link.regions = nil
			a.mu.Unlock()
			return
		}
		a.link.paneID = paneID
		a.link.regions = result.Regions
		a.mu.Unlock()
		a.invalidate()
	}()
}

// clearLink drops the link the pointer was over.
func (a *App) clearLink() {
	a.mu.Lock()
	changed := len(a.link.regions) > 0
	a.link.regions = nil
	a.link.url = ""
	a.link.row = -1
	a.link.column = -1
	a.mu.Unlock()
	if changed {
		a.invalidate()
	}
}

// activateLink asks the daemon to open the link under the pointer.
//
// A plugin's link handler may claim the click; when none does, the daemon
// answers with the address it read, and the window opens it.
func (a *App) activateLink(paneID string, row, column int) {
	if a.endpoint == nil {
		return
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		var result struct {
			Type    string  `json:"type"`
			Handled bool    `json:"handled"`
			URL     *string `json:"url"`
		}
		err := a.endpoint.Request(ctx, "pane.link.activate", map[string]any{
			"pane_id":          paneID,
			"viewport_row":     row,
			"col":              column,
			"content_revision": pane.ContentRevision,
		}, &result)
		if err != nil {
			a.setStatus("link: %v", err)
			return
		}
		if result.Handled {
			a.setStatus("the link was opened by a plugin")
			return
		}
		url := ""
		if result.URL != nil {
			url = *result.URL
		}
		if url == "" {
			// The daemon did not read an address; the window looks at
			// the text of the cell itself.
			url = a.linkTextAt(paneID, row, column)
		}
		if url == "" {
			a.setStatus("there is no link here")
			return
		}
		if _, err := a.OpenPage(url); err != nil {
			a.setStatus("could not open the link: %v", err)
			return
		}
		a.setStatus("opened %s", url)
	}()
}

// linkTextAt reads the address a cell holds, for the daemon that does not
// read the links itself.
func (a *App) linkTextAt(paneID string, row, column int) string {
	frame := a.surfaceOf()
	if frame == nil {
		return ""
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return ""
	}
	left, top, _, _ := paneRegion(pane)
	if row < top {
		return ""
	}
	width := int(frame.Grid.Width)
	// The word around the cell.
	start, end := column, column
	for start > left {
		index := row*width + start - 1
		if index < 0 || index >= len(frame.Grid.Cells) {
			break
		}
		symbol := frame.Grid.Cells[index].Symbol
		if symbol == "" || symbol == " " {
			break
		}
		start--
	}
	for end < width-1 {
		index := row*width + end + 1
		if index < 0 || index >= len(frame.Grid.Cells) {
			break
		}
		symbol := frame.Grid.Cells[index].Symbol
		if symbol == "" || symbol == " " {
			break
		}
		end++
	}
	var word strings.Builder
	for column := start; column <= end; column++ {
		index := row*width + column
		if index < 0 || index >= len(frame.Grid.Cells) {
			break
		}
		word.WriteString(frame.Grid.Cells[index].Symbol)
	}
	text := word.String()
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text
	}
	// A path on this machine opens in a page of its own, the way the
	// browser page serves the file it is given.
	if path := linkPathAt(text); path != "" {
		return path
	}
	return ""
}

// linkPathAt says the word is a path that exists on this machine.
func linkPathAt(word string) string {
	if word == "" || strings.ContainsAny(word, " \t") {
		return ""
	}
	if !strings.HasPrefix(word, "/") && !strings.HasPrefix(word, "./") && !strings.HasPrefix(word, "~/") {
		return ""
	}
	path := word
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, path[2:])
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if _, err := os.Stat(absolute); err != nil {
		return ""
	}
	return absolute
}

// drawLink underlines the cells the link covers.
func (a *App) drawLink(p *ui.Painter, r ui.Rect, cellWidth float32) {
	a.mu.Lock()
	link := a.link
	a.mu.Unlock()
	if len(link.regions) == 0 {
		return
	}
	pane, ok := a.surfacePane(link.paneID)
	if !ok {
		return
	}
	left, top, _, _ := paneRegion(pane)
	for _, region := range link.regions {
		row := region.row + top
		from := region.startCol + left
		to := region.endCol + left
		if to < from {
			continue
		}
		x := r.X + float32(from)*cellWidth
		y := r.Y + float32(row)*a.rowHeightPx() + a.rowHeightPx() - 2
		width := float32(to-from+1) * cellWidth
		p.Fill(ui.Rect{X: x, Y: y, W: width, H: 1}, ui.RGBA(120, 170, 255, 0.85), 0)
	}
}

// hoverLinkAt asks the daemon about the link under the pointer.
func (a *App) hoverLinkAt(x, y float32) {
	a.mu.Lock()
	frame := a.surface
	cellWidth := a.cellWidth
	a.mu.Unlock()
	if frame == nil || cellWidth <= 0 {
		return
	}
	point, ok := cellAt(frame, x, y, cellWidth, a.rowHeightPx())
	if !ok {
		a.clearLink()
		return
	}
	paneID, ok := paneAt(frame, x, y, cellWidth, a.rowHeightPx())
	if !ok {
		a.clearLink()
		return
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		a.clearLink()
		return
	}
	left, top, _, _ := paneRegion(pane)
	row := point.Row - top
	if row < 0 {
		a.clearLink()
		return
	}
	a.resolveLink(paneID, row, point.Col-left)
}

// openLinkAt opens the link under the pointer.
func (a *App) openLinkAt(x, y float32) {
	a.mu.Lock()
	frame := a.surface
	cellWidth := a.cellWidth
	a.mu.Unlock()
	if frame == nil || cellWidth <= 0 {
		return
	}
	point, ok := cellAt(frame, x, y, cellWidth, a.rowHeightPx())
	if !ok {
		return
	}
	paneID, ok := paneAt(frame, x, y, cellWidth, a.rowHeightPx())
	if !ok {
		return
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return
	}
	left, top, _, _ := paneRegion(pane)
	a.activateLink(paneID, point.Row-top, point.Col-left)
}

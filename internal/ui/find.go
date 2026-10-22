package ui

import (
	"fmt"
	"strings"
	"time"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The find bar searches the output of a pane.
//
// The daemon keeps the scrollback of every pane and searches it: the rows
// it answers with are absolute, counted from the top of the scrollback, so
// a match keeps its place while the pane scrolls. The daemon answers with
// the revision of the content it searched, and refuses the search when the
// terminal has moved on since, so the window tries again with the newer
// revision it has painted.
//
// The window keeps a search of its own as a fallback, over the cells it has
// painted, for a daemon that does not serve the search.

// TextPoint is a place in the text of a pane.
type TextPoint struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// TextRange is a stretch of text between two points. The end is inclusive.
type TextRange struct {
	Start TextPoint `json:"start"`
	End   TextPoint `json:"end"`
}

// findResult is what the daemon answers a search with.
type findResult struct {
	Type            string      `json:"type"`
	PaneID          string      `json:"pane_id"`
	ContentRevision uint64      `json:"content_revision"`
	Matches         []TextRange `json:"matches"`
	Total           int         `json:"total"`
	Current         int         `json:"current"`
	CurrentGlobal   int         `json:"current_global"`
}

// findState is what the find bar holds.
type findState struct {
	open    bool
	query   string
	paneID  string
	matches []TextRange
	current int
	total   int
	local   bool
}

// gridText returns the text of the grid, one string per row.
func gridText(grid herdr.Grid) []string {
	if len(grid.Cells) == 0 || grid.Width == 0 {
		return nil
	}
	width := int(grid.Width)
	rows := make([]string, 0, grid.Height)
	for row := 0; row < int(grid.Height); row++ {
		var line strings.Builder
		for column := 0; column < width; column++ {
			index := row*width + column
			if index >= len(grid.Cells) {
				break
			}
			cell := grid.Cells[index]
			if cell.Skip {
				continue
			}
			line.WriteString(cell.Symbol)
		}
		rows = append(rows, line.String())
	}
	return rows
}

// findText searches the text of the grid and returns where the text stands.
func findText(rows []string, query string) []TextRange {
	if query == "" {
		return nil
	}
	lower := strings.ToLower(query)
	var matches []TextRange
	for row, line := range rows {
		lowerLine := strings.ToLower(line)
		from := 0
		for {
			index := strings.Index(lowerLine[from:], lower)
			if index < 0 {
				break
			}
			start := from + index
			matches = append(matches, TextRange{
				Start: TextPoint{Row: row, Col: start},
				End:   TextPoint{Row: row, Col: start + len(query) - 1},
			})
			from = start + len(query)
			if from >= len(lowerLine) {
				break
			}
		}
	}
	return matches
}

// viewportTop says which absolute row the pane shows at its top.
func viewportTop(pane herdr.PaneSurface) int {
	if pane.Scroll == nil {
		return 0
	}
	top := int(pane.Scroll.MaxOffsetFromBottom) - int(pane.Scroll.OffsetFromBottom)
	if top < 0 {
		return 0
	}
	return top
}

// revealOffset says how far the pane should scroll for a match to show,
// or that the match already shows.
func revealOffset(pane herdr.PaneSurface, match TextRange) (int, bool) {
	if pane.Scroll == nil {
		return 0, false
	}
	height := int(pane.InnerRect.Height)
	top := viewportTop(pane)
	if match.Start.Row >= top && match.End.Row < top+height {
		return 0, false
	}
	wantedTop := match.Start.Row - height/2
	if wantedTop < 0 {
		wantedTop = 0
	}
	offset := int(pane.Scroll.MaxOffsetFromBottom) - wantedTop
	if offset < 0 {
		offset = 0
	}
	if offset > int(pane.Scroll.MaxOffsetFromBottom) {
		offset = int(pane.Scroll.MaxOffsetFromBottom)
	}
	return offset, true
}

// find searches the output of a pane and keeps what it finds.
func (a *App) find(paneID, query, direction string) {
	if paneID == "" || query == "" {
		return
	}
	matches, total, current, local, err := a.searchDaemon(paneID, query, direction, nil)
	if err != nil {
		// The daemon does not serve the search, or the pane moved on too
		// often; the window searches the cells it has painted instead.
		matches, total = a.searchLocal(paneID, query)
		current, local = 0, true
	}
	if local {
		a.setStatus("found %d matches in view", total)
	}

	a.mu.Lock()
	a.finder.open = true
	a.finder.query = query
	a.finder.paneID = paneID
	a.finder.matches = matches
	a.finder.total = total
	a.finder.current = current
	a.finder.local = local
	a.mu.Unlock()
	a.invalidate()
}

// searchDaemon asks the daemon to search the scrollback of a pane.
//
// The daemon refuses a search whose revision is old, so the window tries
// again with the newer revision it has painted, a few times over.
func (a *App) searchDaemon(paneID, query, direction string, previous *TextRange) ([]TextRange, int, int, bool, error) {
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(120 * time.Millisecond)
		}
		pane, ok := a.surfacePane(paneID)
		if !ok {
			return nil, 0, 0, false, fmt.Errorf("herdr-ui: the pane is not on screen")
		}
		cursor := a.absoluteCursor(pane)
		params := map[string]any{
			"pane_id":          paneID,
			"query":            query,
			"direction":        direction,
			"cursor":           map[string]any{"row": cursor.Row, "col": cursor.Col},
			"content_revision": pane.ContentRevision,
		}
		if previous != nil {
			params["previous"] = map[string]any{
				"start": map[string]any{"row": previous.Start.Row, "col": previous.Start.Col},
				"end":   map[string]any{"row": previous.End.Row, "col": previous.End.Col},
			}
		}
		var result findResult
		ctx, cancel := contextWithTimeout()
		err := a.endpoint.Request(ctx, "pane.copy_search", params, &result)
		cancel()
		if err != nil {
			lastErr = err
			if strings.Contains(err.Error(), "stale_content") {
				continue
			}
			return nil, 0, 0, false, err
		}
		current := result.Current
		if current < 0 || current >= len(result.Matches) {
			current = 0
		}
		return result.Matches, result.Total, current, false, nil
	}
	return nil, 0, 0, false, lastErr
}

// searchLocal searches the cells the window has painted.
func (a *App) searchLocal(paneID, query string) ([]TextRange, int) {
	a.mu.Lock()
	frame := a.surface
	a.mu.Unlock()
	if frame == nil {
		return nil, 0
	}
	top := 0
	for _, pane := range frame.Panes {
		if pane.PaneID == paneID {
			top = viewportTop(pane)
		}
	}
	rows := gridText(frame.Grid)
	matches := findText(rows, query)
	// The rows of the grid are the rows of the pane, counted from the top
	// of what the pane shows.
	for i := range matches {
		matches[i].Start.Row += top
		matches[i].End.Row += top
	}
	return matches, len(matches)
}

// absoluteCursor says where the cursor of a pane stands in its scrollback.
func (a *App) absoluteCursor(pane herdr.PaneSurface) TextPoint {
	a.mu.Lock()
	frame := a.surface
	a.mu.Unlock()
	if frame == nil || frame.Grid.Cursor == nil {
		return TextPoint{Row: viewportTop(pane), Col: 0}
	}
	return TextPoint{
		Row: viewportTop(pane) + int(frame.Grid.Cursor.Y),
		Col: int(frame.Grid.Cursor.X),
	}
}

// surfacePane returns what the daemon painted of a pane.
func (a *App) surfacePane(paneID string) (herdr.PaneSurface, bool) {
	a.mu.Lock()
	frame := a.surface
	a.mu.Unlock()
	if frame == nil {
		return herdr.PaneSurface{}, false
	}
	for _, pane := range frame.Panes {
		if pane.PaneID == paneID {
			return pane, true
		}
	}
	return herdr.PaneSurface{}, false
}

// findNext moves to the next match, or to the one before it.
func (a *App) findNext(step int) {
	a.mu.Lock()
	total := len(a.finder.matches)
	current := a.finder.current
	query := a.finder.query
	paneID := a.finder.paneID
	local := a.finder.local
	a.mu.Unlock()

	if total == 0 || query == "" {
		return
	}
	if local {
		next := (current + step) % total
		if next < 0 {
			next += total
		}
		a.mu.Lock()
		a.finder.current = next
		a.mu.Unlock()
		a.revealCurrent()
		a.invalidate()
		return
	}

	direction := "forward"
	if step < 0 {
		direction = "backward"
	}
	var previous *TextRange
	if current >= 0 && current < len(a.finder.matches) {
		previous = &a.finder.matches[current]
	}
	matches, newTotal, newCurrent, _, err := a.searchDaemon(paneID, query, direction, previous)
	if err != nil {
		a.setStatus("find: %v", err)
		return
	}
	a.mu.Lock()
	a.finder.matches = matches
	a.finder.total = newTotal
	a.finder.current = newCurrent
	a.mu.Unlock()
	a.revealCurrent()
	a.invalidate()
}

// revealCurrent scrolls the pane so that the match the bar is on shows.
func (a *App) revealCurrent() {
	a.mu.Lock()
	matches := a.finder.matches
	current := a.finder.current
	paneID := a.finder.paneID
	a.mu.Unlock()

	if current < 0 || current >= len(matches) {
		return
	}
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return
	}
	offset, scrolls := revealOffset(pane, matches[current])
	if !scrolls {
		return
	}
	a.request("pane.scroll", map[string]any{"pane_id": paneID, "offset_from_bottom": offset})
}

// closeFind closes the find bar.
func (a *App) closeFind() {
	a.mu.Lock()
	a.finder.open = false
	a.finder.matches = nil
	a.finder.total = 0
	a.finder.current = 0
	a.mu.Unlock()
	a.invalidate()
}

// findMatches returns the matches of the find bar.
func (a *App) findMatches() ([]TextRange, int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.finder.matches, a.finder.current
}

// openFind shows the bar that searches the output of a pane.
func (a *App) openFind() {
	a.mu.Lock()
	a.finder.open = true
	a.mu.Unlock()
	a.invalidate()
}

// drawFindBar shows the bar that searches the output of a pane.
func (a *App) drawFindBar(c *ui.Context) {
	theme := c.Theme()
	a.mu.Lock()
	open := a.finder.open
	query := a.finder.query
	current := a.finder.current
	total := a.finder.total
	local := a.finder.local
	a.mu.Unlock()

	if !open {
		return
	}
	value := query
	ui.Row(c).Padding(6, 10).Gap(8).AlignItems(ui.Center).
		Background(theme.Surface).Border(1, theme.Border).Children(func() {
		ui.Text(c, "Find").TextColor(theme.TextMuted).FontSize(12)
		field := ui.TextInput(c, &value)
		field.Placeholder("Find in the output")
		field.Width(260)
		field.OnChange(func() {
			a.mu.Lock()
			a.finder.query = value
			a.mu.Unlock()
		})
		field.OnSubmit(func() {
			a.find(a.selectedPaneID(), value, "forward")
		})
		if total > 0 {
			ui.Text(c, fmt.Sprintf("%d of %d", current+1, total)).FontSize(12).TextColor(theme.TextMuted)
		} else if query != "" {
			ui.Text(c, "no match").FontSize(12).TextColor(theme.TextMuted)
		}
		if local {
			ui.Text(c, "in view").FontSize(11).TextColor(theme.TextMuted)
		}
		ui.Row(c).Grow(1)
		if ui.Button(c, "‹").Label("Previous match").Clicked() {
			a.findNext(-1)
		}
		if ui.Button(c, "›").Label("Next match").Clicked() {
			a.findNext(1)
		}
		if ui.Button(c, "✕").Label("Close the find bar").Clicked() {
			a.closeFind()
		}
	})
}

// drawFindHighlight marks the match the find bar is on.
func (a *App) drawFindHighlight(p *ui.Painter, r ui.Rect, cellWidth float32) {
	matches, current := a.findMatches()
	if current < 0 || current >= len(matches) {
		return
	}
	paneID := a.finderPaneID()
	pane, ok := a.surfacePane(paneID)
	if !ok {
		return
	}
	match := matches[current]
	top := viewportTop(pane)
	row := match.Start.Row - top
	if row < 0 {
		return
	}
	x := r.X + float32(match.Start.Col)*cellWidth
	y := r.Y + float32(row)*a.rowHeightPx()
	width := float32(match.End.Col-match.Start.Col+1) * cellWidth
	p.Fill(ui.Rect{X: x, Y: y, W: width, H: a.rowHeightPx()}, ui.RGBA(255, 200, 60, 0.35), 0)
}

// finderPaneID returns the pane the find bar searches.
func (a *App) finderPaneID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.finder.paneID
}

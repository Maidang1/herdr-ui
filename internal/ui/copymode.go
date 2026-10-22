package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Copy mode is the keyboard way of walking a pane's text: a cursor that
// moves over the screen and the scrollback, marks what it chooses and
// copies it.
//
// The steps that are arithmetic on the screen — a cell, a line, a page —
// happen here. The motions that depend on the text, such as the end of a
// word, and the searches are the daemon's: they go out as
// `pane.copy_motion` and `pane.copy_search`.

// copyCommand is what one key of the window asks of copy mode.
type copyCommand struct {
	// Kind is what the command does.
	Kind string
	// Rows and Cols are the steps of a cell step.
	Rows, Cols int
	// Top asks for the history rather than the screen.
	Top bool
	// Down and Half say which page a page step wants.
	Down bool
	Half bool
	// Motion is the motion the daemon carries out.
	Motion string
	// Direction is the way a search runs.
	Direction string
	// Lines says a mark chooses whole lines.
	Lines bool
}

// The kinds of copy-mode command.
const (
	copyStep      = "step"
	copyLineStart = "line_start"
	copyHistory   = "history"
	copyPage      = "page"
	copyMotion    = "motion"
	copyPrompt    = "prompt"
	copySearch    = "search"
	copyRepeat    = "repeat"
	copyMark      = "mark"
	copyCopy      = "copy"
	copyCancel    = "cancel"
	copyExit      = "exit"
)

// copyModeState is the copy mode of a pane.
type copyModeState struct {
	// active says the copy mode holds the keyboard.
	active bool
	// paneID is the pane the cursor walks.
	paneID string
	// cursor is where the cursor stands, counted in the scrollback.
	cursor TextPoint
	// mark is where the selection began.
	mark TextPoint
	// hasMark says a selection began.
	hasMark bool
	// markLine says the selection chooses whole lines.
	markLine bool
	// entryOffset is the offset the pane showed when copy mode began; it
	// comes back when copy mode ends.
	entryOffset uint64
	// search is the last query, and direction the way it ran.
	search    string
	direction string
	// matches are where the query was found.
	matches []TextRange
	// current is the match the cursor is on.
	current int
	// total is how many matches there are.
	total int
	// prompt says the search prompt is open.
	prompt bool
	// promptDirection is the way the prompt searches.
	promptDirection string
	// promptText is what has been typed in the prompt.
	promptText string
}

// copyModeCommand turns a key of the window into a copy-mode command.
//
// The keys are the vi ones, the way the daemon's copy mode reads them.
func copyModeCommand(key ui.Key, mods ui.Modifiers) (copyCommand, bool) {
	if mods&(ui.Alt|ui.Super) != 0 {
		return copyCommand{}, false
	}
	shift := mods&ui.Shift != 0
	control := mods&ui.Ctrl != 0

	// A chord moves by half a page or a whole one.
	if control {
		switch key {
		case ui.KeyU:
			return copyCommand{Kind: copyPage, Down: false, Half: true}, true
		case ui.KeyD:
			return copyCommand{Kind: copyPage, Down: true, Half: true}, true
		case ui.KeyB:
			return copyCommand{Kind: copyPage, Down: false}, true
		case ui.KeyF:
			return copyCommand{Kind: copyPage, Down: true}, true
		}
		return copyCommand{}, false
	}

	switch key {
	case ui.KeyLeft:
		return copyCommand{Kind: copyStep, Cols: -1}, true
	case ui.KeyDown:
		return copyCommand{Kind: copyStep, Rows: 1}, true
	case ui.KeyUp:
		return copyCommand{Kind: copyStep, Rows: -1}, true
	case ui.KeyRight:
		return copyCommand{Kind: copyStep, Cols: 1}, true
	case ui.KeyPageUp:
		return copyCommand{Kind: copyPage, Down: false}, true
	case ui.KeyPageDown:
		return copyCommand{Kind: copyPage, Down: true}, true
	case ui.KeyHome:
		return copyCommand{Kind: copyLineStart}, true
	case ui.KeyEnd:
		return copyCommand{Kind: copyMotion, Motion: "line_end"}, true
	case ui.KeyEscape:
		return copyCommand{Kind: copyCancel}, true
	case ui.KeyEnter:
		return copyCommand{Kind: copyCopy}, true
	case ui.KeySpace:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyMark}, true
	case ui.KeyH:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyStep, Cols: -1}, true
	case ui.KeyJ:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyStep, Rows: 1}, true
	case ui.KeyK:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyStep, Rows: -1}, true
	case ui.KeyL:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyStep, Cols: 1}, true
	case ui.KeyW:
		return copyCommand{Kind: copyMotion, Motion: "next_word_start"}, true
	case ui.KeyB:
		return copyCommand{Kind: copyMotion, Motion: "previous_word_start"}, true
	case ui.KeyE:
		return copyCommand{Kind: copyMotion, Motion: "next_word_end"}, true
	case ui.KeyG:
		if shift {
			return copyCommand{Kind: copyHistory, Top: false}, true
		}
		return copyCommand{Kind: copyHistory, Top: true}, true
	case ui.KeyN:
		return copyCommand{Kind: copyRepeat, Direction: "backward"}, true
	case ui.KeyV:
		return copyCommand{Kind: copyMark, Lines: shift}, true
	case ui.KeyY:
		return copyCommand{Kind: copyCopy}, true
	case ui.KeyQ:
		return copyCommand{Kind: copyExit}, true
	case ui.KeySlash:
		if shift {
			return copyCommand{Kind: copyPrompt, Direction: "backward"}, true
		}
		return copyCommand{Kind: copyPrompt, Direction: "forward"}, true
	case ui.Key0:
		if shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyLineStart}, true
	case ui.Key4:
		if !shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyMotion, Motion: "line_end"}, true
	case ui.Key6:
		if !shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyMotion, Motion: "first_non_blank"}, true
	case ui.KeyMinus:
		if shift {
			return copyCommand{Kind: copyPrompt, Direction: "backward"}, true
		}
		return copyCommand{}, false
	case ui.KeyEqual:
		if !shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyPrompt, Direction: "forward"}, true
	case ui.KeyBracketLeft:
		if !shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyMotion, Motion: "previous_paragraph"}, true
	case ui.KeyBracketRight:
		if !shift {
			return copyCommand{}, false
		}
		return copyCommand{Kind: copyMotion, Motion: "next_paragraph"}, true
	}
	return copyCommand{}, false
}

// copyModeChar reads a character as a copy-mode key, for the text an input
// method commits.
func copyModeChar(char rune) (copyCommand, bool) {
	switch char {
	case ' ':
		return copyCommand{Kind: copyMark}, true
	case '$':
		return copyCommand{Kind: copyMotion, Motion: "line_end"}, true
	case '^':
		return copyCommand{Kind: copyMotion, Motion: "first_non_blank"}, true
	case '{':
		return copyCommand{Kind: copyMotion, Motion: "previous_paragraph"}, true
	case '}':
		return copyCommand{Kind: copyMotion, Motion: "next_paragraph"}, true
	case '0':
		return copyCommand{Kind: copyLineStart}, true
	case '/':
		return copyCommand{Kind: copyPrompt, Direction: "forward"}, true
	case '?':
		return copyCommand{Kind: copyPrompt, Direction: "backward"}, true
	}
	lower := strings.ToLower(string(char))
	shift := char >= 'A' && char <= 'Z'
	switch lower {
	case "h":
		return copyCommand{Kind: copyStep, Cols: -1}, true
	case "j":
		return copyCommand{Kind: copyStep, Rows: 1}, true
	case "k":
		return copyCommand{Kind: copyStep, Rows: -1}, true
	case "l":
		return copyCommand{Kind: copyStep, Cols: 1}, true
	case "w":
		if shift {
			return copyCommand{Kind: copyMotion, Motion: "next_big_word_start"}, true
		}
		return copyCommand{Kind: copyMotion, Motion: "next_word_start"}, true
	case "b":
		if shift {
			return copyCommand{Kind: copyMotion, Motion: "previous_big_word_start"}, true
		}
		return copyCommand{Kind: copyMotion, Motion: "previous_word_start"}, true
	case "e":
		if shift {
			return copyCommand{Kind: copyMotion, Motion: "next_big_word_end"}, true
		}
		return copyCommand{Kind: copyMotion, Motion: "next_word_end"}, true
	case "g":
		return copyCommand{Kind: copyHistory, Top: !shift}, true
	case "n":
		return copyCommand{Kind: copyRepeat, Direction: "forward"}, true
	case "v":
		return copyCommand{Kind: copyMark, Lines: shift}, true
	case "y":
		return copyCommand{Kind: copyCopy}, true
	case "q":
		return copyCommand{Kind: copyExit}, true
	}
	return copyCommand{}, false
}

// enterCopyMode begins the copy mode of the focused pane.
func (a *App) enterCopyMode() {
	pane, ok := a.surfacePane(a.selectedPaneID())
	if !ok {
		a.setStatus("no pane is focused")
		return
	}
	frame := a.surfaceOf()
	cursor := TextPoint{Row: viewportTop(pane), Col: 0}
	if frame != nil && frame.Grid.Cursor != nil && frame.Grid.Cursor.Visible {
		cursor = TextPoint{
			Row: viewportTop(pane) + int(frame.Grid.Cursor.Y),
			Col: int(frame.Grid.Cursor.X),
		}
	}
	offset := uint64(0)
	if pane.Scroll != nil {
		offset = pane.Scroll.OffsetFromBottom
	}
	a.mu.Lock()
	a.copyMode.active = true
	a.copyMode.paneID = pane.PaneID
	a.copyMode.cursor = cursor
	a.copyMode.hasMark = false
	a.copyMode.markLine = false
	a.copyMode.matches = nil
	a.copyMode.current = 0
	a.copyMode.total = 0
	a.copyMode.entryOffset = offset
	a.copyMode.prompt = false
	a.copyMode.promptText = ""
	a.mu.Unlock()
	a.invalidate()
}

// leaveCopyMode ends the copy mode and brings the pane back to where it
// was.
func (a *App) leaveCopyMode() {
	a.mu.Lock()
	state := a.copyMode
	a.copyMode = copyModeState{}
	a.mu.Unlock()
	if state.paneID == "" {
		return
	}
	a.request("pane.scroll", map[string]any{
		"pane_id":            state.paneID,
		"offset_from_bottom": state.entryOffset,
	})
	a.invalidate()
}

// copyModeActive says whether the copy mode holds the keyboard.
func (a *App) copyModeActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.copyMode.active
}

// copyModeKey takes a key of the window for the copy mode.
func (a *App) copyModeKey(key ui.Key, mods ui.Modifiers) bool {
	a.mu.Lock()
	prompt := a.copyMode.prompt
	a.mu.Unlock()
	if prompt {
		// The prompt takes the keys that close it, and the rest of the
		// keys are the query.
		switch key {
		case ui.KeyEscape:
			a.closeCopyPrompt()
			return true
		case ui.KeyEnter:
			a.submitCopyPrompt()
			return true
		case ui.KeyBackspace:
			a.copyPromptBackspace()
			return true
		}
		if command, ok := copyModeCommand(key, mods); ok {
			a.closeCopyPrompt()
			a.runCopyCommand(command)
			return true
		}
		return true
	}
	command, ok := copyModeCommand(key, mods)
	if !ok {
		// Every other key is swallowed: nothing reaches the terminal
		// while the copy mode holds the keyboard.
		return true
	}
	a.runCopyCommand(command)
	return true
}

// copyModeText reads text as copy-mode keys.
func (a *App) copyModeText(text string) bool {
	a.mu.Lock()
	prompt := a.copyMode.prompt
	a.mu.Unlock()
	if prompt {
		for _, char := range text {
			if char == '\n' {
				a.submitCopyPrompt()
				continue
			}
			if char == '\r' {
				continue
			}
			a.appendCopyPrompt(string(char))
		}
		return true
	}
	for _, char := range text {
		command, ok := copyModeChar(char)
		if !ok {
			continue
		}
		a.runCopyCommand(command)
	}
	return true
}

// closeCopyPrompt closes the search prompt.
func (a *App) closeCopyPrompt() {
	a.mu.Lock()
	a.copyMode.prompt = false
	a.copyMode.promptText = ""
	a.mu.Unlock()
	a.invalidate()
}

// appendCopyPrompt adds a character to the query of the prompt.
func (a *App) appendCopyPrompt(text string) {
	a.mu.Lock()
	a.copyMode.promptText += text
	a.mu.Unlock()
	a.invalidate()
}

// copyPromptBackspace takes the last character of the query back.
func (a *App) copyPromptBackspace() {
	a.mu.Lock()
	query := a.copyMode.promptText
	if query != "" {
		runes := []rune(query)
		a.copyMode.promptText = string(runes[:len(runes)-1])
	}
	a.mu.Unlock()
	a.invalidate()
}

// submitCopyPrompt sends the query of the prompt to the daemon.
func (a *App) submitCopyPrompt() {
	a.mu.Lock()
	query := a.copyMode.promptText
	direction := a.copyMode.promptDirection
	a.copyMode.prompt = false
	a.copyMode.promptText = ""
	a.mu.Unlock()
	if query == "" {
		return
	}
	a.runCopyCommand(copyCommand{Kind: copySearch, Direction: direction})
	a.mu.Lock()
	a.copyMode.search = query
	a.mu.Unlock()
}

// runCopyCommand carries out one copy-mode command.
func (a *App) runCopyCommand(command copyCommand) {
	switch command.Kind {
	case copyStep:
		a.copyStepBy(command.Rows, command.Cols)
	case copyLineStart:
		a.mu.Lock()
		a.copyMode.cursor.Col = 0
		a.mu.Unlock()
		a.revealCopyCursor()
	case copyHistory:
		a.copyHistory(command.Top)
	case copyPage:
		a.copyPage(command.Down, command.Half)
	case copyMotion:
		a.copyMotion(command.Motion)
	case copyPrompt:
		a.mu.Lock()
		a.copyMode.prompt = true
		a.copyMode.promptDirection = command.Direction
		a.copyMode.promptText = ""
		a.mu.Unlock()
		a.invalidate()
	case copySearch:
		a.copySearch(command.Direction)
	case copyRepeat:
		a.copyRepeat(command.Direction == "backward")
	case copyMark:
		a.copyMark(command.Lines)
	case copyCopy:
		a.copyMarkedText()
	case copyCancel:
		a.copyCancel()
	case copyExit:
		a.leaveCopyMode()
	}
	a.invalidate()
}

// copyStepBy moves the cursor a number of cells.
func (a *App) copyStepBy(rows, cols int) {
	a.mu.Lock()
	cursor := a.copyMode.cursor
	cursor.Row += rows
	if cursor.Col+cols < 0 {
		cursor.Col = 0
	} else {
		cursor.Col += cols
	}
	a.copyMode.cursor = cursor
	a.mu.Unlock()
	a.revealCopyCursor()
}

// copyHistory walks to the oldest row or the newest.
func (a *App) copyHistory(top bool) {
	pane, ok := a.surfacePane(a.copyModePaneID())
	if !ok || pane.Scroll == nil {
		return
	}
	height := int(pane.InnerRect.Height)
	offset := pane.Scroll.MaxOffsetFromBottom
	if !top {
		offset = 0
	}
	a.request("pane.scroll", map[string]any{
		"pane_id":            pane.PaneID,
		"offset_from_bottom": offset,
	})
	a.mu.Lock()
	if top {
		a.copyMode.cursor.Row = int(pane.Scroll.MaxOffsetFromBottom)
		a.copyMode.cursor.Col = 0
	} else {
		a.copyMode.cursor.Row = int(pane.Scroll.MaxOffsetFromBottom) - int(pane.Scroll.OffsetFromBottom) + height - 1
	}
	a.mu.Unlock()
}

// copyPage scrolls the pane a page, or half of one.
func (a *App) copyPage(down, half bool) {
	pane, ok := a.surfacePane(a.copyModePaneID())
	if !ok || pane.Scroll == nil {
		return
	}
	height := int(pane.InnerRect.Height)
	if half {
		height /= 2
	}
	if height < 1 {
		height = 1
	}
	offset := int(pane.Scroll.OffsetFromBottom)
	if down {
		offset -= height
	} else {
		offset += height
	}
	if offset < 0 {
		offset = 0
	}
	if max := int(pane.Scroll.MaxOffsetFromBottom); offset > max {
		offset = max
	}
	a.request("pane.scroll", map[string]any{
		"pane_id":            pane.PaneID,
		"offset_from_bottom": offset,
	})
	a.mu.Lock()
	cursor := a.copyMode.cursor
	if down {
		cursor.Row += height
	} else {
		cursor.Row -= height
	}
	if cursor.Row < 0 {
		cursor.Row = 0
	}
	a.copyMode.cursor = cursor
	a.mu.Unlock()
}

// copyMotion asks the daemon to carry out a motion.
func (a *App) copyMotion(motion string) {
	pane, ok := a.surfacePane(a.copyModePaneID())
	if !ok {
		return
	}
	a.mu.Lock()
	cursor := a.copyMode.cursor
	a.mu.Unlock()

	if a.endpoint == nil {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		var result struct {
			Type   string    `json:"type"`
			Cursor TextPoint `json:"cursor"`
		}
		err := a.endpoint.Request(ctx, "pane.copy_motion", map[string]any{
			"pane_id":          pane.PaneID,
			"cursor":           map[string]any{"row": cursor.Row, "col": cursor.Col},
			"motion":           motion,
			"content_revision": pane.ContentRevision,
		}, &result)
		if err != nil {
			a.setStatus("copy mode: %v", err)
			return
		}
		a.mu.Lock()
		a.copyMode.cursor = result.Cursor
		a.mu.Unlock()
		a.revealCopyCursor()
		a.invalidate()
	}()
}

// copySearch asks the daemon to search the scrollback.
func (a *App) copySearch(direction string) {
	if _, ok := a.surfacePane(a.copyModePaneID()); !ok {
		return
	}
	a.mu.Lock()
	query := a.copyMode.search
	cursor := a.copyMode.cursor
	a.mu.Unlock()
	if query == "" {
		return
	}

	if a.endpoint == nil {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		var result struct {
			Type    string      `json:"type"`
			Matches []TextRange `json:"matches"`
			Total   int         `json:"total"`
			Current int         `json:"current"`
		}
		// The daemon refuses a search of a revision that has moved on,
		// so the window tries again with the newer one it has painted.
		var err error
		for attempt := 0; attempt < 4; attempt++ {
			if attempt > 0 {
				time.Sleep(150 * time.Millisecond)
			}
			pane, ok := a.surfacePane(a.copyModePaneID())
			if !ok {
				return
			}
			err = a.endpoint.Request(ctx, "pane.copy_search", map[string]any{
				"pane_id":          pane.PaneID,
				"query":            query,
				"direction":        direction,
				"cursor":           map[string]any{"row": cursor.Row, "col": cursor.Col},
				"content_revision": pane.ContentRevision,
			}, &result)
			if err == nil {
				break
			}
			if !strings.Contains(err.Error(), "stale_content") {
				break
			}
		}
		if err != nil {
			a.setStatus("copy mode: %v", err)
			return
		}
		a.mu.Lock()
		a.copyMode.matches = result.Matches
		a.copyMode.total = result.Total
		current := result.Current
		if current < 0 || current >= len(result.Matches) {
			current = 0
		}
		a.copyMode.current = current
		if current < len(result.Matches) {
			a.copyMode.cursor = result.Matches[current].Start
		}
		a.mu.Unlock()
		a.revealCopyCursor()
		a.invalidate()
	}()
}

// copyRepeat searches again, in the direction the last search ran.
func (a *App) copyRepeat(backward bool) {
	a.mu.Lock()
	direction := a.copyMode.direction
	if backward {
		direction = "backward"
	}
	if direction == "" {
		direction = "forward"
	}
	a.copyMode.direction = direction
	a.mu.Unlock()
	a.copySearch(direction)
}

// copyMark starts or ends a selection.
func (a *App) copyMark(lines bool) {
	a.mu.Lock()
	if a.copyMode.hasMark {
		a.copyMode.hasMark = false
	} else {
		a.copyMode.hasMark = true
		a.copyMode.mark = a.copyMode.cursor
		a.copyMode.markLine = lines
	}
	a.mu.Unlock()
}

// copyMarkedText copies the selection and leaves the copy mode.
func (a *App) copyMarkedText() {
	a.mu.Lock()
	if !a.copyMode.hasMark {
		a.mu.Unlock()
		return
	}
	mark := a.copyMode.mark
	cursor := a.copyMode.cursor
	paneID := a.copyMode.paneID
	a.mu.Unlock()

	start, end := mark, cursor
	if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
		start, end = end, start
	}
	if a.endpoint == nil {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		var result struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		err := a.endpoint.Request(ctx, "pane.selection.read", map[string]any{
			"pane_id": paneID,
			"anchor":  map[string]any{"row": start.Row, "col": start.Col},
			"cursor":  map[string]any{"row": end.Row, "col": end.Col},
		}, &result)
		if err != nil {
			a.setStatus("copy mode: %v", err)
			return
		}
		a.copyText(strings.TrimRight(result.Text, "\n"))
		a.leaveCopyMode()
	}()
}

// copyCancel drops the selection and the search, or leaves the copy mode
// when there is neither.
func (a *App) copyCancel() {
	a.mu.Lock()
	if a.copyMode.hasMark {
		a.copyMode.hasMark = false
		a.copyMode.matches = nil
		a.copyMode.total = 0
		a.copyMode.current = 0
	} else if len(a.copyMode.matches) > 0 {
		a.copyMode.matches = nil
		a.copyMode.total = 0
		a.copyMode.current = 0
	} else {
		a.mu.Unlock()
		a.leaveCopyMode()
		return
	}
	a.mu.Unlock()
	a.invalidate()
}

// copyModeStateOf returns the copy mode the window holds.
func (a *App) copyModeStateOf() copyModeState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.copyMode
}

// copyModePaneID returns the pane the copy mode walks.
func (a *App) copyModePaneID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.copyMode.paneID
}

// revealCopyCursor scrolls the pane so that the cursor shows.
func (a *App) revealCopyCursor() {
	pane, ok := a.surfacePane(a.copyModePaneID())
	if !ok || pane.Scroll == nil {
		return
	}
	a.mu.Lock()
	cursor := a.copyMode.cursor
	a.mu.Unlock()

	height := int(pane.InnerRect.Height)
	top := viewportTop(pane)
	if cursor.Row >= top && cursor.Row < top+height {
		return
	}
	wantedTop := cursor.Row - height/2
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
	a.request("pane.scroll", map[string]any{
		"pane_id":            pane.PaneID,
		"offset_from_bottom": offset,
	})
}

// drawCopyMode marks the cursor of the copy mode and the cells it chose.
func (a *App) drawCopyMode(p *ui.Painter, r ui.Rect, cellWidth float32) {
	a.mu.Lock()
	state := a.copyMode
	a.mu.Unlock()
	if !state.active {
		return
	}
	pane, ok := a.surfacePane(state.paneID)
	if !ok {
		return
	}
	top := viewportTop(pane)
	left, _, right, _ := paneRegion(pane)

	// The cells the cursor chose.
	if state.hasMark {
		start, end := state.mark, state.cursor
		if end.Row < start.Row || (end.Row == start.Row && end.Col < start.Col) {
			start, end = end, start
		}
		for row := start.Row; row <= end.Row; row++ {
			gridRow := row - top
			if gridRow < 0 || gridRow >= int(pane.InnerRect.Height) {
				continue
			}
			from, to := left, right
			if row == start.Row {
				from = start.Col
			}
			if row == end.Row {
				to = end.Col
			}
			if state.markLine {
				from, to = left, right
			}
			if to < from {
				to = from
			}
			x := r.X + float32(from)*cellWidth
			y := r.Y + float32(gridRow)*a.rowHeightPx()
			width := float32(to-from+1) * cellWidth
			p.Fill(ui.Rect{X: x, Y: y, W: width, H: a.rowHeightPx()}, ui.RGBA(120, 170, 255, 0.30), 0)
		}
	}

	// The cursor of the copy mode.
	gridRow := state.cursor.Row - top
	if gridRow < 0 || gridRow >= int(pane.InnerRect.Height) {
		return
	}
	column := state.cursor.Col
	if column < left {
		column = left
	}
	if column > right {
		column = right
	}
	x := r.X + float32(column)*cellWidth
	y := r.Y + float32(gridRow)*a.rowHeightPx()
	p.Fill(ui.Rect{X: x, Y: y, W: cellWidth, H: a.rowHeightPx()}, ui.RGBA(255, 200, 60, 0.75), 0)
}

// copyModeStatus says what the copy mode is doing, for the status bar.
func (a *App) copyModeStatus() (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.copyMode.active {
		return "", false
	}
	if a.copyMode.prompt {
		return fmt.Sprintf("copy mode /%s%s", a.copyMode.promptDirection, a.copyMode.promptText), true
	}
	if a.copyMode.total > 0 {
		return fmt.Sprintf("copy mode  %d of %d", a.copyMode.current+1, a.copyMode.total), true
	}
	if a.copyMode.hasMark {
		return "copy mode  selection", true
	}
	return "copy mode", true
}

package ui

import (
	"strconv"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/Maidang1/herdr-ui/internal/terminalhost"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// findState is the find bar's: a search of one pane's screen and
// scrollback through its terminal, as Ghostty's find. A closed bar keeps
// the query for the next time it opens over a pane.
type findState struct {
	// pane is the pane searched; "" while the bar is closed. binding
	// addresses its terminal and follows remaps, which recreate it.
	pane    string
	binding terminalhost.Binding
	query   string
	status  terminal.SearchStatus
	// focus has the field take the keyboard as the bar opens.
	focus bool
}

func (w *Window) findOpen() bool { return w.find.pane != "" }

// findPaneBinding returns the focused pane's terminal binding.
func (w *Window) findPaneBinding() (terminalhost.Binding, string, bool) {
	if w.terminal == nil {
		return terminalhost.Binding{}, "", false
	}
	v := w.currentView()
	child := w.terminal.panes[v.PaneID]
	if child == nil {
		return terminalhost.Binding{}, "", false
	}
	return child.binding, v.PaneID, true
}

// findOpenAction opens the bar over the focused pane, the last query to
// type over, or gives the field the keyboard again when it is open there
// already. Opening over another pane ends that pane's search first.
func (w *Window) findOpenAction() {
	binding, pane, ok := w.findPaneBinding()
	if !ok {
		return
	}
	if w.find.pane == pane {
		w.find.focus = true
		return
	}
	query := w.find.query
	w.findClose()
	w.find = findState{pane: pane, binding: binding, query: query, focus: true}
	if w.find.query != "" && w.terminal.host != nil {
		w.terminal.host.SearchPane(binding, w.find.query)
	}
}

// findStep goes to the next match, toward older output, or the previous
// one; with the bar closed it opens, the last query searched again.
func (w *Window) findStep(back bool) {
	if !w.findOpen() || w.terminal == nil || w.terminal.host == nil {
		w.findOpenAction()
		return
	}
	if back {
		w.terminal.host.SearchPrevious(w.find.binding)
		return
	}
	w.terminal.host.SearchNext(w.find.binding)
}

// findClose takes the bar down, its highlights going, and gives the
// keyboard back to the pane.
func (w *Window) findClose() {
	if !w.findOpen() {
		return
	}
	if w.terminal != nil && w.terminal.host != nil {
		w.terminal.host.EndSearch(w.find.binding)
	}
	w.find = findState{query: w.find.query}
	w.BlurInput()
}

// SetSearchStatus reports a terminal's search status, posted by the
// terminal host on the UI thread; the bar counts only the search of the
// pane it has it open on.
func (w *Window) SetSearchStatus(binding terminalhost.Binding, status terminal.SearchStatus) {
	if w.findOpen() && binding == w.find.binding {
		w.find.status = status
	}
}

// findBar floats over the top right of the pane it searches: the query,
// how many matches it has, and the buttons going through them.
func (w *Window) findBar(c *native.Context, r layout.Rect, p design.Palette) {
	if t := w.terminal; t == nil || t.host == nil {
		return
	}
	// A remapped pane recreates its terminal; the bar adopts the new
	// binding and searches its query there again.
	if child := w.terminal.panes[w.find.pane]; child != nil && child.binding != w.find.binding {
		w.find.binding = child.binding
		w.find.status = terminal.SearchStatus{}
		if w.find.query != "" {
			w.terminal.host.SearchPane(w.find.binding, w.find.query)
		}
	}
	anchor := native.Row(c.Key("find-anchor:"+w.find.pane)).Absolute().Left(float32(r.X)).Top(float32(r.Y)).Size(float32(r.W), float32(r.H)).Justify(native.End).AlignItems(native.Start).PassThrough()
	anchor.Label("Find in terminal")
	anchor.Children(func() {
		bar := native.Row(c.Key("find-bar")).Shrink(1).Height(32).MaxWidth(float32(r.W)-16).Padding(0, 4, 0, 10).Gap(4).AlignItems(native.Center).Radius(8).Background(p.Elevated).Border(1, p.Border).Shadow(0, 4, 16, 0, p.ContentShadow)
		bar.Children(func() {
			native.Icon(c, icons.Search).Size(13, 13).Shrink(0).TextColor(p.Tertiary)
			input := native.TextInput(c.Key("find-query"), &w.find.query).Width(150).FontSize(13).Placeholder("Find").TextColor(p.Text)
			input.Label("Find in terminal")
			if w.find.focus {
				w.find.focus = false
				input.Focus()
			}
			if input.Changed() {
				w.terminal.host.SearchPane(w.find.binding, w.find.query)
			}
			// Enter goes to older output, Shift+Enter to newer.
			if bar.Shortcut(native.Shift, native.KeyEnter) {
				w.findStep(true)
			} else if input.Submitted() {
				w.findStep(false)
			}
			native.Text(c, findCount(w.find)).FontSize(12).TextColor(p.Secondary).SingleLine().MinWidth(34).TextAlign(native.End)
			// Up goes back through the output, as Enter does.
			if b := w.findButton(c, "find-next", icons.ChevronDown, "Find next", p, 180); b.Clicked() {
				w.findStep(false)
			}
			if b := w.findButton(c, "find-previous", icons.ChevronDown, "Find previous", p, 0); b.Clicked() {
				w.findStep(true)
			}
			if b := w.findButton(c, "find-close", icons.X, "Close find", p, 0); b.Clicked() {
				w.findClose()
			}
		})
		// Escape closes the bar from its field and its buttons alike.
		if bar.FocusWithin() && bar.Shortcut(0, native.KeyEscape) {
			w.findClose()
		}
	})
}

// findButton is one of the bar's quiet square controls; degrees turns its
// glyph, as a down chevron turned 180 points up.
func (w *Window) findButton(c *native.Context, key string, icon *native.SVG, label string, p design.Palette, degrees float32) native.Element {
	button := native.ButtonBase(c.Key(key)).Size(22, 22).Shrink(0).Radius(5).Center().Label(label).Tooltip(label).KeepFocus()
	w.controlState(button, p)
	button.Children(func() {
		glyph := native.Icon(c, icon).Size(13, 13).TextColor(p.Secondary)
		if degrees != 0 {
			glyph.Rotate(degrees)
		}
	})
	return button
}

// findCount is the bar's "1/3"; an ongoing search shows an ellipsis until
// the scrollback has been gone through.
func findCount(f findState) string {
	switch {
	case f.query == "":
		return ""
	case f.status.Searching:
		return "…"
	case f.status.Total > 0:
		return strconv.Itoa(f.status.Selected) + "/" + strconv.Itoa(f.status.Total)
	}
	return ""
}

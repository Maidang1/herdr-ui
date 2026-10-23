package session

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

type selectionRejection string

func (e selectionRejection) Error() string { return string(e) }

type textPoint struct {
	Row uint32 `json:"row"`
	Col uint16 `json:"col"`
}
type textRange struct {
	Start textPoint `json:"start"`
	End   textPoint `json:"end"`
}

// History captures are unwrapped text, whereas command.invoke expects screen
// cells. Ask Herdr for real coordinates and verify the extracted bytes before
// attaching a revision-guarded selection to the authoritative command. Never
// infer cell columns from rune indices (wide glyphs and wraps make that unsafe).
func resolveCommandSelection(ctx context.Context, client *herdr.Client, op Operation) (map[string]any, error) {
	if op.SelectedText == nil {
		return nil, selectionRejection("missing selection")
	}
	selected := *op.SelectedText
	if selected == "" {
		return nil, selectionRejection("selection is empty")
	}
	var motion struct {
		PaneID   string `json:"pane_id"`
		Revision uint64 `json:"content_revision"`
	}
	if err := client.Call(ctx, "pane.copy_motion", map[string]any{"pane_id": op.PaneID, "cursor": textPoint{}, "motion": "line_end"}, &motion); err != nil {
		return nil, err
	}
	if motion.PaneID != op.PaneID {
		return nil, &herdr.ProtocolError{Reason: "command selection revision target mismatch"}
	}
	search := func(query string) ([]textRange, error) {
		var result struct {
			PaneID   string      `json:"pane_id"`
			Revision uint64      `json:"content_revision"`
			Matches  []textRange `json:"matches"`
		}
		err := client.Call(ctx, "pane.copy_search", map[string]any{"pane_id": op.PaneID, "query": query, "direction": "forward", "cursor": textPoint{}, "content_revision": motion.Revision}, &result)
		if err != nil {
			return nil, err
		}
		if result.PaneID != op.PaneID || result.Revision != motion.Revision {
			return nil, &herdr.ProtocolError{Reason: "command selection search target mismatch"}
		}
		return result.Matches, nil
	}
	lines := strings.Split(selected, "\n")
	first, last := 0, len(lines)-1
	for first < last && lines[first] == "" {
		first++
	}
	for last > first && lines[last] == "" {
		last--
	}
	prefix, suffix := selectionEdge(lines[first], false), selectionEdge(lines[last], true)
	if prefix == "" || suffix == "" {
		return nil, selectionRejection("select non-empty text before invoking a plugin action")
	}
	starts, err := search(prefix)
	if err != nil {
		return nil, err
	}
	ends := starts
	if suffix != prefix {
		ends, err = search(suffix)
		if err != nil {
			return nil, err
		}
	}
	attempts := 0
	for _, start := range starts {
		for _, end := range ends {
			anchor, cursor := start.Start, end.End
			if anchor.Row > cursor.Row || (anchor.Row == cursor.Row && anchor.Col > cursor.Col) {
				continue
			}
			if first > 0 {
				if anchor.Row < uint32(first) {
					continue
				}
				anchor.Row -= uint32(first)
				anchor.Col = 0
			}
			trailing := len(lines) - 1 - last
			if trailing > 0 {
				cursor.Row += uint32(trailing)
				cursor.Col = 0
			}
			if attempts >= 128 {
				return nil, selectionRejection("selection is ambiguous; narrow it and try again")
			}
			attempts++
			params := map[string]any{"pane_id": op.PaneID, "anchor": anchor, "cursor": cursor, "content_revision": motion.Revision}
			var read struct {
				PaneID string `json:"pane_id"`
				Text   string `json:"text"`
			}
			if err := client.Call(ctx, "pane.selection.read", params, &read); err != nil {
				return nil, err
			}
			if read.PaneID != op.PaneID {
				return nil, &herdr.ProtocolError{Reason: "command selection read target mismatch"}
			}
			if read.Text == selected {
				return params, nil
			}
		}
	}
	return nil, selectionRejection("selected text no longer matches terminal history; refresh copy mode and select again")
}
func selectionEdge(text string, suffix bool) string {
	const limit = 2048
	if len(text) <= limit {
		return text
	}
	if suffix {
		start := len(text) - limit
		for !utf8.RuneStart(text[start]) {
			start++
		}
		return text[start:]
	}
	end := limit
	for !utf8.RuneStart(text[end]) {
		end--
	}
	return text[:end]
}

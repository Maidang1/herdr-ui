package ui

import (
	"strings"
	"unicode"
)

// copyState owns an immutable history capture. Copying never mixes revisions
// when the live terminal continues producing output behind this view.
type copyState struct {
	searchFound                  bool
	matchRow, matchCol, matchLen int
	lines                        []string
	row, col                     int
	anchorRow, anchorCol         int
	selecting, linewise          bool
	query                        string
	reverse                      bool
	focus                        bool
}

func newCopyState(text string) *copyState {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return &copyState{lines: lines, row: len(lines) - 1, focus: true}
}
func (s *copyState) clamp() {
	s.row = max(0, min(s.row, len(s.lines)-1))
	s.col = max(0, min(s.col, max(0, len([]rune(s.lines[s.row]))-1)))
}
func (s *copyState) key(key string) (exit, copy bool) {
	switch key {
	case "q":
		return true, false
	case "esc":
		if s.selecting || s.query != "" {
			s.selecting = false
			s.query = ""
			s.searchFound = false
			return
		}
		return true, false
	case "enter", "y":
		return true, s.selecting || s.searchFound
	case "left", "h":
		s.col--
	case "right", "l":
		s.col++
	case "up", "k":
		s.row--
	case "down", "j":
		s.row++
	case "pageup", "ctrl+b":
		s.row -= 15
	case "pagedown", "ctrl+f":
		s.row += 15
	case "ctrl+u":
		s.row -= 7
	case "ctrl+d":
		s.row += 7
	case "home", "0":
		s.col = 0
	case "end", "$":
		s.col = len([]rune(s.lines[s.row])) - 1
	case "^":
		s.col = 0
		for _, r := range []rune(s.lines[s.row]) {
			if !unicode.IsSpace(r) {
				break
			}
			s.col++
		}
	case "g":
		s.row = 0
	case "shift+g":
		s.row = len(s.lines) - 1
	case "v", "space", "shift+v":
		s.selecting = true
		s.linewise = key == "shift+v"
		s.anchorRow = s.row
		s.anchorCol = s.col
	case "n":
		s.search(s.reverse)
	case "shift+n":
		s.search(!s.reverse)
	case "w", "b", "e", "shift+w", "shift+b", "shift+e":
		s.word(key)
	case "{", "}":
		delta := 1
		if key == "{" {
			delta = -1
		}
		s.row += delta
		for s.row > 0 && s.row < len(s.lines)-1 && strings.TrimSpace(s.lines[s.row]) != "" {
			s.row += delta
		}
		s.col = 0
	}
	s.clamp()
	return
}
func (s *copyState) selectionBounds(row int) (int, int) {
	if !s.selecting {
		if s.searchFound && row == s.matchRow {
			return s.matchCol, s.matchCol + s.matchLen
		}
		return -1, -1
	}
	ar, ac, br, bc := s.anchorRow, s.anchorCol, s.row, s.col
	if ar > br || (ar == br && ac > bc) {
		ar, br = br, ar
		ac, bc = bc, ac
	}
	if row < ar || row > br {
		return -1, -1
	}
	n := len([]rune(s.lines[row]))
	lo, hi := 0, n
	if !s.linewise {
		if row == ar {
			lo = min(ac, n)
		}
		if row == br {
			hi = min(bc+1, n)
		}
	}
	return lo, max(lo, hi)
}
func (s *copyState) selectedText() string {
	var lines []string
	for i, line := range s.lines {
		lo, hi := s.selectionBounds(i)
		if lo >= 0 {
			lines = append(lines, string([]rune(line)[lo:hi]))
		}
	}
	return strings.Join(lines, "\n")
}
func (s *copyState) search(reverse bool) {
	s.searchFound = false
	if s.query == "" {
		return
	}
	type point struct{ r, c int }
	var matches []point
	for row, line := range s.lines {
		runes := []rune(line)
		query := []rune(s.query)
		for col := 0; col+len(query) <= len(runes); col++ {
			candidate := string(runes[col : col+len(query)])
			needle := s.query
			if strings.ToLower(s.query) == s.query {
				candidate = strings.ToLower(candidate)
				needle = strings.ToLower(needle)
			}
			if candidate == needle {
				matches = append(matches, point{row, col})
			}
		}
	}
	if len(matches) == 0 {
		return
	}
	found := matches[0]
	if reverse {
		found = matches[len(matches)-1]
		for i := len(matches) - 1; i >= 0; i-- {
			p := matches[i]
			if p.r < s.row || (p.r == s.row && p.c < s.col) {
				found = p
				break
			}
		}
	} else {
		for _, p := range matches {
			if p.r > s.row || (p.r == s.row && p.c > s.col) {
				found = p
				break
			}
		}
	}
	s.row, s.col = found.r, found.c
	s.searchFound = true
	s.matchRow = found.r
	s.matchCol = found.c
	s.matchLen = len([]rune(s.query))
}
func (s *copyState) word(key string) {
	text := []rune(strings.Join(s.lines, "\n"))
	if len(text) == 0 {
		return
	}
	pos := s.col
	for i := 0; i < s.row; i++ {
		pos += len([]rune(s.lines[i])) + 1
	}
	pos = min(pos, len(text)-1)
	big := strings.HasPrefix(key, "shift+")
	key = strings.TrimPrefix(key, "shift+")
	class := func(r rune) int {
		if unicode.IsSpace(r) {
			return 0
		}
		if big || unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			return 1
		}
		return 2
	}
	switch key {
	case "b":
		if pos > 0 {
			pos--
		}
		for pos > 0 && class(text[pos]) == 0 {
			pos--
		}
		for pos > 0 && class(text[pos-1]) == class(text[pos]) {
			pos--
		}
	case "w":
		kind := class(text[pos])
		for pos < len(text)-1 && class(text[pos]) == kind {
			pos++
		}
		for pos < len(text)-1 && class(text[pos]) == 0 {
			pos++
		}
	case "e":
		if pos < len(text)-1 {
			pos++
		}
		for pos < len(text)-1 && class(text[pos]) == 0 {
			pos++
		}
		for pos < len(text)-1 && class(text[pos+1]) == class(text[pos]) {
			pos++
		}
	}
	s.row = 0
	for s.row < len(s.lines)-1 && pos > len([]rune(s.lines[s.row])) {
		pos -= len([]rune(s.lines[s.row])) + 1
		s.row++
	}
	s.col = pos
	s.clamp()
}

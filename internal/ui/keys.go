package ui

import (
	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// The keys the user presses go to the pane that holds the keyboard.
//
// The daemon knows a key by the code of the Rust model, so the window maps
// the keys of the system onto them: the named keys, the function keys, and
// the characters a modifier turns into a chord. The characters themselves
// travel as text, the way the daemon expects them.

const (
	// The size of one cell in the window, before it has been measured.
	// The caret of the terminal needs a place before the first frame.
	estimatedCellWidth = 7.0
)

// paneKey turns a key of the window into the key the daemon knows.
//
// It says no when the key is one the daemon has no code for: the window
// then lets it travel as text.
func paneKey(key ui.Key, mods ui.Modifiers) (herdr.PaneKey, bool) {
	switch key {
	case ui.KeyEnter:
		return herdr.PaneKey{Code: herdr.KeyEnter}, true
	case ui.KeyBackspace:
		return herdr.PaneKey{Code: herdr.KeyBackspace}, true
	case ui.KeyEscape:
		return herdr.PaneKey{Code: herdr.KeyEsc}, true
	case ui.KeyTab:
		if mods&ui.Shift != 0 {
			return herdr.PaneKey{Code: herdr.KeyBackTab}, true
		}
		return herdr.PaneKey{Code: herdr.KeyTab}, true
	case ui.KeyLeft:
		return herdr.PaneKey{Code: herdr.KeyLeft}, true
	case ui.KeyRight:
		return herdr.PaneKey{Code: herdr.KeyRight}, true
	case ui.KeyUp:
		return herdr.PaneKey{Code: herdr.KeyUp}, true
	case ui.KeyDown:
		return herdr.PaneKey{Code: herdr.KeyDown}, true
	case ui.KeyHome:
		return herdr.PaneKey{Code: herdr.KeyHome}, true
	case ui.KeyEnd:
		return herdr.PaneKey{Code: herdr.KeyEnd}, true
	case ui.KeyPageUp:
		return herdr.PaneKey{Code: herdr.KeyPageUp}, true
	case ui.KeyPageDown:
		return herdr.PaneKey{Code: herdr.KeyPageDown}, true
	case ui.KeyDelete:
		return herdr.PaneKey{Code: herdr.KeyDelete}, true
	case ui.KeyInsert:
		return herdr.PaneKey{Code: herdr.KeyInsert}, true
	}
	// The function keys the system reports.
	if key >= ui.KeyF1 && key <= ui.KeyF12 {
		return herdr.PaneKey{
			Code:     herdr.KeyFunction,
			Value:    uint32(key-ui.KeyF1) + 1,
			HasValue: true,
		}, true
	}
	// A character a modifier turns into a chord.
	if mods&(ui.Ctrl|ui.Alt) != 0 {
		r, ok := typedRune(key)
		if !ok {
			return herdr.PaneKey{}, false
		}
		modifiers := uint8(0)
		if mods&ui.Shift != 0 {
			modifiers |= herdr.KeyModShift
		}
		if mods&ui.Ctrl != 0 {
			modifiers |= herdr.KeyModControl
		}
		if mods&ui.Alt != 0 {
			modifiers |= herdr.KeyModAlt
			// The daemon reads the letter as it is typed, so a chord
			// with alt and shift carries the capital letter.
			if mods&ui.Shift != 0 && r >= 'a' && r <= 'z' {
				r -= 'a' - 'A'
			}
		}
		return herdr.PaneKey{
			Code:      herdr.KeyChar,
			Value:     uint32(r),
			HasValue:  true,
			Modifiers: modifiers,
		}, true
	}
	return herdr.PaneKey{}, false
}

// typedRune says the character a key types without modifiers.
func typedRune(key ui.Key) (rune, bool) {
	switch {
	case key >= ui.KeyA && key <= ui.KeyZ:
		return rune('a' + int(key-ui.KeyA)), true
	case key >= ui.Key0 && key <= ui.Key9:
		return rune('0' + int(key-ui.Key0)), true
	}
	switch key {
	case ui.KeySpace:
		return ' ', true
	case ui.KeyMinus:
		return '-', true
	case ui.KeyEqual:
		return '=', true
	case ui.KeyComma:
		return ',', true
	case ui.KeyPeriod:
		return '.', true
	case ui.KeySlash:
		return '/', true
	case ui.KeySemicolon:
		return ';', true
	case ui.KeyQuote:
		return '\'', true
	case ui.KeyBracketLeft:
		return '[', true
	case ui.KeyBracketRight:
		return ']', true
	case ui.KeyBackslash:
		return '\\', true
	case ui.KeyBackquote:
		return '`', true
	}
	return 0, false
}

// terminalKey takes a key of the window for the pane.
//
// The shortcuts of the window come first: the copy of the selection, the
// paste of the clipboard. Every other key goes to the pane.
func (a *App) terminalKey(event ui.InputEvent) bool {
	if event.Mods&ui.Super != 0 {
		switch event.Key {
		case ui.KeyC:
			a.copySelection()
			return true
		case ui.KeyV:
			a.pasteIntoPane()
			return true
		}
		return false
	}
	key, ok := paneKey(event.Key, event.Mods)
	if !ok {
		return false
	}
	// A key that was a chord carries its own text; the text the system
	// sends with it belongs to the same press.
	a.mu.Lock()
	a.suppressText = true
	a.mu.Unlock()
	a.sendKey(key)
	return true
}

// terminalText types text into the pane.
//
// The text a chord produced is dropped, for the chord has already gone to
// the pane as a key.
func (a *App) terminalText(text string) bool {
	if text == "" {
		return false
	}
	a.mu.Lock()
	suppress := a.suppressText
	a.suppressText = false
	a.mu.Unlock()
	if suppress {
		return true
	}
	a.sendTypedText(text)
	return true
}

// sendKey presses one key in the pane that holds the keyboard.
func (a *App) sendKey(key herdr.PaneKey) {
	pane, ok := a.targetPane(a.snapshotOf())
	if !ok {
		a.setStatus("no pane is focused")
		return
	}
	if a.keys == nil {
		a.setStatus("not connected to the daemon")
		return
	}
	if err := a.keys.SendKey(pane.PaneID, key); err != nil {
		a.setStatus("could not send: %v", err)
	}
}

// sendTypedText types text into the pane that holds the keyboard.
func (a *App) sendTypedText(text string) {
	pane, ok := a.targetPane(a.snapshotOf())
	if !ok {
		a.setStatus("no pane is focused")
		return
	}
	if a.keys == nil {
		a.setStatus("not connected to the daemon")
		return
	}
	if err := a.keys.SendText(pane.PaneID, text); err != nil {
		a.setStatus("could not send: %v", err)
	}
}

// pasteIntoPane types the text of the clipboard into the pane.
func (a *App) pasteIntoPane() {
	data, err := mygo.Clipboard.Read(transfer.Text)
	if err != nil {
		a.setStatus("could not read the clipboard: %v", err)
		return
	}
	text, err := data.Read(transfer.Text)
	if err != nil || len(text) == 0 {
		a.setStatus("the clipboard holds no text")
		return
	}
	a.sendTypedText(string(text))
}

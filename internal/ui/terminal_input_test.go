package ui

import (
	"testing"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// fakeInput remembers what the window sends to the panes.
type fakeInput struct {
	texts []string
	keys  []herdr.PaneKey
}

func (f *fakeInput) SendText(paneID, text string) error {
	f.texts = append(f.texts, text)
	return nil
}

func (f *fakeInput) SendKey(paneID string, key herdr.PaneKey) error {
	f.keys = append(f.keys, key)
	return nil
}

func TestTheTerminalFillsTheWindow(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello world"})
	ui.NewTester(app.view, 1100, 700)

	rect := app.terminalBounds()
	if rect.H <= 0 {
		t.Fatalf("the terminal has no height: %+v", rect)
	}
	if rect.W <= 0 {
		t.Fatalf("the terminal has no width: %+v", rect)
	}
	// The sidebar stands to the left of it.
	if rect.X < 200 {
		t.Fatalf("the terminal stands at %v, beside the sidebar", rect.X)
	}
}

func TestTheDragOverTheTerminalChoosesText(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello world", "second line"})
	copied := ""
	app.clipboardWriter = func(text string) error {
		copied = text
		return nil
	}

	tester := ui.NewTester(app.view, 1100, 700)
	cellWidth := app.measuredCellWidth()
	if cellWidth <= 0 {
		t.Fatalf("the window did not measure a cell")
	}
	rect := app.terminalBounds()
	if rect.W <= 0 {
		t.Fatalf("the terminal was not drawn: %+v", rect)
	}

	// The pointer goes down on the first cell and comes up on the sixth.
	tester.Press(rect.X+1, rect.Y+1)
	tester.Move(rect.X+5*cellWidth+1, rect.Y+1)
	tester.Release(rect.X+5*cellWidth+1, rect.Y+1)

	if got := app.selectedText(); got != "hello" {
		t.Fatalf("the selection read %q", got)
	}
	if copied != "hello" {
		t.Fatalf("the clipboard holds %q", copied)
	}
}

func TestTheDragOverTwoLinesChoosesBoth(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello world", "second line"})
	app.clipboardWriter = func(text string) error { return nil }

	tester := ui.NewTester(app.view, 1100, 700)
	cellWidth := app.measuredCellWidth()
	rect := app.terminalBounds()

	tester.Press(rect.X+1, rect.Y+1)
	tester.Move(rect.X+5*cellWidth+1, rect.Y+18+1)
	tester.Release(rect.X+5*cellWidth+1, rect.Y+18+1)

	if got := app.selectedText(); got != "hello world\nsecond" {
		t.Fatalf("the selection read %q", got)
	}
}

func TestTheKeysGoToThePane(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello"})
	input := &fakeInput{}
	app.keys = input

	tester := ui.NewTester(app.view, 1100, 700)
	rect := app.terminalBounds()
	// The click gives the keyboard to the terminal.
	tester.ClickAt(rect.X+5, rect.Y+5)
	tester.Type("hi")

	if len(input.texts) != 1 || input.texts[0] != "hi" {
		t.Fatalf("the pane received %q", input.texts)
	}
}

func TestANamedKeyGoesToThePane(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello"})
	input := &fakeInput{}
	app.keys = input

	tester := ui.NewTester(app.view, 1100, 700)
	rect := app.terminalBounds()
	tester.ClickAt(rect.X+5, rect.Y+5)
	tester.Key(0, ui.KeyEnter)

	if len(input.keys) != 1 {
		t.Fatalf("the pane received %d keys: %+v", len(input.keys), input.keys)
	}
	if input.keys[0].Code != herdr.KeyEnter {
		t.Fatalf("the key came out as %+v", input.keys[0])
	}
}

func TestAChordGoesToThePaneAsAKey(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello"})
	input := &fakeInput{}
	app.keys = input

	tester := ui.NewTester(app.view, 1100, 700)
	rect := app.terminalBounds()
	tester.ClickAt(rect.X+5, rect.Y+5)
	tester.TypeKey(ui.Ctrl, ui.KeyC, "c")

	if len(input.keys) != 1 {
		t.Fatalf("the pane received %d keys: %+v", len(input.keys), input.keys)
	}
	key := input.keys[0]
	if key.Code != herdr.KeyChar || key.Value != 'c' {
		t.Fatalf("the key came out as %+v", key)
	}
	if key.Modifiers&herdr.KeyModControl == 0 {
		t.Fatalf("the key came out as %+v", key)
	}
	// The text that came with the chord belongs to it, and is not sent
	// a second time.
	if len(input.texts) != 0 {
		t.Fatalf("the pane received the text %q as well", input.texts)
	}
}

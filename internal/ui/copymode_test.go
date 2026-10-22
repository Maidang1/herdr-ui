package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTheCopyModeKeysAreTheViOnes(t *testing.T) {
	cases := []struct {
		key  ui.Key
		mods ui.Modifiers
		kind string
	}{
		{key: ui.KeyK, kind: copyStep},
		{key: ui.KeyJ, kind: copyStep},
		{key: ui.KeyH, kind: copyStep},
		{key: ui.KeyL, kind: copyStep},
		{key: ui.KeyUp, kind: copyStep},
		{key: ui.KeyDown, kind: copyStep},
		{key: ui.KeyLeft, kind: copyStep},
		{key: ui.KeyRight, kind: copyStep},
		{key: ui.Key0, kind: copyLineStart},
		{key: ui.KeyHome, kind: copyLineStart},
		{key: ui.KeyEnd, kind: copyMotion},
		{key: ui.KeyW, kind: copyMotion},
		{key: ui.KeyB, kind: copyMotion},
		{key: ui.KeyE, kind: copyMotion},
		{key: ui.KeyG, kind: copyHistory},
		{key: ui.KeyV, kind: copyMark},
		{key: ui.KeyY, kind: copyCopy},
		{key: ui.KeyQ, kind: copyExit},
		{key: ui.KeyEscape, kind: copyCancel},
		{key: ui.KeySlash, kind: copyPrompt},
		{key: ui.KeyN, kind: copyRepeat},
	}
	for _, test := range cases {
		command, ok := copyModeCommand(test.key, test.mods)
		if !ok {
			t.Errorf("the key %v has no copy-mode command", test.key)
			continue
		}
		if command.Kind != test.kind {
			t.Errorf("the key %v asks for %q, want %q", test.key, command.Kind, test.kind)
		}
	}
}

func TestTheCopyModeChordsMoveByAPage(t *testing.T) {
	command, ok := copyModeCommand(ui.KeyU, ui.Ctrl)
	if !ok {
		t.Fatal("the chord has no copy-mode command")
	}
	if command.Kind != copyPage || !command.Half || command.Down {
		t.Fatalf("the chord asks for %+v", command)
	}
	command, ok = copyModeCommand(ui.KeyD, ui.Ctrl)
	if !ok {
		t.Fatal("the chord has no copy-mode command")
	}
	if command.Kind != copyPage || !command.Half || !command.Down {
		t.Fatalf("the chord asks for %+v", command)
	}
}

func TestTheCopyModeReadsTheCharactersOfTheKeyboard(t *testing.T) {
	command, ok := copyModeChar('G')
	if !ok {
		t.Fatal("the character has no copy-mode command")
	}
	if command.Kind != copyHistory || command.Top {
		t.Fatalf("the character asks for %+v", command)
	}
	command, ok = copyModeChar('$')
	if !ok || command.Kind != copyMotion {
		t.Fatalf("the character asks for %+v", command)
	}
}

func TestTheCopyModeWalksTheText(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello", "world"})
	app.pane = "w1:p1"

	app.enterCopyMode()
	if !app.copyModeActive() {
		t.Fatal("the copy mode did not begin")
	}
	state := app.copyModeStateOf()
	if state.paneID != "w1:p1" {
		t.Fatalf("the copy mode walks %q", state.paneID)
	}

	// Two rows down and one column right.
	app.runCopyCommand(copyCommand{Kind: copyStep, Rows: 2})
	app.runCopyCommand(copyCommand{Kind: copyStep, Cols: 1})
	state = app.copyModeStateOf()
	if state.cursor.Row != 2 || state.cursor.Col != 1 {
		t.Fatalf("the cursor came out as %+v", state.cursor)
	}

	// The mark chooses from the cursor back to where it was.
	app.runCopyCommand(copyCommand{Kind: copyMark})
	state = app.copyModeStateOf()
	if !state.hasMark {
		t.Fatalf("the mark was not set: %+v", state)
	}
	app.runCopyCommand(copyCommand{Kind: copyStep, Rows: -1})
	state = app.copyModeStateOf()
	if state.cursor.Row != 1 {
		t.Fatalf("the cursor came out as %+v", state.cursor)
	}
}

func TestTheCopyModeSearchPromptOpens(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello"})
	app.pane = "w1:p1"
	app.enterCopyMode()

	app.runCopyCommand(copyCommand{Kind: copyPrompt, Direction: "forward"})
	state := app.copyModeStateOf()
	if !state.prompt || state.promptDirection != "forward" {
		t.Fatalf("the prompt came out as %+v", state)
	}
	app.appendCopyPrompt("wor")
	state = app.copyModeStateOf()
	if state.promptText != "wor" {
		t.Fatalf("the query came out as %q", state.promptText)
	}
	app.copyPromptBackspace()
	state = app.copyModeStateOf()
	if state.promptText != "wo" {
		t.Fatalf("the query came out as %q", state.promptText)
	}
}

func TestTheCopyModeEndsWithThePane(t *testing.T) {
	app := testApp()
	app.surface = selectionFrame([]string{"hello"})
	app.pane = "w1:p1"
	app.enterCopyMode()
	app.leaveCopyMode()
	if app.copyModeActive() {
		t.Fatal("the copy mode did not end")
	}
}

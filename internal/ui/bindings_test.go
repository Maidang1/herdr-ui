package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAShortcutIsReadTheWayTheRustClientWritesIt(t *testing.T) {
	binding, err := ParseBinding("cmd-shift-c")
	if err != nil {
		t.Fatalf("the shortcut: %v", err)
	}
	if !binding.Cmd || !binding.Shift || binding.Ctrl || binding.Alt {
		t.Fatalf("the modifiers came out as %+v", binding)
	}
	if binding.Key != ui.KeyC {
		t.Fatalf("the key came out as %v", binding.Key)
	}
}

func TestTheShortcutsNameTheKeysThatAreNotLetters(t *testing.T) {
	binding, err := ParseBinding("cmd-comma")
	if err != nil {
		t.Fatalf("the shortcut: %v", err)
	}
	if binding.Key != ui.KeyComma {
		t.Fatalf("the key came out as %v", binding.Key)
	}
	binding, err = ParseBinding("f5")
	if err != nil {
		t.Fatalf("the shortcut: %v", err)
	}
	if binding.Key != ui.KeyF5 {
		t.Fatalf("the key came out as %v", binding.Key)
	}
	binding, err = ParseBinding("alt-left")
	if err != nil {
		t.Fatalf("the shortcut: %v", err)
	}
	if !binding.Alt || binding.Key != ui.KeyLeft {
		t.Fatalf("the shortcut came out as %+v", binding)
	}
}

func TestABrokenShortcutIsRefused(t *testing.T) {
	for _, text := range []string{"", "cmd-", "cmd-nonsense", "nonsense", "cmd-ctrl"} {
		if _, err := ParseBinding(text); err == nil {
			t.Fatalf("%q was taken for a shortcut", text)
		}
	}
}

func TestAShortcutIsWrittenBackAsItIsRead(t *testing.T) {
	for _, text := range []string{"cmd-k", "cmd-shift-a", "alt-left", "f12", "ctrl-c"} {
		binding, err := ParseBinding(text)
		if err != nil {
			t.Fatalf("the shortcut %q: %v", text, err)
		}
		if got := binding.String(); got != text {
			t.Fatalf("the shortcut came out as %q, want %q", got, text)
		}
	}
}

func TestTheShortcutTheUserSetsStandsOverTheDefault(t *testing.T) {
	action, ok := ActionByName("command_palette")
	if !ok {
		t.Fatal("the action is missing")
	}
	if action.Default != "cmd-k" {
		t.Fatalf("the default came out as %q", action.Default)
	}
	binding := bindingOf(action.Name, "cmd-p")
	if binding.Key != ui.KeyP || !binding.Cmd {
		t.Fatalf("the shortcut came out as %+v", binding)
	}
	// A shortcut that cannot be read leaves the default standing.
	binding = bindingOf(action.Name, "nonsense")
	if binding.Key != ui.KeyK {
		t.Fatalf("the shortcut came out as %+v", binding)
	}
}

func TestTheLayoutOfTheSidebarIsRead(t *testing.T) {
	layout := ParseLayout("compact")
	if layout.Density != densityCompact || layout.Style != styleFlat {
		t.Fatalf("the layout came out as %+v", layout)
	}
	layout = ParseLayout("comfortable-rounded")
	if layout.Density != densityComfortable || !layout.Rounded() {
		t.Fatalf("the layout came out as %+v", layout)
	}
	layout = ParseLayout("orca")
	if layout.Density != densityComfortable || !layout.Rounded() {
		t.Fatalf("the layout came out as %+v", layout)
	}
	layout = ParseLayout("minimal")
	if !layout.Minimal {
		t.Fatalf("the layout came out as %+v", layout)
	}
	// A layout the window does not know stands for the default one.
	layout = ParseLayout("no-such-layout")
	if layout.Density != densityNormal || layout.Style != styleFlat {
		t.Fatalf("the layout came out as %+v", layout)
	}
}

func TestTheRowsOfTheSidebarTakeTheRoomTheLayoutAsksFor(t *testing.T) {
	compact := ParseLayout("compact")
	comfortable := ParseLayout("comfortable")
	if compact.rowPadding() >= comfortable.rowPadding() {
		t.Fatalf("the compact rows take %v, the comfortable ones %v",
			compact.rowPadding(), comfortable.rowPadding())
	}
	if compact.rowFontSize(12) >= comfortable.rowFontSize(12) {
		t.Fatalf("the compact text is %v, the comfortable one %v",
			compact.rowFontSize(12), comfortable.rowFontSize(12))
	}
}

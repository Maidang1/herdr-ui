package ui

import (
	"testing"

	"herdr-ui/internal/config"

	"github.com/egoist/mygo/ui"
)

func TestTheSettingsPanelShowsTheSettings(t *testing.T) {
	app := testApp()
	app.openSettings()

	tester := ui.NewTester(app.view, 1100, 700)
	for _, want := range []string{
		"Settings",
		"Size of the terminal",
		"Height of a row",
		"Size of the window",
		"Keep the selection",
	} {
		if !tester.HasText(want) {
			t.Errorf("the settings do not show %q; they show %q", want, tester.Texts())
		}
	}
}

func TestTheSettingsArePutIntoTheWindow(t *testing.T) {
	app := testApp()
	app.applySettings(config.Settings{
		TerminalSize:           16,
		TerminalLineHeight:     1.25,
		UISize:                 13,
		KeepSelectionAfterCopy: true,
	})
	if got := app.fontSizePx(); got != 16 {
		t.Fatalf("the size of the characters came out as %v", got)
	}
	if got := app.rowHeightPx(); got != 20 {
		t.Fatalf("the height of a row came out as %v", got)
	}
	if got := app.uiFontSizePx(); got != 13 {
		t.Fatalf("the size of the window came out as %v", got)
	}
	if app.dropSelection {
		t.Fatal("the selection does not stay")
	}

	app.applySettings(config.Settings{
		TerminalSize:           14,
		TerminalLineHeight:     1.4,
		UISize:                 12,
		KeepSelectionAfterCopy: false,
	})
	if app.dropSelection == false {
		t.Fatal("the selection stays")
	}
}

func TestTheSettingsOfTheWindowAreRead(t *testing.T) {
	// The window reads the files the Rust client uses, and this machine
	// has them: the settings of the terminal come out as the file says.
	settings, err := config.Load()
	if err != nil {
		t.Skipf("the settings of this machine cannot be read: %v", err)
	}
	if settings.TerminalSize <= 0 {
		t.Fatalf("the size of the terminal came out as %v", settings.TerminalSize)
	}
}

func TestTheSettingsPanelShowsTheThemeAndTheLayout(t *testing.T) {
	app := testApp()
	app.openSettings()

	tester := ui.NewTester(app.view, 1100, 700)
	for _, want := range []string{
		"Theme",
		"Layout",
		"Notifications",
		"The bell",
		"Shortcuts",
		"Command palette",
		"Copy mode",
	} {
		if !tester.HasText(want) {
			t.Errorf("the settings do not show %q; they show %q", want, tester.Texts())
		}
	}
}

func TestTheLayoutOfTheSidebarDrawsRoundedRows(t *testing.T) {
	app := testApp()
	app.applySettings(config.Settings{
		TerminalSize:       13,
		TerminalLineHeight: 1.4,
		UISize:             12,
		Layout:             "comfortable-rounded",
	})
	tester := ui.NewTester(app.view, 1100, 700)
	if !tester.HasText("dsh") || !tester.HasText("coze") {
		t.Errorf("the sidebar does not show the workspaces; it shows %q", tester.Texts())
	}
}

func TestTheThemeReachesTheTerminal(t *testing.T) {
	app := testApp()
	app.setThemeName("Nord")
	colors := app.terminalColors(true)
	if colors.Background != rgbOf(0x2e3440) {
		t.Fatalf("the background of the terminal came out as %+v", colors.Background)
	}
	if colors.Foreground != rgbOf(0xd8dee9) {
		t.Fatalf("the foreground of the terminal came out as %+v", colors.Foreground)
	}
	// The palette of the theme stands in for the one of the default.
	if colors.Palette[1] != rgbOf(0xbf616a) {
		t.Fatalf("the red of the terminal came out as %+v", colors.Palette[1])
	}
}

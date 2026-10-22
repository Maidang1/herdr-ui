package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheSettingsOfTheWindowAreRead(t *testing.T) {
	folder := t.TempDir()
	root := filepath.Join(folder, "herdr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", folder)

	defaults := `
# The managed defaults.
[terminal]
size = 14.0
line_height = 1.3

[ui]
size = 11.0
`
	local := `
# The user's overrides.
keep_selection_after_copy = true
theme = "gruvbox"

[terminal]
size = 16.0
`
	if err := os.WriteFile(filepath.Join(root, "config-gpui.toml"), []byte(defaults), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config-gpui.local.toml"), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}

	settings, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if settings.TerminalSize != 16 {
		t.Fatalf("the size of the terminal came out as %v", settings.TerminalSize)
	}
	if settings.TerminalLineHeight != 1.3 {
		t.Fatalf("the height of a row came out as %v", settings.TerminalLineHeight)
	}
	if settings.UISize != 11 {
		t.Fatalf("the size of the window came out as %v", settings.UISize)
	}
	if !settings.KeepSelectionAfterCopy {
		t.Fatal("the selection is not kept")
	}
	if settings.Theme != "gruvbox" {
		t.Fatalf("the theme came out as %q", settings.Theme)
	}
}

func TestTheSettingsFallBackWhenTheFilesSayNothing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if settings.TerminalSize != DefaultTerminalSize {
		t.Fatalf("the size of the terminal came out as %v", settings.TerminalSize)
	}
	if settings.UISize != DefaultUISize {
		t.Fatalf("the size of the window came out as %v", settings.UISize)
	}
	if !settings.KeepSelectionAfterCopy {
		t.Fatal("the selection is not kept")
	}
}

func TestAKeyIsWrittenOverTheOneThatIsThere(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "config-gpui.local.toml")
	text := "# The overrides.\n\n[terminal]\nsize = 14.0\n# a comment about the size\nline_height = 1.4\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setKey(path, "terminal.size", "16"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# The overrides.\n\n[terminal]\nsize = 16\n# a comment about the size\nline_height = 1.4\n"
	if string(got) != want {
		t.Fatalf("the file came out as\n%q\nwant\n%q", got, want)
	}
}

func TestAKeyIsAddedToItsTable(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "config-gpui.local.toml")
	text := "# The overrides.\n\n[terminal]\nsize = 14.0\n\n[ui]\nsize = 12.0\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setKey(path, "terminal.line_height", "1.3"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# The overrides.\n\n[terminal]\nsize = 14.0\nline_height = 1.3\n\n[ui]\nsize = 12.0\n"
	if string(got) != want {
		t.Fatalf("the file came out as\n%q\nwant\n%q", got, want)
	}
}

func TestATableThatIsNotThereIsAdded(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "config-gpui.local.toml")
	text := "# The overrides.\n\nlayout = \"normal\"\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setKey(path, "terminal.size", "15"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# The overrides.\n\nlayout = \"normal\"\n\n[terminal]\nsize = 15\n"
	if string(got) != want {
		t.Fatalf("the file came out as\n%q\nwant\n%q", got, want)
	}
}

func TestAKeyOfTheTopIsAdded(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "config-gpui.local.toml")
	if err := setKey(path, "keep_selection_after_copy", "true"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "keep_selection_after_copy = true" {
		t.Fatalf("the file came out as %q", got)
	}
}

func TestTheValueOfASettingIsWrittenAsTOML(t *testing.T) {
	cases := []struct {
		value any
		want  string
	}{
		{value: "normal", want: `"normal"`},
		{value: true, want: "true"},
		{value: 16, want: "16"},
		{value: 16.5, want: "16.5"},
	}
	for _, test := range cases {
		got, err := formatValue(test.value)
		if err != nil {
			t.Fatalf("format: %v", err)
		}
		if got != test.want {
			t.Fatalf("the value came out as %q, want %q", got, test.want)
		}
	}
}

func TestTheLayoutAndTheNotificationsAreRead(t *testing.T) {
	folder := t.TempDir()
	root := filepath.Join(folder, "herdr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", folder)
	local := `
layout = "comfortable-rounded"
theme = "Nord"

[notifications]
enabled = false
finished = false

[bell]
attention = false
sound = true

[sound]
volume = 80
`
	if err := os.WriteFile(filepath.Join(root, "config-gpui.local.toml"), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if settings.Layout != "comfortable-rounded" {
		t.Fatalf("the layout came out as %q", settings.Layout)
	}
	if settings.Theme != "Nord" {
		t.Fatalf("the theme came out as %q", settings.Theme)
	}
	if settings.NotificationsEnabled {
		t.Fatal("the notices are shown")
	}
	if settings.NotificationsFinished {
		t.Fatal("the notices of the finished tasks are shown")
	}
	if settings.BellAttention {
		t.Fatal("the icon bounces")
	}
	if !settings.BellSound {
		t.Fatal("the bell plays no sound")
	}
	if settings.SoundVolume != 80 {
		t.Fatalf("the loudness came out as %v", settings.SoundVolume)
	}
}

func TestTheShortcutsOfTheUserAreRead(t *testing.T) {
	folder := t.TempDir()
	root := filepath.Join(folder, "herdr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", folder)
	local := `
[keybindings]
command_palette = "cmd-p"
find = ["cmd-f", "cmd-g"]
`
	if err := os.WriteFile(filepath.Join(root, "config-gpui.local.toml"), []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if settings.Keybindings["command_palette"] != "cmd-p" {
		t.Fatalf("the shortcuts came out as %v", settings.Keybindings)
	}
	if settings.Keybindings["find"] != "cmd-f,cmd-g" {
		t.Fatalf("the shortcuts came out as %v", settings.Keybindings)
	}
}

func TestTheShortcutOfAnActionIsWrittenAndTakenOutAgain(t *testing.T) {
	folder := t.TempDir()
	root := filepath.Join(folder, "herdr")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", folder)
	if err := SetLocalKeybinding("command_palette", "cmd-p"); err != nil {
		t.Fatalf("set: %v", err)
	}
	path := filepath.Join(root, "config-gpui.local.toml")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "[keybindings]") ||
		!strings.Contains(string(got), `command_palette = "cmd-p"`) {
		t.Fatalf("the file came out as %q", got)
	}
	// A shortcut that is taken out leaves no empty table behind.
	if err := SetLocalKeybinding("command_palette", ""); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "command_palette") {
		t.Fatalf("the file came out as %q", got)
	}
}

func TestAKeyThatIsTakenOutLeavesTheRestOfItsTable(t *testing.T) {
	folder := t.TempDir()
	path := filepath.Join(folder, "config-gpui.local.toml")
	text := "# The overrides.\n\n[keybindings]\n# the palette of the commands\ncommand_palette = \"cmd-p\"\nfind = \"cmd-f\"\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := removeKey(path, "keybindings.command_palette"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "# The overrides.\n\n[keybindings]\n# the palette of the commands\nfind = \"cmd-f\"\n"
	if string(got) != want {
		t.Fatalf("the file came out as\n%q\nwant\n%q", got, want)
	}
}

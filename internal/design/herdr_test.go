package design

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func herdrTestConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestHerdrAutoSwitchAndOverridePrecedence(t *testing.T) {
	path := herdrTestConfig(t, `
[theme]
name = "tokyo-night"
auto_switch = true
[ui]
accent = "#123456"
[theme.custom]
text = "#778899"
[theme.custom.light]
accent = "#abcdef"
yellow = "#112233"
[theme.custom.dark]
sidebar_bg = "#456789"
`)
	light, dark, err := HerdrThemes(path, "herdr")
	if err != nil {
		t.Fatal(err)
	}
	if light.Theme.Dark || !dark.Theme.Dark {
		t.Fatal("expected Tokyo Night light/dark siblings")
	}
	if light.Theme.Background != ui.Hex("#e1e2e7") || dark.Theme.Background != ui.Hex("#1a1b26") {
		t.Fatal("wrong sibling backgrounds")
	}
	if light.Theme.Accent != ui.Hex("#abcdef") || dark.Theme.Accent != ui.Hex("#123456") {
		t.Fatal("mode accent should override legacy ui.accent")
	}
	if light.Theme.Text != ui.Hex("#778899") || dark.Theme.Text != ui.Hex("#778899") {
		t.Fatal("global custom text must apply to both modes")
	}
	if light.Colors.Working != ui.Hex("#112233") || dark.Colors.Sidebar != ui.Hex("#456789") {
		t.Fatal("status and sidebar tokens were lost")
	}

	path = herdrTestConfig(t, `
[theme]
auto_switch = true
[ui]
accent = "#123456"
[theme.custom]
accent = "#aabbcc"
[theme.custom.light]
accent = "#fedcba"
`)
	light, dark, err = HerdrThemes(path, "herdr")
	if err != nil {
		t.Fatal(err)
	}
	if light.Theme.Accent != ui.Hex("#fedcba") || dark.Theme.Accent != ui.Hex("#aabbcc") {
		t.Fatal("global custom accent must disable legacy ui.accent")
	}
}

func TestHerdrFixedPaletteTracksActualAppearance(t *testing.T) {
	path := herdrTestConfig(t, `
[theme]
name = "nord"
[theme.custom]
accent = "#123456"
[theme.custom.light]
accent = "#abcdef"
`)
	light, dark, err := HerdrThemes(path, "herdr")
	if err != nil {
		t.Fatal(err)
	}
	if !light.Theme.Dark || !dark.Theme.Dark {
		t.Fatal("fixed dark palette must stay dark in both system slots")
	}
	if *light.Theme != *dark.Theme || light.Colors != dark.Colors {
		t.Fatal("manual theme should resolve identically in both slots")
	}
	if light.Theme.Accent != ui.Hex("#123456") {
		t.Fatal("mode overrides must not apply when auto_switch is off")
	}
	light, dark, err = HerdrThemes(path, "catppuccin-latte")
	if err != nil {
		t.Fatal(err)
	}
	if light.Theme.Dark || dark.Theme.Dark {
		t.Fatal("explicit light palette must stay light in both system slots")
	}
	if light.Theme.Accent != ui.Hex("#1e66f5") {
		t.Fatal("explicit built-in must ignore config")
	}
}

func TestHerdrFallbacksAndTerminalDefaults(t *testing.T) {
	path := herdrTestConfig(t, `
[theme]
auto_switch = true
dark_name = "invalid"
light_name = "invalid"
`)
	light, dark, err := HerdrThemes(path, "herdr")
	if err != nil {
		t.Fatal(err)
	}
	if light.Theme.Background != ui.Hex("#eff1f5") || dark.Theme.Background != ui.Hex("#181825") {
		t.Fatal("unknown config themes should use upstream appearance fallbacks")
	}
	light, dark, err = HerdrThemes(path, "terminal")
	if err != nil {
		t.Fatal(err)
	}
	if *light.Theme != *NativeTheme(false, false) || *dark.Theme != *NativeTheme(true, false) {
		t.Fatal("terminal should inherit native defaults")
	}
	if light.Colors != Colors(false) || dark.Colors != Colors(true) {
		t.Fatal("terminal design colors should use native defaults")
	}
}

func TestHerdrColorParsing(t *testing.T) {
	fallback := ui.Hex("#123456")
	for _, value := range []string{"reset", "default", "none", "transparent"} {
		if got := parseHerdrColor(value, fallback); got != fallback {
			t.Errorf("%s: got %v", value, got)
		}
	}
	for value, want := range map[string]ui.Color{
		"#aBc": ui.Hex("#aabbcc"), "rgb(1, 2, 3)": ui.RGB(1, 2, 3),
		"purple": ui.Hex("#800080"), "not-a-color": ui.Hex("#008080"),
		"#12345678": ui.Hex("#008080"),
	} {
		if got := parseHerdrColor(value, fallback); got != want {
			t.Errorf("%s: got %v, want %v", value, got, want)
		}
	}
}

func TestHerdrModeSlotsRemainIndependentOfPaletteDarkness(t *testing.T) {
	path := herdrTestConfig(t, `
[theme]
auto_switch = true
light_name = "catppuccin"
dark_name = "catppuccin-latte"
[theme.custom.light]
accent = "#123456"
[theme.custom.dark]
accent = "#abcdef"
`)
	light, dark, err := HerdrThemes(path, "herdr")
	if err != nil {
		t.Fatal(err)
	}
	if !light.Theme.Dark || dark.Theme.Dark {
		t.Fatal("Dark must describe the palette background")
	}
	if light.Theme.Accent != ui.Hex("#123456") || dark.Theme.Accent != ui.Hex("#abcdef") {
		t.Fatal("mode custom must follow system slot rather than palette darkness")
	}
}

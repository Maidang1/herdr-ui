package ui

import (
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

func TestColorSchemeAppliesToWindowAndSettings(t *testing.T) {
	w := settingsWindow(t)
	light, dark, err := design.HerdrThemes("", "gruvbox")
	if err != nil {
		t.Fatal(err)
	}
	w.SetActionHooks(ActionHooks{ColorScheme: func(name string) error {
		w.SetColorThemes(light, dark)
		return nil
	}})
	w.chooseColorScheme("gruvbox")
	for _, view := range []func(*native.Context){w.View, w.SettingsView} {
		var got *native.Theme
		tester := native.NewTester(func(c *native.Context) { view(c); got = c.Theme() }, 1000, 900)
		tester.Frame()
		if got.Background != light.Theme.Background || got.Text != light.Theme.Text || got.Dark != light.Theme.Dark {
			t.Fatalf("theme did not reach view: %+v", got)
		}
	}
	w.SetColorThemes(nil, nil)
	tester := native.NewTester(func(c *native.Context) {
		p := w.appearancePalette(c)
		if p.Content != design.Colors(false).Content {
			t.Fatal("native colors not restored")
		}
	}, 720, 520)
	tester.Frame()
	if w.settings.snapshot.ColorScheme != "gruvbox" {
		t.Fatal("selection lost")
	}
}

func TestColorSchemeErrorKeepsAppliedChoice(t *testing.T) {
	w := settingsWindow(t)
	w.SetActionHooks(ActionHooks{ColorScheme: func(string) error { return errors.New("invalid configuration") }})
	w.chooseColorScheme("gruvbox")
	if w.settings.snapshot.ColorScheme != "herdr" || w.settings.schemeError == "" {
		t.Fatal("failed choice replaced active theme or hid error")
	}
}

func TestRenderHerdrColors(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export color renders")
	}
	for _, name := range []string{"gruvbox", "catppuccin-latte"} {
		w := settingsWindow(t)
		light, dark, err := design.HerdrThemes("", name)
		if err != nil {
			t.Fatal(err)
		}
		w.SetColorThemes(light, dark)
		w.chooseColorScheme(name)
		for page, view := range map[string]func(*native.Context){"appearance": w.SettingsView, "window": w.View} {
			tester := native.NewTester(view, 1000, 800)
			tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
			tester.Frame()
			file, err := os.Create(filepath.Join(dir, fmt.Sprintf("colors-%s-%s.png", name, page)))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(file, tester.Image())
			file.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFollowHerdrSwitchRestoresManualScheme(t *testing.T) {
	w := settingsWindow(t)
	var choices []string
	w.SetActionHooks(ActionHooks{ColorScheme: func(name string) error { choices = append(choices, name); return nil }})
	tester := native.NewTester(w.SettingsView, 1000, 800)
	if err := tester.Click("Hergo Dark"); err != nil {
		t.Fatal(err)
	}
	if w.settings.snapshot.Theme != "system" {
		t.Fatal("appearance changed while following Herdr")
	}
	if err := tester.Click("Follow Herdr"); err != nil {
		t.Fatal(err)
	}
	if w.settings.snapshot.ColorScheme != "native" {
		t.Fatal("switch off should enable native theme")
	}
	tester.Frame()
	if err := tester.Click("catppuccin"); err != nil {
		t.Fatal(err)
	}
	if w.settings.snapshot.ColorScheme != "catppuccin" {
		t.Fatal("theme card did not apply choice")
	}
	tester.Frame()
	if err := tester.Click("Follow Herdr"); err != nil {
		t.Fatal(err)
	}
	if w.settings.snapshot.ColorScheme != "herdr" {
		t.Fatal("switch on did not follow Herdr")
	}
	if err := tester.Click("Follow Herdr"); err != nil {
		t.Fatal(err)
	}
	if w.settings.snapshot.ColorScheme != "catppuccin" {
		t.Fatal("manual choice was lost")
	}
	if len(choices) != 4 {
		t.Fatalf("unexpected theme changes: %v", choices)
	}
}

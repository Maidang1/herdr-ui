package ui

import (
	"testing"

	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

func settingsWindow(t *testing.T) *Window {
	t.Helper()
	w := referenceWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Theme: "system", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, Notifications: true})
	return w
}

func TestSettingsPagesNavigate(t *testing.T) {
	w := settingsWindow(t)
	tester := native.NewTester(w.SettingsView, 720, 520)
	for _, page := range settingsPages {
		if _, ok := tester.Find(page); !ok {
			t.Fatalf("settings nav missing page %q", page)
		}
	}
	if err := tester.Click("Terminal"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Font", "Size", "Weight", "Line height", "Preview", "Selection", "Minimum contrast"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("terminal page missing %q", label)
		}
	}
	if err := tester.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Find("Edit Herdr configuration"); !ok {
		t.Fatal("shortcuts page lost the config editor entry")
	}
}

func TestSettingsThemeChoiceCallsHook(t *testing.T) {
	w := settingsWindow(t)
	var chosen string
	w.SetActionHooks(ActionHooks{Theme: func(source string) { chosen = source }})
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if chosen != "dark" {
		t.Fatalf("theme hook saw %q", chosen)
	}
	if w.settings.snapshot.Theme != "dark" {
		t.Fatal("working copy did not follow the choice")
	}
}

func TestSettingsCopyOnSelectTogglesHook(t *testing.T) {
	w := settingsWindow(t)
	var enabled bool
	w.SetActionHooks(ActionHooks{CopyOnSelect: func(on bool) { enabled = on }})
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Terminal"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Copy on select"); err != nil {
		t.Fatal(err)
	}
	if !w.settings.snapshot.CopyOnSelect || !enabled {
		t.Fatal("copy-on-select did not flip and report")
	}
}

func TestSettingsZoomSharesTheFontHook(t *testing.T) {
	w := populatedChromeWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Theme: "system", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, Notifications: true})
	var last terminal.Font
	w.SetActionHooks(ActionHooks{Font: func(f terminal.Font) { last = f }})
	w.Dispatch("zoom_in")
	if last.Size != 14 || last.Weight != 400 {
		t.Fatalf("zoom lost the rest of the font: %+v", last)
	}
	if w.settings.snapshot.Font.Size != 14 {
		t.Fatal("zoom did not update the settings copy")
	}
}

func TestBindingsSourceStillRender(t *testing.T) {
	w := settingsWindow(t)
	if _, err := input.Resolve(nil, "test"); err != nil {
		t.Fatal(err)
	}
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("Prefix chords load from Herdr's own configuration") {
		t.Fatalf("shortcuts page explanation missing: %v", tester.Texts())
	}
}

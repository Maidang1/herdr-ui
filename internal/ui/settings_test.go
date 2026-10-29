package ui

import (
	"runtime"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

func settingsWindow(t *testing.T) *Window {
	t.Helper()
	w := referenceWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Theme: "system", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, Toast: ToastSettings{Delivery: "herdr", Position: "bottom-right", DelaySeconds: 1}})
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
	for _, label := range []string{"Font", "Family", "Size", "Weight", "Line height", "Preview", "Cursor", "Blink", "Input", "Option as Meta", "Copy on select", "Readability", "Minimum contrast"} {
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
	tester := native.NewTester(w.SettingsView, 720, 1200)
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

func TestSettingsCursorAndOptionKeyCallHooks(t *testing.T) {
	w := settingsWindow(t)
	var style string
	blink, meta := true, false
	w.SetInitialSettings(SettingsSnapshot{Theme: "system", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, CursorBlink: true})
	w.SetActionHooks(ActionHooks{Cursor: func(s string, b bool) { style, blink = s, b }, OptionAsAlt: func(on bool) { meta = on }})
	tester := native.NewTester(w.SettingsView, 720, 1200)
	if err := tester.Click("Terminal"); err != nil {
		t.Fatal(err)
	}
	if !w.settings.snapshot.CursorBlink || w.settings.snapshot.Cursor != "block" {
		t.Fatalf("cursor defaults = %q, blink %v", w.settings.snapshot.Cursor, w.settings.snapshot.CursorBlink)
	}
	for _, label := range []string{"Bar", "Blink", "Option as Meta"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if style != "bar" || blink || !meta {
		t.Fatalf("hooks saw cursor %q blink %v, option as meta %v", style, blink, meta)
	}
}

func TestFontStaysInThePreviewUntilApply(t *testing.T) {
	w := settingsWindow(t)
	var pushed []float32
	w.SetActionHooks(ActionHooks{Font: func(f terminal.Font) { pushed = append(pushed, f.Size) }})
	w.settings.snapshot.Font.Size = 18
	tester := native.NewTester(w.SettingsView, 720, 900)
	if err := tester.Click("Terminal"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("18 pt") {
		t.Fatalf("size is not a whole number of points: %v", tester.Texts())
	}
	if len(pushed) != 0 {
		t.Fatal("changing the preview pushed the font to terminals")
	}
	if err := tester.Click("Apply"); err != nil {
		t.Fatal(err)
	}
	if len(pushed) != 1 || pushed[0] != 18 {
		t.Fatalf("apply sent %+v", pushed)
	}
	if w.settings.fontPending() {
		t.Fatal("apply left the draft pending")
	}
}

func TestSettingsZoomSharesTheFontHook(t *testing.T) {
	w := populatedChromeWindow(t)
	w.SetInitialSettings(SettingsSnapshot{Theme: "system", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, Toast: ToastSettings{Delivery: "herdr", Position: "bottom-right", DelaySeconds: 1}})
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
	bindings, err := input.Resolve(nil, "portable-shortcuts-test")
	if err != nil {
		t.Fatal(err)
	}
	w.ConfigureBindings(bindings, nil)
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Shortcuts"); err != nil {
		t.Fatal(err)
	}
	guidance := "Prefix chords come from Herdr’s own configuration"
	if runtime.GOOS != "darwin" {
		guidance = "Herdr’s configured prefix chords remain available"
	}
	if !tester.HasText(guidance) {
		t.Fatalf("shortcuts page explanation missing: %v", tester.Texts())
	}
	if !tester.HasText("portable-shortcuts-test") {
		t.Fatalf("binding source missing: %v", tester.Texts())
	}
	if runtime.GOOS == "windows" && !tester.HasText("Ctrl+Alt remains available for AltGr text") {
		t.Fatalf("Windows key profile guidance missing: %v", tester.Texts())
	}
}

func TestSettingsNotificationDeliveryFollowsHerdrChoices(t *testing.T) {
	w := settingsWindow(t)
	var got []ToastSettings
	w.SetActionHooks(ActionHooks{Notifications: func(s ToastSettings) { got = append(got, s) }})
	tester := native.NewTester(w.SettingsView, 720, 520)
	if err := tester.Click("Notifications"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Off", "In-app", "System", "Position", "Delay"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("notifications page missing %q", label)
		}
	}
	// Position is a pop-up: open it on its current corner, then choose.
	for _, label := range []string{"Bottom right", "Top left"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if err := tester.Click("System"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Position != "top-left" || got[1] != (ToastSettings{Delivery: "system", Position: "top-left", DelaySeconds: 1}) {
		t.Fatalf("notification hook saw %+v", got)
	}
	if _, ok := tester.Find("Position"); ok {
		t.Fatal("system delivery still offers an in-app position")
	}
	if err := tester.Click("Off"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Find("Delay"); ok || got[len(got)-1].Delivery != "off" {
		t.Fatal("off still offers a delay or was not reported", got)
	}
}

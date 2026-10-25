package prefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMissingFileReturnsDefaults(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "nested", "prefs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Settings(); got != Defaults() {
		t.Fatalf("missing file changed settings: %+v", got)
	}
}

func TestUpdatePersistsAndReloads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(func(v *Settings) { v.WindowTheme = "dark"; v.FontSize = 15 }); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := again.Settings()
	if got.WindowTheme != "dark" || got.FontSize != 15 {
		t.Fatalf("round trip lost values: %+v", got)
	}
}

func TestTerminalBehaviorRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Settings(); got.FontFamily != "" || got.OptionAsAlt || got.Cursor != "block" || !got.CursorBlink {
		t.Fatalf("defaults = %+v", got)
	}
	if _, err := s.Update(func(v *Settings) { v.FontFamily, v.OptionAsAlt, v.Cursor, v.CursorBlink = "Menlo", true, "bar", false }); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Settings(); got.FontFamily != "Menlo" || !got.OptionAsAlt || got.Cursor != "bar" || got.CursorBlink {
		t.Fatalf("round trip lost values: %+v", got)
	}
	if err := os.WriteFile(path, []byte(`{"cursor_style":"beam"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if invalid, err := Open(path); err != nil || invalid.Settings().Cursor != "block" {
		t.Fatalf("invalid cursor accepted: %+v, %v", invalid.Settings(), err)
	}
}

func TestInvalidValuesFallBackToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte(`{"window_theme":"neon","font_size":99,"future":"kept"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Settings()
	if got.WindowTheme != "system" || got.FontSize != 13 {
		t.Fatalf("invalid values accepted: %+v", got)
	}
	persisted, err := s.Update(func(*Settings) {})
	if err != nil {
		t.Fatal(err)
	}
	if persisted != got {
		t.Fatalf("update lost fallback values: %+v", persisted)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || !contains(string(data), `"future"`) {
		t.Fatalf("unknown fields were dropped: %s", data)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func TestToastFollowsHerdrUntilChosenHere(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(config, []byte("[ui.toast]\ndelivery = \"system\"\ndelay_seconds = 3\n[ui.toast.herdr]\nposition = \"top-left\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "prefs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.InheritToast(HerdrToast(config))
	if got := s.Settings().Toast; got != (Toast{Delivery: "system", Position: "top-left", DelaySeconds: 3}) {
		t.Fatalf("Herdr's toast settings not inherited: %+v", got)
	}
	// An unrelated change must not pin the inherited values.
	if _, err := s.Update(func(v *Settings) { v.FontSize = 15 }); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(func(v *Settings) { v.Toast.Delivery = "herdr" }); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	again.InheritToast(Toast{Delivery: "off", Position: "bottom-left", DelaySeconds: 0})
	if got := again.Settings().Toast; got != (Toast{Delivery: "herdr", Position: "bottom-left", DelaySeconds: 0}) {
		t.Fatalf("chosen delivery lost or unchosen values pinned: %+v", got)
	}
}

func TestHerdrToastReadsLegacyAndIgnoresInvalid(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.toml")
	for text, want := range map[string]Toast{
		"[ui.toast]\nenabled = true\n":                                {Delivery: "herdr", DelaySeconds: -1},
		"[ui.toast]\ndelivery = \"terminal\"\n":                       {Delivery: "herdr", DelaySeconds: -1},
		"[ui.toast]\ndelivery = \"loud\"\ndelay_seconds = 9999\n":     {DelaySeconds: -1},
		"[ui.toast.herdr]\nposition = \"middle\"\n[keys]\nprefix = 1": {DelaySeconds: -1},
		"not toml [": {DelaySeconds: -1},
	} {
		if err := os.WriteFile(config, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := HerdrToast(config); got != want {
			t.Fatalf("%q read as %+v", text, got)
		}
	}
	if got := HerdrToast(filepath.Join(dir, "missing.toml")); got != (Toast{DelaySeconds: -1}) {
		t.Fatalf("missing config read as %+v", got)
	}
}

func TestLegacyNotificationsOffMeansDeliveryOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte(`{"notifications":false}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s.InheritToast(Toast{Delivery: "system", DelaySeconds: -1})
	if got := s.Settings().Toast.Delivery; got != "off" {
		t.Fatalf("the old off switch was not kept: %q", got)
	}
}

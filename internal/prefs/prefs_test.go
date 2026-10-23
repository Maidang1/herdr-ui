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

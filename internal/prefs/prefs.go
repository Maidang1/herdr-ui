// Package prefs persists herdr-ui's own window settings — the appearance
// choice and the terminal font size — apart from Herdr's keybinding config,
// which stays in Herdr's own file.
package prefs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings is everything herdr-ui remembers between launches. Unknown
// fields in a newer file are kept on load so a downgrade does not erase
// them.
type Settings struct {
	// WindowTheme is the app appearance: "system", "light" or "dark".
	WindowTheme string
	// FontSize is the terminal font size in DIPs.
	FontSize float32
	// FontWeight is the terminal text weight, 100–900; 400 is regular.
	FontWeight int
	// LineHeight is the terminal row height as a multiple of the font's
	// own, 1–2.
	LineHeight float32
	// CopyOnSelect copies a pointer selection to the clipboard as it ends.
	CopyOnSelect bool
	// MinimumContrast is the least text contrast a terminal keeps, 1–21;
	// 1 leaves colors as they come.
	MinimumContrast float32
	// Notifications reports agent status changes through the system.
	// Absent in the file means true, so an upgrade keeps them on.
	Notifications bool
}

// Defaults returns the settings a fresh installation starts with.
func Defaults() Settings {
	return Settings{WindowTheme: "system", FontSize: 13, FontWeight: 400, LineHeight: 1, MinimumContrast: 1, Notifications: true}
}

// Store loads and saves the settings file. Load applies defaults for
// missing or invalid values rather than failing the launch.
type Store struct {
	mu     sync.Mutex
	path   string
	values Settings
	extra  map[string]json.RawMessage
}

func pathFor(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "herdr-ui", "prefs.json"), nil
}

// Open loads the settings at path, or the default location when empty. A
// missing file is not an error: the defaults come back ready to save.
func Open(path string) (*Store, error) {
	resolved, err := pathFor(path)
	if err != nil {
		return nil, err
	}
	s := &Store{path: resolved, values: Defaults()}
	data, err := os.ReadFile(resolved)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prefs %s: %w", resolved, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return s, nil
	}
	s.extra = raw
	if v, ok := raw["window_theme"]; ok {
		var theme string
		if json.Unmarshal(v, &theme) == nil {
			switch theme {
			case "system", "light", "dark":
				s.values.WindowTheme = theme
			}
		}
	}
	if v, ok := raw["font_size"]; ok {
		var size float64
		if json.Unmarshal(v, &size) == nil && size >= 8 && size <= 48 {
			s.values.FontSize = float32(size)
		}
	}
	if v, ok := raw["font_weight"]; ok {
		var weight int
		if json.Unmarshal(v, &weight) == nil && weight >= 100 && weight <= 900 {
			s.values.FontWeight = weight
		}
	}
	if v, ok := raw["line_height"]; ok {
		var height float64
		if json.Unmarshal(v, &height) == nil && height >= 1 && height <= 2 {
			s.values.LineHeight = float32(height)
		}
	}
	if v, ok := raw["copy_on_select"]; ok {
		var copy bool
		if json.Unmarshal(v, &copy) == nil {
			s.values.CopyOnSelect = copy
		}
	}
	if v, ok := raw["minimum_contrast"]; ok {
		var contrast float64
		if json.Unmarshal(v, &contrast) == nil && contrast >= 1 && contrast <= 21 {
			s.values.MinimumContrast = float32(contrast)
		}
	}
	if v, ok := raw["notifications"]; ok {
		var enabled bool
		if json.Unmarshal(v, &enabled) == nil {
			s.values.Notifications = enabled
		}
	} else {
		s.values.Notifications = true
	}
	return s, nil
}

// Settings returns the loaded values.
func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.values
}

// Update applies fn to the settings and persists the result. It returns
// the persisted values. A failed save is reported but leaves the in-memory
// values applied, so the session keeps the user's choice.
func (s *Store) Update(fn func(*Settings)) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.values)
	if s.extra == nil {
		s.extra = map[string]json.RawMessage{}
	}
	document := s.extra
	document["window_theme"], _ = json.Marshal(s.values.WindowTheme)
	document["font_size"], _ = json.Marshal(s.values.FontSize)
	document["font_weight"], _ = json.Marshal(s.values.FontWeight)
	document["line_height"], _ = json.Marshal(s.values.LineHeight)
	document["copy_on_select"], _ = json.Marshal(s.values.CopyOnSelect)
	document["minimum_contrast"], _ = json.Marshal(s.values.MinimumContrast)
	document["notifications"], _ = json.Marshal(s.values.Notifications)
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return s.values, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return s.values, err
	}
	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, append(data, '\n'), 0o644); err != nil {
		return s.values, err
	}
	if err := os.Rename(temp, s.path); err == nil {
		return s.values, nil
	}
	// A rename across devices cannot happen within one directory, but a
	// replaced file can; fall back to a direct write for exotic setups.
	err = os.WriteFile(s.path, append(data, '\n'), 0o644)
	return s.values, err
}

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
}

// Defaults returns the settings a fresh installation starts with.
func Defaults() Settings { return Settings{WindowTheme: "system", FontSize: 13} }

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
		if json.Unmarshal(v, &size) == nil && size >= 8 && size <= 36 {
			s.values.FontSize = float32(size)
		}
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

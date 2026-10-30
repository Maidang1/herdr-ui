// Package prefs persists herdr-ui's own window settings — the appearance
// choice and the terminal's font, cursor and keyboard — apart from Herdr's keybinding config,
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
	// ColorScheme is "native", "herdr", or a Herdr built-in theme name.
	ColorScheme string
	// FontFamily is the terminal font family; "" is the system's
	// monospaced font.
	FontFamily string
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
	// OptionAsAlt makes Option the Meta (Alt) key of programs instead of
	// the key that types accented letters and symbols.
	OptionAsAlt bool
	// Cursor is the terminal cursor's shape, "block", "bar" or
	// "underline", until a program asks for another; CursorBlink whether
	// it blinks.
	Cursor      string
	CursorBlink bool
	// Toast is how agent notifications reach the user.
	Toast Toast
}

// Toast mirrors Herdr's [ui.toast] settings. Until the user picks a value
// here, it follows Herdr's own configuration (see InheritToast).
type Toast struct {
	// Delivery is "off", "herdr" (an in-app card) or "system" (the
	// Notification Center), as Herdr names them.
	Delivery string
	// Position is the in-app card's corner: "top-left", "top-right",
	// "bottom-left" or "bottom-right".
	Position string
	// DelaySeconds holds agent notifications back, 0–3600, and drops
	// those whose agent moved on meanwhile.
	DelaySeconds int
}

// Defaults returns the settings a fresh installation starts with.
func Defaults() Settings {
	return Settings{WindowTheme: "system", ColorScheme: "herdr", FontSize: 13, FontWeight: 400, LineHeight: 1, MinimumContrast: 4.5, Cursor: "block", CursorBlink: true, Toast: Toast{Delivery: "herdr", Position: "bottom-right", DelaySeconds: 1}}
}

// Store loads and saves the settings file. Load applies defaults for
// missing or invalid values rather than failing the launch.
type Store struct {
	mu     sync.Mutex
	path   string
	values Settings
	extra  map[string]json.RawMessage
	// toastSet names the toast keys chosen here, which Herdr's
	// configuration no longer overrides.
	toastSet map[string]bool
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
	s := &Store{path: resolved, values: Defaults(), toastSet: map[string]bool{}}
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
	if v, ok := raw["color_scheme"]; ok {
		var scheme string
		if json.Unmarshal(v, &scheme) == nil && scheme != "" {
			s.values.ColorScheme = scheme
		}
	}
	if v, ok := raw["font_family"]; ok {
		var family string
		if json.Unmarshal(v, &family) == nil {
			s.values.FontFamily = family
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
	if v, ok := raw["option_as_alt"]; ok {
		var on bool
		if json.Unmarshal(v, &on) == nil {
			s.values.OptionAsAlt = on
		}
	}
	if v, ok := raw["cursor_style"]; ok {
		var style string
		if json.Unmarshal(v, &style) == nil && ValidCursor(style) {
			s.values.Cursor = style
		}
	}
	if v, ok := raw["cursor_blink"]; ok {
		var blink bool
		if json.Unmarshal(v, &blink) == nil {
			s.values.CursorBlink = blink
		}
	}
	if v, ok := raw["toast_delivery"]; ok {
		var delivery string
		if json.Unmarshal(v, &delivery) == nil && validDelivery(delivery) {
			s.values.Toast.Delivery = delivery
			s.toastSet["delivery"] = true
		}
	} else if v, ok := raw["notifications"]; ok {
		// The earlier on/off switch: off still means off.
		var enabled bool
		if json.Unmarshal(v, &enabled) == nil && !enabled {
			s.values.Toast.Delivery = "off"
			s.toastSet["delivery"] = true
		}
	}
	if v, ok := raw["toast_position"]; ok {
		var position string
		if json.Unmarshal(v, &position) == nil && validPosition(position) {
			s.values.Toast.Position = position
			s.toastSet["position"] = true
		}
	}
	if v, ok := raw["toast_delay_seconds"]; ok {
		var delay int
		if json.Unmarshal(v, &delay) == nil && delay >= 0 && delay <= maxToastDelay {
			s.values.Toast.DelaySeconds = delay
			s.toastSet["delay"] = true
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
	before := s.values.Toast
	fn(&s.values)
	if s.toastSet == nil {
		s.toastSet = map[string]bool{}
	}
	after := s.values.Toast
	s.toastSet["delivery"] = s.toastSet["delivery"] || after.Delivery != before.Delivery
	s.toastSet["position"] = s.toastSet["position"] || after.Position != before.Position
	s.toastSet["delay"] = s.toastSet["delay"] || after.DelaySeconds != before.DelaySeconds
	if s.extra == nil {
		s.extra = map[string]json.RawMessage{}
	}
	document := s.extra
	document["window_theme"], _ = json.Marshal(s.values.WindowTheme)
	document["color_scheme"], _ = json.Marshal(s.values.ColorScheme)
	document["font_family"], _ = json.Marshal(s.values.FontFamily)
	document["font_size"], _ = json.Marshal(s.values.FontSize)
	document["font_weight"], _ = json.Marshal(s.values.FontWeight)
	document["line_height"], _ = json.Marshal(s.values.LineHeight)
	document["copy_on_select"], _ = json.Marshal(s.values.CopyOnSelect)
	document["minimum_contrast"], _ = json.Marshal(s.values.MinimumContrast)
	document["option_as_alt"], _ = json.Marshal(s.values.OptionAsAlt)
	document["cursor_style"], _ = json.Marshal(s.values.Cursor)
	document["cursor_blink"], _ = json.Marshal(s.values.CursorBlink)
	if s.toastSet["delivery"] {
		document["toast_delivery"], _ = json.Marshal(s.values.Toast.Delivery)
	}
	if s.toastSet["position"] {
		document["toast_position"], _ = json.Marshal(s.values.Toast.Position)
	}
	if s.toastSet["delay"] {
		document["toast_delay_seconds"], _ = json.Marshal(s.values.Toast.DelaySeconds)
	}
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

// InheritToast adopts Herdr's own [ui.toast] settings for every toast value
// not chosen here, so both clients agree until the user tells them apart.
// Fields of herdr left empty (DelaySeconds below zero) keep the defaults.
func (s *Store) InheritToast(herdr Toast) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.toastSet["delivery"] && herdr.Delivery != "" {
		s.values.Toast.Delivery = herdr.Delivery
	}
	if !s.toastSet["position"] && herdr.Position != "" {
		s.values.Toast.Position = herdr.Position
	}
	if !s.toastSet["delay"] && herdr.DelaySeconds >= 0 {
		s.values.Toast.DelaySeconds = herdr.DelaySeconds
	}
}

const maxToastDelay = 3600

// ValidCursor reports whether v names a cursor shape Settings.Cursor takes.
func ValidCursor(v string) bool { return v == "block" || v == "bar" || v == "underline" }

func validDelivery(v string) bool { return v == "off" || v == "herdr" || v == "system" }

func validPosition(v string) bool {
	return v == "top-left" || v == "top-right" || v == "bottom-left" || v == "bottom-right"
}

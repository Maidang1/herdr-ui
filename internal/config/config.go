// Package config reads and writes the settings of the window.
//
// The settings live in the files the Rust client uses: `config-gpui.toml`
// holds the managed defaults and `config-gpui.local.toml` the overrides
// the user writes, which merge key by key over the defaults.
//
// Reading uses a TOML parser; writing goes into the overrides file one key
// at a time, so that the comments and the order of that file stay as they
// are.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Settings are the settings of the window that the window honors.
type Settings struct {
	// TerminalSize is the size of the characters of the terminal.
	TerminalSize float64
	// TerminalLineHeight is the height of a row, as a multiple of the
	// size of the characters.
	TerminalLineHeight float64
	// UISize is the size of the characters of the rest of the window.
	UISize float64
	// KeepSelectionAfterCopy keeps the selection highlighted after it
	// was copied.
	KeepSelectionAfterCopy bool
	// Theme is the name of the theme.
	Theme string
	// Layout is the name of the sidebar layout.
	Layout string
	// NotificationsEnabled says the window shows the notices the daemon
	// gives.
	NotificationsEnabled bool
	// NotificationsFinished says the window shows the notices that say a
	// task is done.
	NotificationsFinished bool
	// Keybindings are the shortcuts the user set, by the name of the
	// action they belong to.
	Keybindings map[string]string
	// SoundVolume is how loud the sounds of the agents are, as a
	// percentage of the loudness of the sound itself.
	SoundVolume int
	// BellAttention bounces the icon of the window when a pane rings.
	BellAttention bool
	// BellSound plays the sound of the system when a pane rings.
	BellSound bool
}

// The defaults the window uses when the files say nothing.
const (
	DefaultTerminalSize       = 13.0
	DefaultTerminalLineHeight = 1.4
	DefaultUISize             = 12.0
	// DefaultKeepSelection keeps the selection highlighted after it was
	// copied, the way the Rust client does by default.
	DefaultKeepSelection = true
	// DefaultLayout is the sidebar layout the window uses when the files
	// say nothing.
	DefaultLayout = "normal"
	// DefaultNotificationsEnabled shows the notices of the daemon.
	//
	// The Rust client keeps them off and lets the daemon post them to the
	// system instead. This client has no such path, so the notices show
	// in the window unless the user turns them off.
	DefaultNotificationsEnabled = true
	// DefaultNotificationsFinished shows the notices that say a task is
	// done, the way the Rust client does by default.
	DefaultNotificationsFinished = true
	// DefaultSoundVolume is how loud the sounds of the agents are, the way
	// the Rust client has it by default.
	DefaultSoundVolume = 50
	// DefaultBellAttention bounces the icon of the window when a pane
	// rings, the way the Rust client does by default.
	DefaultBellAttention = true
	// DefaultBellSound plays no sound when a pane rings, the way the Rust
	// client does by default.
	DefaultBellSound = false
)

// Paths says where the settings live.
//
// The root is the same one the Rust client uses: the XDG config folder, or
// the folder of the user when there is none.
func Paths() (defaults string, local string, err error) {
	root, err := configRoot()
	if err != nil {
		return "", "", err
	}
	folder := filepath.Join(root, "herdr")
	return filepath.Join(folder, "config-gpui.toml"),
		filepath.Join(folder, "config-gpui.local.toml"),
		nil
}

// configRoot is the folder the settings live in.
func configRoot() (string, error) {
	if value := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); value != "" {
		if !filepath.IsAbs(value) {
			return "", fmt.Errorf("herdr-ui: XDG_CONFIG_HOME is not an absolute path")
		}
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config"), nil
}

// Load reads the settings of the window.
//
// The defaults come first and the overrides are laid over them, table by
// table, the way the Rust client loads them.
func Load() (Settings, error) {
	settings := Settings{
		TerminalSize:          DefaultTerminalSize,
		TerminalLineHeight:    DefaultTerminalLineHeight,
		UISize:                DefaultUISize,
		Layout:                DefaultLayout,
		NotificationsEnabled:  DefaultNotificationsEnabled,
		NotificationsFinished: DefaultNotificationsFinished,
		SoundVolume:           DefaultSoundVolume,
		BellAttention:         DefaultBellAttention,
		BellSound:             DefaultBellSound,
	}
	defaults, local, err := Paths()
	if err != nil {
		return settings, err
	}
	merged := map[string]any{}
	for _, path := range []string{defaults, local} {
		table, err := readFile(path)
		if err != nil {
			continue
		}
		merged = mergeTables(merged, table)
	}
	settings.TerminalSize = numberOf(merged, DefaultTerminalSize, "terminal", "size")
	settings.TerminalLineHeight = numberOf(merged, DefaultTerminalLineHeight, "terminal", "line_height")
	settings.UISize = numberOf(merged, DefaultUISize, "ui", "size")
	settings.KeepSelectionAfterCopy = boolOf(merged, DefaultKeepSelection, "keep_selection_after_copy")
	settings.Theme = stringOf(merged, "", "theme")
	settings.Layout = stringOf(merged, DefaultLayout, "layout")
	settings.NotificationsEnabled = boolOf(merged, DefaultNotificationsEnabled, "notifications", "enabled")
	settings.NotificationsFinished = boolOf(merged, DefaultNotificationsFinished, "notifications", "finished")
	settings.Keybindings = keybindingsOf(merged)
	settings.SoundVolume = intOf(merged, DefaultSoundVolume, "sound", "volume")
	settings.BellAttention = boolOf(merged, DefaultBellAttention, "bell", "attention")
	settings.BellSound = boolOf(merged, DefaultBellSound, "bell", "sound")
	return settings, nil
}

// keybindingsOf reads the shortcuts the user set.
//
// The table holds one key per action. A key may name one shortcut or a
// list of them, the way the Rust client allows.
func keybindingsOf(table map[string]any) map[string]string {
	values, ok := lookup(table, "keybindings")
	if !ok {
		return nil
	}
	entries, ok := values.(map[string]any)
	if !ok {
		return nil
	}
	bindings := map[string]string{}
	for name, value := range entries {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				bindings[name] = typed
			}
		case []any:
			var parts []string
			for _, item := range typed {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
			if len(parts) > 0 {
				bindings[name] = strings.Join(parts, ",")
			}
		}
	}
	if len(bindings) == 0 {
		return nil
	}
	return bindings
}

// readFile reads one TOML file into a table.
func readFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	table := map[string]any{}
	if err := toml.Unmarshal(data, &table); err != nil {
		return nil, err
	}
	return table, nil
}

// mergeTables lays one table over another, key by key.
func mergeTables(base, over map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range over {
		if table, ok := value.(map[string]any); ok {
			if existing, ok := merged[key].(map[string]any); ok {
				merged[key] = mergeTables(existing, table)
				continue
			}
		}
		merged[key] = value
	}
	return merged
}

// lookup finds a value in a table, by the parts of its key.
func lookup(table map[string]any, parts ...string) (any, bool) {
	var current any = table
	for _, part := range parts {
		asTable, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = asTable[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func numberOf(table map[string]any, fallback float64, parts ...string) float64 {
	value, ok := lookup(table, parts...)
	if !ok {
		return fallback
	}
	switch number := value.(type) {
	case float64:
		return number
	case int64:
		return float64(number)
	case int:
		return float64(number)
	}
	return fallback
}

// intOf reads a whole number out of a table.
func intOf(table map[string]any, fallback int, parts ...string) int {
	value, ok := lookup(table, parts...)
	if !ok {
		return fallback
	}
	switch number := value.(type) {
	case int64:
		return int(number)
	case int:
		return number
	case float64:
		return int(number)
	}
	return fallback
}

func boolOf(table map[string]any, fallback bool, parts ...string) bool {
	value, ok := lookup(table, parts...)
	if !ok {
		return fallback
	}
	if number, ok := value.(bool); ok {
		return number
	}
	return fallback
}

func stringOf(table map[string]any, fallback string, parts ...string) string {
	value, ok := lookup(table, parts...)
	if !ok {
		return fallback
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fallback
}

// SetLocal writes one setting into the overrides file.
//
// The key is dotted: `terminal.size` is the size of the characters of the
// terminal. The value is written as TOML.
func SetLocal(key string, value any) error {
	_, local, err := Paths()
	if err != nil {
		return err
	}
	text, err := formatValue(value)
	if err != nil {
		return err
	}
	return setKey(local, key, text)
}

// SetLocalKeybinding writes the shortcut of one action into the overrides
// file.
//
// An empty shortcut unbinds the action, and its key is taken out of the
// file, so that the default of the action stands again.
func SetLocalKeybinding(name, binding string) error {
	_, local, err := Paths()
	if err != nil {
		return err
	}
	if strings.TrimSpace(binding) == "" {
		return removeKey(local, "keybindings."+name)
	}
	return setKey(local, "keybindings."+name, strconv.Quote(binding))
}

// formatValue writes a value the way TOML wants it.
func formatValue(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int:
		return strconv.Itoa(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	}
	out, err := toml.Marshal(map[string]any{"value": value})
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(out))
	text = strings.TrimPrefix(text, "value = ")
	return strings.TrimSuffix(text, "\n"), nil
}

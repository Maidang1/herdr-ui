// Package input resolves Herdr key bindings without initializing a terminal.
package input

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

// Defaults transcribed from Herdr 7c34cf2 src/config/model.rs KeysConfig.
//
//go:embed defaults.json
var defaultsJSON []byte

// Command preserves the execution payload for a custom key binding.
type Command struct {
	Command     string `json:"command"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Width       any    `json:"width,omitempty"`
	Height      any    `json:"height,omitempty"`
}

type Bindings struct {
	Diagnostics []string          `json:"diagnostics,omitempty"`
	Indices     map[string]int    `json:"indices"` // Zero-based target or command index.
	Navigation  map[string]string `json:"navigation"`
	Commands    []Command         `json:"commands,omitempty"`
	Prefix      string            `json:"prefix"`
	Source      string            `json:"source"`
	Actions     map[string]string `json:"actions"`
}

// Resolve disables individual invalid/conflicting bindings with diagnostics.
// Invalid TOML remains fatal; unrelated valid bindings stay available.
func Resolve(data []byte, source string) (Bindings, error) {
	b := Bindings{Prefix: "ctrl+b", Source: source, Actions: map[string]string{}, Indices: map[string]int{}, Navigation: map[string]string{}}
	var config struct {
		Keys map[string]any `toml:"keys"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return b, fmt.Errorf("bindings %s: %w", source, err)
	}
	var defaults []struct{ Action, Key string }
	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return b, err
	}
	known := map[string]bool{"prefix": true, "command": true, "indexed": true, "fullscreen": true}
	for _, d := range defaults {
		known[d.Action] = true
	}
	names := make([]string, 0, len(config.Keys))
	for k := range config.Keys {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		if !known[k] {
			b.diagnostic("unknown keys.%s; ignoring field", k)
		}
	}
	if v, ok := config.Keys["fullscreen"]; ok {
		if _, exists := config.Keys["zoom"]; exists {
			b.diagnostic("keys.fullscreen ignored because keys.zoom is also set")
		} else {
			config.Keys["zoom"] = v
		}
	}
	if v, ok := config.Keys["prefix"]; ok {
		raw, isString := v.(string)
		n, err := Normalize(raw)
		if !isString || err != nil || strings.HasPrefix(n, "prefix+") {
			b.diagnostic("invalid keys.prefix %q; using ctrl+b", raw)
		} else if err = nativeBindingError(n); err != nil {
			b.diagnostic("keys.prefix: %v; using ctrl+b", err)
		} else {
			b.Prefix = n
		}
	}

	// Legacy indexed bindings are applied after all explicit builtin actions,
	// before custom commands. A nonempty legacy entry suppresses modern defaults,
	// even when the legacy modifier itself is invalid.
	type legacyBinding struct {
		action, field string
		value         any
	}
	var legacy []legacyBinding
	legacySet := map[string]bool{}
	if v, ok := config.Keys["indexed"]; ok {
		rows, ok := v.(map[string]any)
		if !ok {
			b.diagnostic("keys.indexed must be a table; ignoring field")
		} else {
			for _, entry := range []struct{ name, action string }{{"tabs", "switch_tab"}, {"workspaces", "switch_workspace"}, {"agents", "focus_agent"}} {
				raw, exists := rows[entry.name]
				if !exists {
					continue
				}
				field := "keys.indexed." + entry.name
				mods, ok := raw.(string)
				if !ok {
					b.diagnostic("%s must be a string; disabling binding", field)
					legacySet[entry.action] = true
					continue
				}
				if strings.TrimSpace(mods) == "" {
					continue
				}
				legacySet[entry.action] = true
				valid := true
				for _, mod := range strings.Split(mods, "+") {
					mod = strings.ToLower(strings.TrimSpace(mod))
					if alias, ok := aliases[mod]; ok {
						mod = alias
					}
					if mod != "ctrl" && mod != "alt" && mod != "shift" && mod != "cmd" {
						b.diagnostic("%s: unsupported native modifier %q; disabling binding", field, mod)
						valid = false
						break
					}
				}
				if valid {
					legacy = append(legacy, legacyBinding{entry.action, field, mods + "+1..9"})
				}
			}
			var unknown []string
			for name := range rows {
				if name != "tabs" && name != "workspaces" && name != "agents" {
					unknown = append(unknown, name)
				}
			}
			sort.Strings(unknown)
			for _, name := range unknown {
				b.diagnostic("unknown keys.indexed.%s; ignoring field", name)
			}

		}
	}
	var commandBindings []legacyBinding
	if v, ok := config.Keys["command"]; ok {
		rows, ok := v.([]any)
		if !ok {
			b.diagnostic("keys.command must be an array of tables; disabling custom commands")
		} else {
			// Keep source indices stable, including disabled command rows.
			b.Commands = make([]Command, len(rows))
			for i, raw := range rows {
				field := fmt.Sprintf("keys.command[%d]", i)
				row, ok := raw.(map[string]any)
				if !ok {
					b.diagnostic("%s must be a table; disabling command", field)
					continue
				}
				command, ok := row["command"].(string)
				if !ok || strings.TrimSpace(command) == "" {
					b.diagnostic("%s has empty command; disabling command", field)
					continue
				}
				kind := "shell"
				if v, exists := row["type"]; exists {
					kind, ok = v.(string)
					if !ok || (kind != "shell" && kind != "pane" && kind != "popup" && kind != "plugin_action") {
						b.diagnostic("%s has unsupported type; disabling command", field)
						continue
					}
				}
				desc, _ := row["description"].(string)
				payload := Command{Command: command, Type: kind, Description: desc}
				invalid := false
				for _, dimension := range []string{"width", "height"} {
					if value, exists := row[dimension]; exists {
						if err := validatePopupSize(value); err != nil {
							b.diagnostic("%s.%s: %v; disabling command", field, dimension, err)
							invalid = true
						}
					}
				}
				if invalid {
					continue
				}
				if kind == "popup" {
					payload.Width, payload.Height = row["width"], row["height"]
				} else if row["width"] != nil || row["height"] != nil {
					b.diagnostic("%s: ignoring popup size on non-popup command", field)
				}
				b.Commands[i] = payload
				key := row["key"]
				if key == nil {
					key = ""
				}
				commandBindings = append(commandBindings, legacyBinding{fmt.Sprintf("command[%d]", i), field + ".key", key})
			}
		}
	}
	for _, configured := range []bool{true, false} {
		for _, d := range defaults {
			value, user := config.Keys[d.Action]
			if user != configured {
				continue
			}
			if !user {
				if legacySet[d.Action] {
					continue
				}
				value = d.Key
			}
			b.applyBindings(d.Action, "keys."+d.Action, value, user)
		}
		if configured {
			for _, entry := range legacy {
				b.applyBindings(entry.action, entry.field, entry.value, true)
			}
			for _, entry := range commandBindings {
				b.applyBindings(entry.action, entry.field, entry.value, true)
			}
		}
	}
	return b, nil
}
func (b *Bindings) diagnostic(format string, args ...any) {
	b.Diagnostics = append(b.Diagnostics, fmt.Sprintf(format, args...))
}
func (b *Bindings) applyBindings(action, field string, value any, configured bool) {
	var labels []string
	switch value := value.(type) {
	case string:
		labels = []string{value}
	case []any:
		if action == "remote_image_paste" {
			b.diagnostic("%s must be a string; disabling binding", field)
			return
		}
		for _, raw := range value {
			if label, ok := raw.(string); ok {
				labels = append(labels, label)
			} else {
				b.diagnostic("%s must contain strings; disabling invalid entry", field)
			}
		}
	default:
		b.diagnostic("%s must be string or string array; disabling binding", field)
		return
	}
	indexed := action == "switch_tab" || action == "switch_workspace" || action == "focus_agent"
	for _, label := range labels {
		if strings.TrimSpace(label) == "" {
			continue
		}
		expanded := []string{label}
		if strings.Contains(label, "1..9") {
			if !indexed || strings.Count(label, "1..9") != 1 {
				b.diagnostic("%s: range is only valid for indexed actions; disabling %q", field, label)
				continue
			}
			expanded = nil
			for i := 1; i <= 9; i++ {
				expanded = append(expanded, strings.Replace(label, "1..9", strconv.Itoa(i), 1))
			}
		}
		for _, raw := range expanded {
			b.applyBinding(action, field, raw, indexed, configured)
		}
	}
}
func (b *Bindings) applyBinding(action, field, raw string, indexed, configured bool) {
	n, err := Normalize(raw)
	if err != nil {
		b.diagnostic("%s: %v; disabling binding", field, err)
		return
	}
	if err := nativeBindingError(n); err != nil {
		b.diagnostic("%s: %v; disabling binding", field, err)
		return
	}
	index := 0
	if indexed {
		digit := ""
		for _, part := range strings.Split(raw, "+") {
			part = strings.TrimSpace(part)
			if len(part) == 1 && part[0] >= '1' && part[0] <= '9' {
				digit = part
			}
		}
		if digit == "" {
			b.diagnostic("%s must use digits 1..9; disabling %q", field, raw)
			return
		}
		index = int(digit[0] - '1')
	}
	navigation := strings.HasPrefix(action, "navigate_")
	prefix := strings.HasPrefix(n, "prefix+")
	mods, code := splitKey(strings.TrimPrefix(n, "prefix+"))
	if action == "remote_image_paste" && prefix {
		b.diagnostic("%s must be a direct key; disabling %q", field, raw)
		return
	}
	if n == b.Prefix || n == "prefix+"+b.Prefix {
		if configured {
			b.diagnostic("%s: kept keys.prefix, disabled %q", field, raw)
		}
		return
	}
	if prefix && code == "esc" {
		b.diagnostic("%s: Escape cancels prefix mode; disabling %q", field, raw)
		return
	}
	if navigation {
		if prefix || code == "esc" || n == "enter" || n == "tab" || n == "shift+tab" || n == "left" || n == "right" || (len(n) == 1 && n[0] >= '1' && n[0] <= '9') {
			b.diagnostic("%s uses reserved navigation key %s; disabling binding", field, n)
			return
		}
	} else if action != "remote_image_paste" && !prefix && !mods["ctrl"] && !mods["alt"] && !mods["cmd"] && (utf8.RuneCountInString(code) == 1 || code == "space") {
		b.diagnostic("%s: unsafe direct key %s intercepts typing; disabling binding", field, n)
		return
	}
	target := b.Actions
	if navigation {
		target = b.Navigation
	}
	if previous, exists := target[n]; exists {
		if configured {
			b.diagnostic("%s: kept %s, disabled %s", n, previous, field)
		}
		return
	}
	target[n] = action
	if !navigation {
		if strings.HasPrefix(action, "command[") {
			index, _ = strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(action, "command["), "]"))
		}
		b.Indices[n] = index
	}
}

// nativeBindingError prevents advertising chords the pinned native key API
// cannot produce. In particular Hyper is distinct from Ctrl+Alt+Shift+Super.
func nativeBindingError(key string) error {
	_, code := splitKey(strings.TrimPrefix(key, "prefix+"))
	if strings.HasPrefix(code, "f") {
		if n, err := strconv.Atoi(code[1:]); err == nil && n > 12 {
			return fmt.Errorf("%s is unavailable in the native key event API (F1–F12 only)", code)
		}
	}
	if utf8.RuneCountInString(code) == 1 {
		r := []rune(code)[0]
		if r < ' ' || r > '~' {
			return fmt.Errorf("%q is unavailable as a native shortcut key", code)
		}
	}
	return nil
}

// splitKey accepts normalized literal-plus keys, including cmd++.
func splitKey(key string) (map[string]bool, string) {
	if key == "" {
		return map[string]bool{}, ""
	}
	parts := strings.Split(key, "+")
	code := parts[len(parts)-1]
	if code == "" {
		code = "+"
		parts = parts[:len(parts)-1]
	}
	mods := map[string]bool{}
	for _, part := range parts[:len(parts)-1] {
		mods[part] = true
	}
	return mods, code
}

func validatePopupSize(value any) error {
	switch v := value.(type) {
	case int64:
		if v >= 0 && v <= 65535 {
			return nil
		}
	case string:
		if strings.HasSuffix(v, "%") {
			n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
			if err == nil && n > 0 && n <= 100 {
				return nil
			}
		}
	}
	return fmt.Errorf("expected cell count from 0 to 65535 or percentage from 1%% to 100%%")
}

var aliases = map[string]string{"pgup": "pageup", "pgdn": "pagedown", "del": "delete", "ins": "insert", "backtab": "shift+tab", "control": "ctrl", "option": "alt", "meta": "alt", "command": "cmd", "super": "cmd", "escape": "esc", "return": "enter", "bs": "backspace", "minus": "-", "comma": ",", "period": ".", "slash": "/", "backslash": "\\", "quote": "'", "double_quote": "\"", "double-quote": "\"", "semicolon": ";", "colon": ":", "percent": "%", "ampersand": "&", "backtick": "`", "plus": "+"}

func Normalize(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	tokens := strings.Split(raw, "+")
	for i, token := range tokens {
		if strings.EqualFold(strings.TrimSpace(token), "backtab") {
			tokens[i] = "shift+tab"
		}
	}
	raw = strings.Join(tokens, "+")
	if raw == "+" {
		raw = "plus"
	} else if strings.HasSuffix(raw, "++") {
		raw = strings.TrimSuffix(raw, "+") + "plus"
	}
	parts := strings.Split(raw, "+")
	mods := map[string]bool{}
	key := ""
	pref := false
	for _, part := range parts {
		p := strings.TrimSpace(part)
		low := strings.ToLower(p)
		if a, ok := aliases[low]; ok {
			low = a
		}
		switch low {
		case "prefix":
			if pref {
				return "", fmt.Errorf("duplicate prefix in %q", raw)
			}
			pref = true
		case "hyper":
			return "", fmt.Errorf("hyper is an independent modifier unavailable in native key events")
		case "ctrl", "alt", "shift", "cmd":
			mods[low] = true
		default:
			if key != "" || low == "" {
				return "", fmt.Errorf("invalid binding %q", raw)
			}
			if len(p) == 1 && p[0] >= 'A' && p[0] <= 'Z' {
				mods["shift"] = true
			}
			key = low
			if utf8.RuneCountInString(p) == 1 && len(p) > 1 {
				key = p
			}
		}
	}
	if key == "" {
		return "", fmt.Errorf("binding lacks a key: %q", raw)
	}
	valid := utf8.RuneCountInString(key) == 1
	switch key {
	case "enter", "esc", "tab", "backspace", "left", "right", "up", "down", "space", "home", "end", "delete", "insert", "pageup", "pagedown":
		valid = true
	}
	if strings.HasPrefix(key, "f") {
		if n, e := strconv.Atoi(key[1:]); e == nil && n >= 1 && n <= 255 {
			valid = true
		}
	}
	if !valid {
		return "", fmt.Errorf("unsupported key %q", key)
	}
	// Crossterm normalizes shifted printable ASCII to its resulting symbol.
	if mods["shift"] && len(key) == 1 {
		const plain = "`1234567890-=[]\\;',./"
		const shifted = "~!@#$%^&*()_+{}|:\"<>?"
		if i := strings.Index(plain, key); i >= 0 {
			key = string(shifted[i])
			delete(mods, "shift")
		} else if strings.Contains(shifted, key) {
			delete(mods, "shift")
		}
	}
	out := []string{}
	if pref {
		out = append(out, "prefix")
	}
	for _, m := range []string{"ctrl", "alt", "shift", "cmd"} {
		if mods[m] {
			out = append(out, m)
		}
	}
	return strings.Join(append(out, key), "+"), nil
}

func (b Bindings) Labels() []string {
	out := make([]string, 0, len(b.Actions))
	for k, v := range b.Actions {
		out = append(out, k+" → "+v)
	}
	sort.Strings(out)
	return out
}

// LoadBindings resolves the user config location. Missing configuration uses
// verified defaults; malformed configuration is an error.
func LoadBindings(path string) (Bindings, error) {
	if path == "" {
		path = os.Getenv("HERDR_CONFIG_PATH")
	}
	if path == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return Bindings{}, err
			}
			base = filepath.Join(home, ".config")
		}
		path = filepath.Join(base, "herdr", "config.toml")
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Resolve(nil, "defaults (missing "+path+")")
	}
	if err != nil {
		return Bindings{}, fmt.Errorf("bindings %s: %w", path, err)
	}
	return Resolve(data, path)
}

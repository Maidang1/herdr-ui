// Package input resolves the M0 key contract without initializing a terminal.
package input

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"
)

// Defaults transcribed from Herdr 7c34cf2 src/config/model.rs KeysConfig.
// Navigation/indexed overlays need a separate mode-aware parity pass.
//
//go:embed defaults.json
var defaultsJSON []byte

type Bindings struct {
	Prefix  string            `json:"prefix"`
	Source  string            `json:"source"`
	Actions map[string]string `json:"actions"`
}

// Resolve deliberately rejects unimplemented config forms instead of claiming
// they use defaults. This is the M0 subset, not server-memory configuration.
func Resolve(data []byte, source string) (Bindings, error) {
	b := Bindings{Prefix: "ctrl+b", Source: source, Actions: map[string]string{}}
	var config struct {
		Keys map[string]any `toml:"keys"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return b, fmt.Errorf("bindings %s: %w", source, err)
	}
	var defaults []struct {
		Action string
		Key    string
	}
	if err := json.Unmarshal(defaultsJSON, &defaults); err != nil {
		return b, err
	}
	known := map[string]bool{"prefix": true, "command": true}
	for _, d := range defaults {
		known[d.Action] = true
	}
	for k := range config.Keys {
		if !known[k] {
			return b, fmt.Errorf("bindings: unsupported keys.%s in M0 (no default fallback)", k)
		}
	}
	if commands, ok := config.Keys["command"]; ok {
		rows, ok := commands.([]any)
		if !ok {
			return b, fmt.Errorf("keys.command must be an array of tables")
		}
		for i, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				return b, fmt.Errorf("keys.command[%d] must be a table", i)
			}
			text, ok := row["command"].(string)
			if !ok || strings.TrimSpace(text) == "" {
				return b, fmt.Errorf("keys.command[%d] has empty command", i)
			}
			if kind, ok := row["type"]; ok && kind != "shell" && kind != "pane" && kind != "popup" && kind != "plugin_action" {
				return b, fmt.Errorf("keys.command[%d] has unsupported type", i)
			}
			action := fmt.Sprintf("command[%d]", i)
			config.Keys[action] = row["key"]
			defaults = append(defaults, struct {
				Action string
				Key    string
			}{Action: action})
		}
	}
	if v, ok := config.Keys["prefix"]; ok {
		s, ok := v.(string)
		if !ok {
			return b, fmt.Errorf("keys.prefix must be a string")
		}
		n, err := Normalize(s)
		if err != nil || strings.HasPrefix(n, "prefix+") {
			return b, fmt.Errorf("invalid keys.prefix %q", s)
		}
		// M0 can safely transmit a literal control prefix through direct attach.
		if !strings.HasPrefix(n, "ctrl+") || len(n) != 6 || n[5] < 'a' || n[5] > 'z' {
			return b, fmt.Errorf("M0 requires a ctrl+letter prefix, got %q", s)
		}
		b.Prefix = n
	}
	// Herdr gives user bindings precedence over defaults. Conflicts among user
	// bindings are errors here so the probe cannot silently misrepresent them.
	for _, user := range []bool{true, false} {
		for _, d := range defaults {
			v, configured := config.Keys[d.Action]
			if configured != user {
				continue
			}
			labels := []string{d.Key}
			if configured {
				switch v := v.(type) {
				case string:
					labels = []string{v}
				case []any:
					labels = nil
					for _, x := range v {
						s, ok := x.(string)
						if !ok {
							return b, fmt.Errorf("keys.%s must contain strings", d.Action)
						}
						labels = append(labels, s)
					}
				default:
					return b, fmt.Errorf("keys.%s must be string or string array", d.Action)
				}
			}
			for _, label := range labels {
				if strings.TrimSpace(label) == "" {
					continue
				}
				expanded := []string{label}
				if strings.Contains(label, "1..9") {
					if d.Action != "switch_tab" && d.Action != "switch_workspace" && d.Action != "focus_agent" {
						return b, fmt.Errorf("range not valid for %s", d.Action)
					}
					expanded = nil
					for i := 1; i <= 9; i++ {
						expanded = append(expanded, strings.ReplaceAll(label, "1..9", strconv.Itoa(i)))
					}
				}
				for _, s := range expanded {
					n, err := Normalize(s)
					if err != nil {
						return b, fmt.Errorf("keys.%s: %w", d.Action, err)
					}
					if !strings.HasPrefix(n, "prefix+") && !strings.Contains(n, "ctrl+") && !strings.Contains(n, "alt+") && !strings.Contains(n, "cmd+") {
						key := strings.TrimPrefix(n, "shift+")
						if utf8.RuneCountInString(key) == 1 || key == "space" {
							return b, fmt.Errorf("keys.%s: unsafe direct binding %s intercepts typing", d.Action, n)
						}
					}
					if n == b.Prefix || n == "prefix+"+b.Prefix {
						if !configured {
							continue
						}
						return b, fmt.Errorf("keys.%s conflicts with literal prefix", d.Action)
					}
					if previous, ok := b.Actions[n]; ok {
						if !configured {
							continue
						}
						return b, fmt.Errorf("binding %s conflicts: %s and %s", n, previous, d.Action)
					}
					b.Actions[n] = d.Action
				}
			}
		}
	}
	return b, nil
}

var aliases = map[string]string{"control": "ctrl", "option": "alt", "meta": "alt", "command": "cmd", "super": "cmd", "escape": "esc", "return": "enter", "bs": "backspace", "minus": "-", "comma": ",", "period": ".", "slash": "/", "backslash": "\\", "quote": "'", "double_quote": "\"", "double-quote": "\"", "semicolon": ";", "colon": ":", "percent": "%", "ampersand": "&", "backtick": "`", "plus": "+"}

func Normalize(raw string) (string, error) {
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
		}
	}
	if key == "" {
		return "", fmt.Errorf("binding lacks a key: %q", raw)
	}
	valid := utf8.RuneCountInString(key) == 1
	switch key {
	case "enter", "esc", "tab", "backspace", "left", "right", "up", "down", "space":
		valid = true
	}
	if strings.HasPrefix(key, "f") {
		if n, e := strconv.Atoi(key[1:]); e == nil && n >= 1 && n <= 12 {
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

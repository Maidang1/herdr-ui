// Copyright Herdr contributors. SPDX-License-Identifier: Apache-2.0
// Palette data and resolution adapted from herdrdev/herdr; see herdr-ATTRIBUTION.md.
package design

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/pelletier/go-toml/v2"
)

// HerdrTheme keeps the native widget theme and all Herdr surface/status tokens
// together. Theme.Dark describes the resolved background, not the system slot.
type HerdrTheme struct {
	Theme  *ui.Theme
	Colors Palette
}

//go:embed herdr_palettes.json
var herdrPaletteJSON []byte

var herdrPalettes = func() map[string]map[string]string {
	var palettes map[string]map[string]string
	if err := json.Unmarshal(herdrPaletteJSON, &palettes); err != nil {
		panic(err)
	}
	return palettes
}()

// HerdrThemeNames returns a fresh list of the upstream canonical built-ins.
func HerdrThemeNames() []string {
	return []string{"catppuccin", "catppuccin-latte", "terminal", "tokyo-night", "tokyo-night-day", "dracula", "nord", "gruvbox", "gruvbox-light", "one-dark", "one-light", "solarized", "solarized-light", "kanagawa", "kanagawa-lotus", "rose-pine", "rose-pine-dawn", "vesper"}
}

func canonicalHerdrName(name string) string {
	name = strings.NewReplacer(" ", "-", "_", "-").Replace(strings.ToLower(name))
	switch name {
	case "catppuccin", "catppuccin-mocha":
		return "catppuccin"
	case "catppuccin-latte", "latte", "light":
		return "catppuccin-latte"
	case "terminal":
		return "terminal"
	case "tokyo-night", "tokyonight":
		return "tokyo-night"
	case "tokyo-night-day", "tokyo-day", "tokyonight-day":
		return "tokyo-night-day"
	case "dracula":
		return "dracula"
	case "nord":
		return "nord"
	case "gruvbox", "gruvbox-dark":
		return "gruvbox"
	case "gruvbox-light":
		return "gruvbox-light"
	case "one-dark", "onedark":
		return "one-dark"
	case "one-light", "onelight":
		return "one-light"
	case "solarized", "solarized-dark":
		return "solarized"
	case "solarized-light":
		return "solarized-light"
	case "kanagawa":
		return "kanagawa"
	case "kanagawa-lotus", "lotus":
		return "kanagawa-lotus"
	case "rose-pine", "rosepine":
		return "rose-pine"
	case "rose-pine-dawn", "rosepine-dawn", "dawn":
		return "rose-pine-dawn"
	case "vesper":
		return "vesper"
	}
	return ""
}

func siblingHerdrNames(name string) (string, string) {
	switch canonicalHerdrName(name) {
	case "catppuccin", "catppuccin-latte":
		return "catppuccin", "catppuccin-latte"
	case "tokyo-night", "tokyo-night-day":
		return "tokyo-night", "tokyo-night-day"
	case "gruvbox", "gruvbox-light":
		return "gruvbox", "gruvbox-light"
	case "one-dark", "one-light":
		return "one-dark", "one-light"
	case "solarized", "solarized-light":
		return "solarized", "solarized-light"
	case "kanagawa", "kanagawa-lotus":
		return "kanagawa", "kanagawa-lotus"
	case "rose-pine", "rose-pine-dawn":
		return "rose-pine", "rose-pine-dawn"
	}
	return name, name
}

type herdrConfig struct {
	Theme struct {
		Name       *string        `toml:"name"`
		AutoSwitch bool           `toml:"auto_switch"`
		DarkName   *string        `toml:"dark_name"`
		LightName  *string        `toml:"light_name"`
		Custom     map[string]any `toml:"custom"`
	} `toml:"theme"`
	UI struct {
		Accent *string `toml:"accent"`
	} `toml:"ui"`
}

// HerdrThemes resolves the light and dark system-appearance slots. "herdr"
// reads config.toml at path; missing files use upstream defaults. A built-in
// name selects that palette for both slots and ignores configuration. "terminal"
// uses native defaults because there is no host terminal palette to inherit.
func HerdrThemes(path, name string) (light, dark *HerdrTheme, err error) {
	if name != "herdr" {
		canonical := canonicalHerdrName(name)
		if canonical == "" {
			return nil, nil, fmt.Errorf("unknown Herdr theme %q", name)
		}
		return resolveHerdr(canonical, false, nil, nil, nil), resolveHerdr(canonical, true, nil, nil, nil), nil
	}
	var config herdrConfig
	if path != "" {
		data, readErr := os.ReadFile(path)
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, nil, fmt.Errorf("read Herdr theme: %w", readErr)
		}
		if readErr == nil {
			if err := toml.Unmarshal(data, &config); err != nil {
				return nil, nil, fmt.Errorf("parse Herdr theme: %w", err)
			}
		}
	}
	custom, err := herdrOverrides(config.Theme.Custom)
	if err != nil {
		return nil, nil, err
	}
	var legacy *string
	if config.UI.Accent != nil && *config.UI.Accent != "cyan" && custom["accent"] == nil {
		legacy = config.UI.Accent
	}
	manual := "catppuccin"
	if config.Theme.Name != nil {
		manual = *config.Theme.Name
	}
	if !config.Theme.AutoSwitch {
		base := canonicalHerdrName(manual)
		if base == "" {
			base = "catppuccin"
		}
		return resolveHerdr(base, false, custom, nil, legacy), resolveHerdr(base, true, custom, nil, legacy), nil
	}
	darkName, lightName := siblingHerdrNames(manual)
	if config.Theme.DarkName != nil {
		darkName = *config.Theme.DarkName
	}
	if config.Theme.LightName != nil {
		lightName = *config.Theme.LightName
	}
	darkName, lightName = canonicalHerdrName(darkName), canonicalHerdrName(lightName)
	if darkName == "" {
		darkName = "catppuccin"
	}
	if lightName == "" {
		lightName = "catppuccin-latte"
	}
	lightCustom, err := herdrModeOverrides(config.Theme.Custom, "light")
	if err != nil {
		return nil, nil, err
	}
	darkCustom, err := herdrModeOverrides(config.Theme.Custom, "dark")
	if err != nil {
		return nil, nil, err
	}
	return resolveHerdr(lightName, false, custom, lightCustom, legacy), resolveHerdr(darkName, true, custom, darkCustom, legacy), nil
}

func herdrOverrides(values map[string]any) (map[string]*string, error) {
	overrides := make(map[string]*string)
	for key := range herdrPalettes["catppuccin"] {
		value, ok := values[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("Herdr theme custom.%s must be a color string", key)
		}
		overrides[key] = &text
	}
	return overrides, nil
}

func herdrModeOverrides(values map[string]any, mode string) (map[string]*string, error) {
	value, exists := values[mode]
	if !exists {
		return nil, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Herdr theme custom.%s must be a table", mode)
	}
	return herdrOverrides(table)
}

func nativeHerdrTokens(dark bool) map[string]ui.Color {
	p := Colors(dark)
	return map[string]ui.Color{
		"accent": p.Accent, "panel_bg": p.Shell, "sidebar_bg": p.Sidebar,
		"active_row_bg": p.RowSelected, "selection_bg": p.Selected,
		"surface0": p.Elevated, "surface1": p.Hover, "surface_dim": p.Content,
		"overlay0": p.Tertiary, "overlay1": p.Secondary, "text": p.Text,
		"subtext0": p.Secondary, "mauve": p.Working, "green": p.Success,
		"yellow": p.Working, "red": p.Danger, "blue": p.Accent,
		"teal": p.Success, "peach": p.Warning,
	}
}

func resolveHerdr(name string, slotDark bool, custom, mode map[string]*string, legacy *string) *HerdrTheme {
	if name == "terminal" && len(custom) == 0 && len(mode) == 0 && legacy == nil {
		return &HerdrTheme{Theme: NativeTheme(slotDark, false), Colors: Colors(slotDark)}
	}
	native := nativeHerdrTokens(slotDark)
	tokens := make(map[string]ui.Color, len(native))
	for key, value := range native {
		tokens[key] = value
	}
	apply := func(key, value string) { tokens[key] = parseHerdrColor(value, native[key]) }
	if name != "terminal" {
		for key, value := range herdrPalettes[name] {
			// Upstream Reset preserves the underlying background.
			fallback := native[key]
			if key == "sidebar_bg" {
				fallback = tokens["panel_bg"]
			}
			tokens[key] = parseHerdrColor(value, fallback)
		}
		// Map iteration order must not affect the reset sidebar background.
		if herdrPalettes[name]["sidebar_bg"] == "reset" {
			tokens["sidebar_bg"] = tokens["panel_bg"]
		}
	}
	for key, value := range custom {
		apply(key, *value)
	}
	if legacy != nil {
		apply("accent", *legacy)
	}
	for key, value := range mode {
		apply(key, *value)
	}
	if name != "terminal" {
		// A reset sidebar follows a custom panel background too.
		sidebar := herdrPalettes[name]["sidebar_bg"]
		if v := custom["sidebar_bg"]; v != nil {
			sidebar = *v
		}
		if v := mode["sidebar_bg"]; v != nil {
			sidebar = *v
		}
		if isHerdrReset(sidebar) {
			tokens["sidebar_bg"] = tokens["panel_bg"]
		}
	}
	dark := herdrLuminance(tokens["panel_bg"]) < .5
	t := NativeTheme(dark, false)
	p := Colors(dark)
	p.Shell, p.Top, p.Middle, p.Bottom = tokens["panel_bg"], tokens["panel_bg"], tokens["panel_bg"], tokens["panel_bg"]
	p.Sidebar, p.SidebarTint = tokens["sidebar_bg"], tokens["sidebar_bg"]
	p.Content, p.Grouped = tokens["surface_dim"], tokens["surface_dim"]
	p.Inset, p.Elevated = tokens["surface0"], tokens["surface0"]
	p.Hover, p.Pressed = tokens["surface1"], tokens["surface0"]
	p.Text, p.Secondary, p.Tertiary = tokens["text"], tokens["subtext0"], tokens["overlay0"]
	p.Border, p.ControlBorder, p.Disabled = tokens["surface1"], tokens["overlay0"], tokens["overlay0"]
	p.Selected, p.RowSelected, p.InactiveSelected = tokens["selection_bg"], tokens["active_row_bg"], tokens["active_row_bg"]
	p.Accent, p.Focus = tokens["accent"], tokens["accent"]
	p.Working, p.Idle, p.Warning = tokens["yellow"], tokens["green"], tokens["peach"]
	p.Danger, p.Success = tokens["red"], tokens["green"]
	t.Background, t.Surface = p.Shell, p.Elevated
	t.SurfaceHover, t.SurfacePressed, t.Border = p.Hover, p.Pressed, p.ControlBorder
	t.Text, t.TextMuted = p.Text, p.Secondary
	t.Accent, t.Focus = p.Accent, p.Focus
	t.AccentText = herdrAccentText(t.Accent)
	t.AccentHover = t.Accent.Mix(t.AccentText, .1)
	t.AccentPressed = t.Accent.Mix(t.AccentText, .2)
	t.Selection = p.Selected
	t.Danger, t.Warning, t.Success = p.Danger, tokens["yellow"], p.Success
	t.Inverse, t.InverseText = p.Text, p.Shell
	t.Scrollbar = tokens["overlay1"]
	return &HerdrTheme{Theme: t, Colors: p}
}

func isHerdrReset(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "reset", "default", "none", "transparent":
		return true
	}
	return false
}

// parseHerdrColor follows upstream syntax, including cyan for invalid input.
// Reset resolves against the corresponding native token.
func parseHerdrColor(value string, fallback ui.Color) ui.Color {
	value = strings.ToLower(strings.TrimSpace(value))
	if isHerdrReset(value) {
		return fallback
	}
	if strings.HasPrefix(value, "#") && (len(value) == 4 || len(value) == 7) {
		if _, err := strconv.ParseUint(value[1:], 16, 32); err == nil {
			return ui.Hex(value)
		}
	}
	if strings.HasPrefix(value, "rgb(") && strings.HasSuffix(value, ")") {
		parts := strings.Split(value[4:len(value)-1], ",")
		if len(parts) == 3 {
			var rgb [3]uint8
			valid := true
			for i, part := range parts {
				number, err := strconv.ParseUint(strings.TrimSpace(part), 10, 8)
				if err != nil {
					valid = false
					break
				}
				rgb[i] = uint8(number)
			}
			if valid {
				return ui.RGB(rgb[0], rgb[1], rgb[2])
			}
		}
	}
	if color, ok := herdrANSI[value]; ok {
		return color
	}
	return herdrANSI["cyan"]
}

var herdrANSI = map[string]ui.Color{
	"black": ui.Hex("#000000"), "red": ui.Hex("#800000"),
	"green": ui.Hex("#008000"), "yellow": ui.Hex("#808000"),
	"blue": ui.Hex("#000080"), "magenta": ui.Hex("#800080"), "purple": ui.Hex("#800080"),
	"cyan": ui.Hex("#008080"), "white": ui.Hex("#ffffff"),
	"gray": ui.Hex("#c0c0c0"), "grey": ui.Hex("#c0c0c0"),
	"darkgray": ui.Hex("#808080"), "darkgrey": ui.Hex("#808080"),
	"lightred": ui.Hex("#ff0000"), "lightgreen": ui.Hex("#00ff00"),
	"lightyellow": ui.Hex("#ffff00"), "lightblue": ui.Hex("#0000ff"),
	"lightmagenta": ui.Hex("#ff00ff"), "lightcyan": ui.Hex("#00ffff"),
}

func herdrLuminance(c ui.Color) float64 {
	linear := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= .04045 {
			return x / 12.92
		}
		return math.Pow((x+.055)/1.055, 2.4)
	}
	return .2126*linear(c.R) + .7152*linear(c.G) + .0722*linear(c.B)
}

func herdrAccentText(accent ui.Color) ui.Color {
	if herdrLuminance(accent) > .179 {
		return ui.Hex("#000000")
	}
	return ui.Hex("#ffffff")
}

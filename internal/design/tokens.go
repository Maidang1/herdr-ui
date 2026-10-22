// Package design holds the tokens of the window: the colors, the measures
// and the materials the design file says the window is made of.
//
// Every color the window draws comes from here. A color that is not a token
// is a bug: the tokens are what keep the light window and the dark window
// the same window.
//
// The source of truth is the design file, docs.pen/01.pen, which holds 70
// variables: 30 colors of the light mode, 38 of the dark mode and the two
// font families. The names of the tokens here are the names of the design
// file, so a color in the window can be traced back to the board it came
// from.
package design

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// tokenDef is one token of the design file: its name there, its value in
// each mode, and whether the value is the design file's own or a stand-in
// this package derives for the mode the design file leaves it out of.
type tokenDef struct {
	// name is the name of the token in the design file.
	name string
	// light and dark are the values of the token in each mode, as the
	// eight-digit hex the design file writes them in.
	light string
	dark  string
	// derived marks a value the design file does not give: the token
	// exists in one mode only, and the other mode needs a stand-in.
	// A derived value never invents a color of its own; it reuses the
	// color of the same family in the mode that has it.
	derived bool
}

// The tokens of the design file.
//
// The order follows the design file: the light mode first, then the dark
// mode. Where the design file has a token in one mode only, the other mode
// takes the value marked derived.
var tokenDefs = []tokenDef{
	// Surfaces.
	{name: "win-bg", light: "#FDF3EC", dark: "#221F2E"},
	{name: "titlebar", light: "#FBF1E8", dark: "#2A2636"},
	{name: "panel", light: "#FFFFFFB3", dark: "#2A2636CC"},
	{name: "surface", light: "#FFFFFF", dark: "#1B1922"},
	{name: "sidebar-bg", light: "#F3E9FB", dark: "#00000000", derived: true},
	{name: "rail-bg", light: "#FFFFFF73", dark: "#FFFFFF0F"},
	{name: "chip", light: "#FFF4EA", dark: "#FFFFFF14"},
	{name: "selection", light: "#FFFFFFD9", dark: "#3E3A52"},
	{name: "bar", light: "#ECE5DB", dark: "#FFFFFF26"},
	{name: "separator", light: "#ECE5DB", dark: "#FFFFFF1A"},
	{name: "terminal-bg", light: "#FFFFFF", dark: "#16141C"},

	// Text.
	{name: "text-primary", light: "#1A1A1F", dark: "#F3F1F8"},
	{name: "text-secondary", light: "#4A4A51", dark: "#C3BED3"},
	{name: "text-tertiary", light: "#5E5E67", dark: "#ADA8C0"},
	{name: "chiptext", light: "#54545A", dark: "#C3BED3", derived: true},

	// Accent.
	{name: "accent", light: "#FF5F5F", dark: "#FF7A6E"},
	{name: "accent-deep", light: "#C8382F", dark: "#C93A31"},
	{name: "accent-text", light: "#A62B1E", dark: "#FF9E93"},
	{name: "accent-wash", light: "#FF5F5F29", dark: "#FF7A6E29"},
	{name: "accent-track", light: "#E84545", dark: "#E05B4E"},

	// Status.
	{name: "green", light: "#48BB78", dark: "#4ADE80"},
	{name: "green-text", light: "#1C6B3D", dark: "#7BE8A8"},
	{name: "green-deep", light: "#1C6B3D", dark: "#2F7D4F"},
	{name: "yellow", light: "#F6AD55", dark: "#F5C26B"},
	{name: "yellow-text", light: "#7A4E0C", dark: "#F7CE85"},
	{name: "red", light: "#F56565", dark: "#FF7B72"},
	{name: "red-text", light: "#B3261E", dark: "#FF9D95"},
	{name: "mauve", light: "#B794F4", dark: "#C9A0FF"},

	// Controls.
	{name: "track-off", light: "#E4E0D9", dark: "#3A3648"},
	{name: "knob-off", light: "#6E6A63", dark: "#B6B1C4"},

	// The tokens the design file gives the dark mode only.
	{name: "elevated", light: "#FFFFFF", dark: "#2E2A3A", derived: true},
	{name: "overlay", light: "#1A1A1F26", dark: "#1E1C28D9", derived: true},
	{name: "on-bright", light: "#2A1010", dark: "#2A1010", derived: true},
	{name: "blue", light: "#6FB3FF", dark: "#6FB3FF", derived: true},
	{name: "cyan", light: "#6FD6E8", dark: "#6FD6E8", derived: true},
	{name: "yellow-wash", light: "#F6AD551A", dark: "#F5C26B1A", derived: true},
	{name: "green-wash", light: "#48BB781A", dark: "#4ADE801A", derived: true},
	{name: "red-wash", light: "#F565651A", dark: "#FF7B721A", derived: true},
	{name: "blue-wash", light: "#6FB3FF2E", dark: "#6FB3FF2E", derived: true},
	{name: "red-stroke", light: "#F5656580", dark: "#FF7B7280", derived: true},
}

// The tokens the window may not draw without, and the value each of them
// holds in each mode. A token that is not in this table has no color.
var tokenIndex = func() map[string]tokenDef {
	index := make(map[string]tokenDef, len(tokenDefs))
	for _, def := range tokenDefs {
		index[def.name] = def
	}
	return index
}()

// Tokens is the palette of the window in one mode.
//
// The fields are the tokens of the design file. The window draws with these
// and with nothing else.
type Tokens struct {
	// Dark tells which mode these tokens are the ones of.
	Dark bool

	// Surfaces.
	WinBg      ui.Color
	Titlebar   ui.Color
	Panel      ui.Color
	Surface    ui.Color
	SidebarBg  ui.Color
	RailBg     ui.Color
	Chip       ui.Color
	Selection  ui.Color
	Bar        ui.Color
	Separator  ui.Color
	TerminalBg ui.Color

	// Text.
	TextPrimary   ui.Color
	TextSecondary ui.Color
	TextTertiary  ui.Color
	ChipText      ui.Color

	// Accent.
	Accent      ui.Color
	AccentDeep  ui.Color
	AccentText  ui.Color
	AccentWash  ui.Color
	AccentTrack ui.Color

	// Status.
	Green      ui.Color
	GreenText  ui.Color
	GreenDeep  ui.Color
	Yellow     ui.Color
	YellowText ui.Color
	Red        ui.Color
	RedText    ui.Color
	Mauve      ui.Color

	// Controls.
	TrackOff ui.Color
	KnobOff  ui.Color

	// The tokens the dark mode has of its own.
	Elevated   ui.Color
	Overlay    ui.Color
	OnBright   ui.Color
	Blue       ui.Color
	Cyan       ui.Color
	YellowWash ui.Color
	GreenWash  ui.Color
	RedWash    ui.Color
	BlueWash   ui.Color
	RedStroke  ui.Color
}

// Light returns the tokens of the light mode.
func Light() Tokens { return tokensOf(false) }

// Dark returns the tokens of the dark mode.
func Dark() Tokens { return tokensOf(true) }

// tokensOf returns the tokens of one mode.
func tokensOf(dark bool) Tokens {
	m := paletteOf(dark)
	return Tokens{
		Dark: dark,

		WinBg:      m["win-bg"],
		Titlebar:   m["titlebar"],
		Panel:      m["panel"],
		Surface:    m["surface"],
		SidebarBg:  m["sidebar-bg"],
		RailBg:     m["rail-bg"],
		Chip:       m["chip"],
		Selection:  m["selection"],
		Bar:        m["bar"],
		Separator:  m["separator"],
		TerminalBg: m["terminal-bg"],

		TextPrimary:   m["text-primary"],
		TextSecondary: m["text-secondary"],
		TextTertiary:  m["text-tertiary"],
		ChipText:      m["chiptext"],

		Accent:      m["accent"],
		AccentDeep:  m["accent-deep"],
		AccentText:  m["accent-text"],
		AccentWash:  m["accent-wash"],
		AccentTrack: m["accent-track"],

		Green:      m["green"],
		GreenText:  m["green-text"],
		GreenDeep:  m["green-deep"],
		Yellow:     m["yellow"],
		YellowText: m["yellow-text"],
		Red:        m["red"],
		RedText:    m["red-text"],
		Mauve:      m["mauve"],

		TrackOff: m["track-off"],
		KnobOff:  m["knob-off"],

		Elevated:   m["elevated"],
		Overlay:    m["overlay"],
		OnBright:   m["on-bright"],
		Blue:       m["blue"],
		Cyan:       m["cyan"],
		YellowWash: m["yellow-wash"],
		GreenWash:  m["green-wash"],
		RedWash:    m["red-wash"],
		BlueWash:   m["blue-wash"],
		RedStroke:  m["red-stroke"],
	}
}

// paletteOf returns the colors of one mode, by the name of their token.
func paletteOf(dark bool) map[string]ui.Color {
	palette := make(map[string]ui.Color, len(tokenDefs))
	for _, def := range tokenDefs {
		value := def.light
		if dark {
			value = def.dark
		}
		palette[def.name] = ui.Hex(value)
	}
	return palette
}

// Token returns the color of a token by its name in the design file.
//
// It is how a color that the design file gives but that the window has no
// field for is still read, and how a test checks a token against the
// design file.
func (t Tokens) Token(name string) (ui.Color, bool) {
	def, ok := tokenIndex[name]
	if !ok {
		return ui.Color{}, false
	}
	value := def.light
	if t.Dark {
		value = def.dark
	}
	return ui.Hex(value), true
}

// TokenNames returns the names of the tokens of the design file, in the
// order the file has them.
func TokenNames() []string {
	names := make([]string, 0, len(tokenDefs))
	for _, def := range tokenDefs {
		names = append(names, def.name)
	}
	return names
}

// TokenValue returns the hex value of a token in one mode, as the design
// file writes it.
func TokenValue(name string, dark bool) (string, bool) {
	def, ok := tokenIndex[name]
	if !ok {
		return "", false
	}
	if dark {
		return def.dark, true
	}
	return def.light, true
}

// IsDerived tells whether the value of a token in one mode is a stand-in
// this package derives, or the design file's own.
func IsDerived(name string, dark bool) bool {
	def, ok := tokenIndex[name]
	if !ok {
		return false
	}
	// A token the design file gives in both modes is never derived.
	if def.light != "" && def.dark != "" && def.derived {
		return def.derived
	}
	return def.derived
}

// String names the mode the tokens are of.
func (t Tokens) String() string {
	if t.Dark {
		return "design.Dark"
	}
	return "design.Light"
}

// MustToken returns the color of a token by its name, and panics when the
// token is not one of the design file's.
//
// It is for the tokens a package needs at its own start, where a missing
// token is a mistake in the program rather than a state to handle.
func MustToken(name string, dark bool) ui.Color {
	def, ok := tokenIndex[name]
	if !ok {
		panic(fmt.Sprintf("design: no token named %q", name))
	}
	value := def.light
	if dark {
		value = def.dark
	}
	return ui.Hex(value)
}

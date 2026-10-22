// Package theme reads the colors of the terminal.
//
// The themes are the ones the Rust client uses: a Ghostty theme file, or one
// of the small palettes the Rust client writes by hand. The settings name a
// theme, and a theme may name a different one for the light and the dark
// side of the system.
//
// A theme is only about the terminal: the rest of the window follows the
// system theme of the toolkit the window is drawn with.
package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Theme is the set of colors a terminal paints with.
//
// The colors are 24-bit RGB values, without an alpha channel, the way the
// Rust client keeps them.
type Theme struct {
	// Background is behind the characters, and Foreground is the color of
	// the characters themselves.
	Background uint32
	Foreground uint32
	// Cursor is the color of the cursor.
	Cursor uint32
	// Bold is the color of the bold characters, when the theme sets one.
	Bold *uint32
	// Surface, Active and Muted are the colors of the chrome around the
	// terminal: the face of a row, the row under the pointer, and the text
	// that says less.
	Surface uint32
	Active  uint32
	Muted   uint32
	// Accent is the color of the marks of the window, when the theme sets
	// one.
	Accent *uint32
	// Palette holds the 256 colors of the terminal.
	Palette [256]uint32
}

// The name of the theme that follows what the daemon says.
const FollowHerdr = "Follow Herdr"

// Default returns the theme the window uses when the settings name none.
func Default() Theme {
	theme := Theme{
		Background: 0x101419,
		Foreground: 0xd8dee9,
	}
	theme.Cursor = theme.Foreground
	theme.Palette[0] = 0x000000
	theme.Palette[1] = 0x800000
	theme.Palette[2] = 0x008000
	theme.Palette[3] = 0x808000
	theme.Palette[4] = 0x000080
	theme.Palette[5] = 0x800080
	theme.Palette[6] = 0x008080
	theme.Palette[7] = 0xc0c0c0
	theme.Palette[8] = 0x808080
	theme.Palette[9] = 0xff0000
	theme.Palette[10] = 0x00ff00
	theme.Palette[11] = 0xffff00
	theme.Palette[12] = 0x0000ff
	theme.Palette[13] = 0xff00ff
	theme.Palette[14] = 0x00ffff
	theme.Palette[15] = 0xffffff
	for index := 16; index < 232; index++ {
		n := index - 16
		level := func(value int) uint32 {
			if value == 0 {
				return 0
			}
			return uint32(55 + value*40)
		}
		theme.Palette[index] = level(n/36)<<16 | level(n/6%6)<<8 | level(n%6)
	}
	for index := 232; index < 256; index++ {
		theme.Palette[index] = uint32(8+(index-232)*10) * 0x010101
	}
	theme.deriveChrome()
	return theme
}

// BuiltinNames are the themes that need no file.
var BuiltinNames = []string{"Default", "Nord", "Dracula", "Catppuccin Mocha", "Catppuccin Latte"}

// Builtin returns the palette of a theme the Rust client writes by hand.
func Builtin(name string) (Theme, bool) {
	var background, foreground uint32
	var ansi [16]uint32
	switch name {
	case "Default":
		return Default(), true
	case "Nord":
		background, foreground = 0x2e3440, 0xd8dee9
		ansi = [16]uint32{
			0x3b4252, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x88c0d0, 0xe5e9f0,
			0x4c566a, 0xbf616a, 0xa3be8c, 0xebcb8b, 0x81a1c1, 0xb48ead, 0x8fbcbb, 0xeceff4,
		}
	case "Dracula":
		background, foreground = 0x282a36, 0xf8f8f2
		ansi = [16]uint32{
			0x21222c, 0xff5555, 0x50fa7b, 0xf1fa8c, 0xbd93f9, 0xff79c6, 0x8be9fd, 0xf8f8f2,
			0x6272a4, 0xff6e6e, 0x69ff94, 0xffffa5, 0xd6acff, 0xff92df, 0xa4ffff, 0xffffff,
		}
	case "Catppuccin Mocha":
		background, foreground = 0x1e1e2e, 0xcdd6f4
		ansi = [16]uint32{
			0x45475a, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x94e2d5, 0xbac2de,
			0x585b70, 0xf38ba8, 0xa6e3a1, 0xf9e2af, 0x89b4fa, 0xf5c2e7, 0x8fbcbb, 0xa6adc8,
		}
	case "Catppuccin Latte":
		background, foreground = 0xeff1f5, 0x4c4f69
		ansi = [16]uint32{
			0x5c5f77, 0xd20f39, 0x40a02b, 0xdf8e1d, 0x1e66f5, 0xea76cb, 0x179299, 0xacb0be,
			0x6c6f85, 0xd20f39, 0x40a02b, 0xdf8e1d, 0x1e66f5, 0xea76cb, 0x179299, 0xbcc0cc,
		}
	default:
		return Theme{}, false
	}
	theme := Default()
	theme.Background = background
	theme.Foreground = foreground
	theme.Cursor = foreground
	copy(theme.Palette[:16], ansi[:])
	theme.deriveChrome()
	return theme, true
}

// ParseGhostty reads a Ghostty theme file.
//
// Only the keys that paint the terminal are read: the background, the
// foreground, the cursor, the bold color, and the palette. Everything else
// in the file, includes and commands alike, is left alone.
func ParseGhostty(text string) (Theme, error) {
	theme := Default()
	cursorSet := false
	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		bad := func(reason string) error {
			return fmt.Errorf("herdr-ui: the theme line %d (%s) %s", index+1, key, reason)
		}
		color := func(text string) (uint32, error) {
			hex := strings.TrimPrefix(text, "#")
			if len(hex) != 6 {
				return 0, bad("is not a color")
			}
			for _, digit := range hex {
				if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
					return 0, bad("is not a color")
				}
			}
			number, err := strconv.ParseUint(hex, 16, 32)
			if err != nil {
				return 0, bad("is not a color")
			}
			return uint32(number), nil
		}
		switch key {
		case "background":
			parsed, err := color(value)
			if err != nil {
				return Theme{}, err
			}
			theme.Background = parsed
		case "foreground":
			parsed, err := color(value)
			if err != nil {
				return Theme{}, err
			}
			theme.Foreground = parsed
		case "cursor-color":
			parsed, err := color(value)
			if err != nil {
				return Theme{}, err
			}
			theme.Cursor = parsed
			cursorSet = true
		case "bold-color":
			if value == "bright" {
				continue
			}
			parsed, err := color(value)
			if err != nil {
				return Theme{}, err
			}
			theme.Bold = &parsed
		case "palette":
			indexText, colorText, ok := strings.Cut(value, "=")
			if !ok {
				return Theme{}, bad("has no color")
			}
			position, err := strconv.Atoi(strings.TrimSpace(indexText))
			if err != nil {
				return Theme{}, bad("has no color number")
			}
			if position < 0 || position >= 256 {
				return Theme{}, bad("names a color outside the palette")
			}
			parsed, err := color(strings.TrimSpace(colorText))
			if err != nil {
				return Theme{}, err
			}
			theme.Palette[position] = parsed
		}
	}
	if !cursorSet {
		theme.Cursor = theme.Foreground
	}
	theme.deriveChrome()
	return theme, nil
}

// deriveChrome works out the colors of the chrome from the colors of the
// terminal, the way the Rust client does.
func (t *Theme) deriveChrome() {
	blend := func(percent uint32) uint32 { return Mix(t.Background, t.Foreground, percent) }
	t.Surface = blend(5)
	t.Active = blend(12)
	t.Muted = blend(55)
}

// Mix blends `percent` of `over` onto `base`, channel by channel.
func Mix(base, over uint32, percent uint32) uint32 {
	channel := func(shift uint) uint32 {
		from := (base >> shift) & 255
		to := (over >> shift) & 255
		return (from*(100-percent) + to*percent) / 100
	}
	return channel(16)<<16 | channel(8)<<8 | channel(0)
}

// Pair is the name of a theme, which may be one name for the light side of
// the system and another for the dark side.
type Pair struct {
	// Light is the theme of the light side, and Dark the theme of the
	// dark side. A theme that names only one side has the same name in
	// both.
	Light string
	Dark  string
}

// ParsePair reads the name of a theme.
//
// The name may say `light:Nord,dark:Dracula`, which gives the light and
// the dark side of the system a theme each.
func ParsePair(value string) Pair {
	value = strings.TrimSpace(value)
	side := func(part string) (bool, string, bool) {
		part = strings.TrimSpace(part)
		if rest, ok := strings.CutPrefix(part, "light:"); ok {
			return true, strings.TrimSpace(rest), true
		}
		if rest, ok := strings.CutPrefix(part, "dark:"); ok {
			return false, strings.TrimSpace(rest), true
		}
		return false, "", false
	}
	if _, _, ok := side(value); !ok {
		return Pair{Light: value, Dark: value}
	}
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return Pair{Light: value, Dark: value}
	}
	lightIsLight, light, lightOK := side(parts[0])
	darkIsLight, dark, darkOK := side(parts[1])
	if !lightOK || !darkOK || light == "" || dark == "" {
		return Pair{Light: value, Dark: value}
	}
	if lightIsLight == darkIsLight {
		return Pair{Light: value, Dark: value}
	}
	if lightIsLight {
		return Pair{Light: light, Dark: dark}
	}
	return Pair{Light: dark, Dark: light}
}

// System says whether the name asks for a different theme on each side of
// the system.
func System(value string) bool {
	pair := ParsePair(value)
	return pair.Light != pair.Dark
}

// Name gives the name of the theme of the light or the dark side.
func Name(value string, light bool) string {
	pair := ParsePair(value)
	if light {
		return pair.Light
	}
	return pair.Dark
}

// Directories returns the folders a theme file is looked for in.
//
// The order is the order the Rust client looks in them: the themes of
// Herdr first, then the themes of Ghostty, then the ones the system
// shares.
func Directories() ([]string, error) {
	root, err := configRoot()
	if err != nil {
		return nil, err
	}
	directories := []string{
		filepath.Join(root, "herdr", "themes"),
		filepath.Join(root, "ghostty", "themes"),
	}
	if resources := strings.TrimSpace(os.Getenv("GHOSTTY_RESOURCES_DIR")); resources != "" {
		directories = append(directories, filepath.Join(resources, "themes"))
	}
	directories = append(directories, "/Applications/Ghostty.app/Contents/Resources/ghostty/themes")
	if data := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); data != "" {
		directories = append(directories, filepath.Join(data, "ghostty", "themes"))
	} else if home, err := os.UserHomeDir(); err == nil {
		directories = append(directories, filepath.Join(home, ".local", "share", "ghostty", "themes"))
	}
	if data := strings.TrimSpace(os.Getenv("XDG_DATA_DIRS")); data != "" {
		for _, part := range filepath.SplitList(data) {
			directories = append(directories, filepath.Join(part, "ghostty", "themes"))
		}
	} else {
		directories = append(directories,
			filepath.Join("/usr", "local", "share", "ghostty", "themes"),
			filepath.Join("/usr", "share", "ghostty", "themes"))
	}
	return directories, nil
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

// File returns the file a theme is read from.
//
// A name that starts with `~/` or is absolute names its own file. Any other
// name is looked for in the theme folders, the first one holding it wins.
func File(name string, directories []string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("herdr-ui: the theme has no name")
	}
	if rest, ok := strings.CutPrefix(name, "~/"); ok {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, rest), nil
	}
	if filepath.IsAbs(name) {
		return name, nil
	}
	if filepath.Base(name) != name {
		return "", fmt.Errorf("herdr-ui: %q is not the name of a theme", name)
	}
	for _, directory := range directories {
		candidate := filepath.Join(directory, name)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("herdr-ui: the theme %q is in none of the theme folders", name)
}

// Available returns the names of the themes that can be chosen.
//
// The names the Rust client writes by hand come first, then the names of
// the theme files that were found.
func Available(directories []string) []string {
	names := append([]string{}, BuiltinNames...)
	names = append(names, FollowHerdr)
	seen := map[string]bool{}
	for _, name := range names {
		seen[name] = true
	}
	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			name := entry.Name()
			if seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.SliceStable(names[2:], func(i, j int) bool {
		return strings.ToLower(names[2+i]) < strings.ToLower(names[2+j])
	})
	return names
}

// Resolve reads the theme the settings name.
//
// The name may name a different theme for each side of the system, so the
// side of the system is asked for.
func Resolve(value string, light bool) (Theme, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Default(), nil
	}
	name := Name(value, light)
	directories, err := Directories()
	if err != nil {
		return Theme{}, err
	}
	if name == FollowHerdr {
		// The daemon names its own theme, and the window follows it.
		herdr, err := HerdrThemeName(light)
		if err != nil || herdr == "" {
			return Default(), nil
		}
		found, err := resolveName(herdr, directories)
		if err != nil {
			return Default(), nil
		}
		return found, nil
	}
	return resolveName(name, directories)
}

// HerdrThemeName reads the name of the theme the daemon says.
//
// The daemon keeps its own theme in its own configuration file: the name of
// the theme, and the names it uses when it follows the light and the dark
// side of the system.
func HerdrThemeName(light bool) (string, error) {
	root, err := configRoot()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "herdr", "config.toml"))
	if err != nil {
		return "", err
	}
	table := map[string]any{}
	if err := toml.Unmarshal(data, &table); err != nil {
		return "", err
	}
	values, ok := table["theme"].(map[string]any)
	if !ok {
		return "", nil
	}
	name, _ := values["name"].(string)
	switching, _ := values["auto_switch"].(bool)
	if !switching {
		return name, nil
	}
	if light {
		if light, ok := values["light_name"].(string); ok && light != "" {
			return light, nil
		}
		return name, nil
	}
	if dark, ok := values["dark_name"].(string); ok && dark != "" {
		return dark, nil
	}
	return name, nil
}

// builtinAliases are the names of the daemon that stand for a palette the
// window writes by hand.
var builtinAliases = map[string]string{
	"catppuccin":         "Catppuccin Mocha",
	"catppuccin-mocha":   "Catppuccin Mocha",
	"catppuccin-latte":   "Catppuccin Latte",
	"latte":              "Catppuccin Latte",
	"light":              "Catppuccin Latte",
	"nord":               "Nord",
	"dracula":            "Dracula",
	"terminal":           "Default",
	"default":            "Default",
	"catppuccin-mocha-2": "Catppuccin Mocha",
}

// resolveName reads the theme of a name.
//
// The name is looked for among the palettes the window writes by hand, and
// then among the theme files of the system, which are matched without
// regard to their case.
func resolveName(name string, directories []string) (Theme, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Theme{}, fmt.Errorf("herdr-ui: the theme has no name")
	}
	if found, ok := Builtin(name); ok {
		return found, nil
	}
	lowered := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(name, "_", "-"), " ", "-"))
	if alias, ok := builtinAliases[lowered]; ok {
		if found, ok := Builtin(alias); ok {
			return found, nil
		}
	}
	// A theme may be named after the dark or the light flavour of it.
	candidates := []string{name}
	for _, suffix := range []string{" dark", " light", "-dark", "-light"} {
		candidates = append(candidates, name+suffix)
	}
	path, err := File(name, directories)
	if err == nil {
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			return ParseGhostty(string(data))
		}
	}
	// The theme files of the system are named as they please, so a name is
	// matched against them without regard to their case, and the names of
	// the dark and the light flavour of a theme are tried as well.
	for _, candidate := range candidates {
		wanted := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(candidate, "_", "-"), " ", "-"))
		for _, directory := range directories {
			entries, readErr := os.ReadDir(directory)
			if readErr != nil {
				continue
			}
			for _, entry := range entries {
				info, infoErr := entry.Info()
				if infoErr != nil || !info.Mode().IsRegular() {
					continue
				}
				found := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(entry.Name(), "_", "-"), " ", "-"))
				if found != wanted {
					continue
				}
				data, readErr := os.ReadFile(filepath.Join(directory, entry.Name()))
				if readErr != nil {
					continue
				}
				return ParseGhostty(string(data))
			}
		}
	}
	return Theme{}, fmt.Errorf("herdr-ui: the theme %q is in none of the theme folders", name)
}

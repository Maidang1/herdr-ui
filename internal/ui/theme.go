package ui

import (
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo/ui"
)

// The theme of the terminal.
//
// The window draws its chrome with the system theme of the toolkit, but the
// terminal itself follows the theme the settings name, the way the Rust
// client does: a Ghostty theme file, one of the palettes it writes by hand,
// or a different one for the light and the dark side of the system.

// terminalColors are the colors the terminal paints with.
type terminalColors struct {
	// Foreground is the color of the characters, and Background the color
	// behind them.
	Foreground ui.Color
	Background ui.Color
	// Palette holds the 256 colors of the terminal.
	Palette [256]ui.Color
}

// setThemeName puts the name of the theme into the window.
//
// The themes themselves are read when the window draws, because only then is
// it known which side of the system the window is on.
func (a *App) setThemeName(name string) {
	a.mu.Lock()
	a.themeName = name
	a.themeLight = nil
	a.themeDark = nil
	a.mu.Unlock()
}

// terminalTheme returns the theme of the terminal.
//
// The dark argument says which side of the system the window is on. A theme
// that names one theme for both sides gives it for either.
func (a *App) terminalTheme(dark bool) theme.Theme {
	a.mu.Lock()
	name := a.themeName
	cached := a.themeLight
	if dark {
		cached = a.themeDark
	}
	a.mu.Unlock()

	if cached != nil {
		return *cached
	}
	found, err := theme.Resolve(name, !dark)
	if err != nil {
		found = theme.Default()
		// The notice is given once, so that a theme that cannot be read
		// does not ask for another drawing of the window every time.
		a.mu.Lock()
		changed := a.themeError != err.Error()
		a.themeError = err.Error()
		a.mu.Unlock()
		if changed {
			a.setStatus("could not read the theme: %v", err)
		}
	} else {
		a.mu.Lock()
		a.themeError = ""
		a.mu.Unlock()
	}
	a.mu.Lock()
	if dark {
		a.themeDark = &found
	} else {
		a.themeLight = &found
	}
	a.mu.Unlock()
	return found
}

// terminalColors returns the colors the terminal paints with.
func (a *App) terminalColors(dark bool) terminalColors {
	found := a.terminalTheme(dark)
	colors := terminalColors{
		Foreground: rgbOf(found.Foreground),
		Background: rgbOf(found.Background),
	}
	for index, value := range found.Palette {
		colors.Palette[index] = rgbOf(value)
	}
	return colors
}

// rgbOf turns a color of a theme into one the window paints.
func rgbOf(value uint32) ui.Color {
	return ui.RGB(uint8(value>>16), uint8(value>>8), uint8(value))
}

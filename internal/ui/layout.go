package ui

import (
	"strings"
)

// The layout of the sidebar.
//
// The settings name a layout, the way the Rust client does: `normal`,
// `compact`, `comfortable`, and any of them with `-rounded` at the end.
// The other names of the Rust client, which draw designs of their own,
// stand for the nearest of these.

// Layout says how the sidebar is drawn.
type Layout struct {
	// Name is the name the settings gave.
	Name string
	// Density says how much the sidebar fits: normal, compact or
	// comfortable.
	Density string
	// Style says how the rows are drawn: flat or rounded.
	Style string
	// Minimal says the rows show only the status and the name of the
	// workspace.
	Minimal bool
}

// The layouts the window knows.
const (
	densityNormal      = "normal"
	densityCompact     = "compact"
	densityComfortable = "comfortable"
	styleFlat          = "flat"
	styleRounded       = "rounded"
)

// ParseLayout reads the name of a layout.
//
// A name the window does not know stands for the default one, the way an
// unknown setting does in the Rust client.
func ParseLayout(name string) Layout {
	name = strings.ToLower(strings.TrimSpace(name))
	switch name {
	case "compact":
		return Layout{Name: name, Density: densityCompact, Style: styleFlat}
	case "comfortable":
		return Layout{Name: name, Density: densityComfortable, Style: styleFlat}
	case "normal-rounded":
		return Layout{Name: name, Density: densityNormal, Style: styleRounded}
	case "compact-rounded":
		return Layout{Name: name, Density: densityCompact, Style: styleRounded}
	case "comfortable-rounded":
		return Layout{Name: name, Density: densityComfortable, Style: styleRounded}
	case "superset":
		return Layout{Name: name, Density: densityNormal, Style: styleFlat}
	case "orca":
		return Layout{Name: name, Density: densityComfortable, Style: styleRounded}
	case "minimal":
		return Layout{Name: name, Density: densityNormal, Style: styleFlat, Minimal: true}
	case "devices":
		return Layout{Name: name, Density: densityNormal, Style: styleFlat}
	default:
		return Layout{Name: "normal", Density: densityNormal, Style: styleFlat}
	}
}

// LayoutNames are the layouts the settings may name, in the order the
// settings panel lists them.
var LayoutNames = []string{
	"normal",
	"compact",
	"comfortable",
	"normal-rounded",
	"compact-rounded",
	"comfortable-rounded",
	"superset",
	"orca",
	"minimal",
	"devices",
}

// rowPadding says how much room a row of the sidebar takes at its left.
func (l Layout) rowPadding() float32 {
	switch l.Density {
	case densityCompact:
		return 8
	case densityComfortable:
		return 16
	default:
		return rowPadding
	}
}

// rowGap says how much room the rows keep between them.
func (l Layout) rowGap() float32 {
	switch l.Density {
	case densityCompact:
		return 0
	case densityComfortable:
		return 4
	default:
		return 2
	}
}

// rowFontSize says how large the text of a row is, the size of the window
// aside.
func (l Layout) rowFontSize(size float32) float32 {
	if l.Density == densityCompact {
		return size - 1
	}
	return size
}

// Rounded says the rows are drawn as rounded, bordered cards.
func (l Layout) Rounded() bool { return l.Style == styleRounded }

// sidebarLayout returns the layout of the sidebar.
func (a *App) sidebarLayout() Layout {
	a.mu.Lock()
	defer a.mu.Unlock()
	return ParseLayout(a.layout)
}

// sidebarPadding returns the room the rows of the sidebar keep at their
// left, as the layout of the settings says.
func (a *App) sidebarPadding() float32 {
	return a.sidebarLayout().rowPadding()
}

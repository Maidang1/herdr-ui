package ui

import (
	"embed"
	"strings"
	"sync"

	"herdr-ui/internal/palette"
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo/ui"
)

// The chrome of the window.
//
// The window is a rail of views at the left, the sidebar beside it, the
// terminal in the middle and the bar of the session at the bottom, the way
// the Rust client draws it. The colors come from the palette of the daemon,
// so that the window looks the way the session is themed.
//
//go:embed icons/*.svg
var iconFiles embed.FS

// The measures of the chrome.
//
// They are the measures of the Rust client, so that the two windows look
// the same.
const (
	// railWidth is the width of the column of views at the left.
	railWidth = 48
	// sidebarWidth is the width of the sidebar of the session.
	sidebarWidth = 232
	// rowPadding is the room a row keeps at its left.
	rowPadding = 12
	// statusWidth is the width of the mark of a state.
	statusWidth = 8
	// labelGap is the room between the mark of a state and its label.
	labelGap = 8
	// childIndent is how deep a child row stands under its parent.
	childIndent = 28
	// iconSize is the size of the icons of the window.
	iconSize = 16
	// headerHeight is the height of the head of the sidebar.
	headerHeight = 34
	// sectionHeight is the height of the head of a section.
	sectionHeight = 26
	// tabBarHeight is the height of the strip of the tabs.
	tabBarHeight = 34
	// meterBarHeight is the height of the row of the meters.
	meterBarHeight = 24
)

// The icons of the window, read once from the files of the project.
var (
	iconOnce sync.Once
	iconMap  map[string]*ui.SVG
)

// icon returns the icon of a name.
//
// The name is the name of the file without its suffix, as `panel-left`.
func icon(name string) *ui.SVG {
	iconOnce.Do(func() {
		iconMap = map[string]*ui.SVG{}
		entries, err := iconFiles.ReadDir("icons")
		if err != nil {
			return
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".svg") {
				continue
			}
			data, err := iconFiles.ReadFile("icons/" + entry.Name())
			if err != nil {
				continue
			}
			parsed, err := ui.ParseSVG(data)
			if err != nil {
				continue
			}
			key := strings.TrimSuffix(entry.Name(), ".svg")
			iconMap[key] = parsed
		}
	})
	return iconMap[name]
}

// chrome is the colors the window paints itself with.
//
// The palette is the one the daemon names in its own configuration, and the
// terminal keeps the colors of the theme of the session.
type chrome struct {
	// palette is the palette of the daemon.
	palette palette.Palette
	// background is behind the window.
	background ui.Color
	// panel is behind the panels and the bars.
	panel ui.Color
	// sidebar is behind the sidebar.
	sidebar ui.Color
	// text is the color of the text that matters.
	text ui.Color
	// muted is the color of the text that says less.
	muted ui.Color
	// dim is the color of the marks of the idle rows.
	dim ui.Color
	// accent is the color of the marks of the window.
	accent ui.Color
	// green, yellow and red are the colors of the states of the agents.
	green  ui.Color
	yellow ui.Color
	red    ui.Color
	// mauve is the color of a branch name.
	mauve ui.Color
}

// newChrome works out the colors of the window.
//
// The palette of the daemon comes first, and the terminal behind the window
// stands in for the colors the palette leaves to the terminal itself.
func (a *App) newChrome(dark bool) chrome {
	found := palette.Catppuccin()
	if name, err := theme.HerdrThemeName(!dark); err == nil && name != "" {
		if picked, ok := palette.ByName(name); ok {
			found = picked
		}
	}
	terminal := a.terminalColors(dark)

	colors := chrome{
		palette:    found,
		background: terminal.Background,
		text:       colorOf(found.Text),
		muted:      colorOf(found.Overlay0),
		dim:        colorOf(found.Subtext0),
		accent:     colorOf(found.Accent),
		green:      colorOf(found.Green),
		yellow:     colorOf(found.Yellow),
		red:        colorOf(found.Red),
		mauve:      colorOf(found.Mauve),
	}
	colors.panel = colorOf(found.PanelBg)
	if found.PanelBg.Reset {
		colors.panel = terminal.Background
	}
	colors.sidebar = colorOf(found.SidebarBg)
	if found.SidebarBg.Reset {
		// The sidebar is a panel of its own, so it wears the face of the
		// panels when the palette leaves it to the terminal.
		colors.sidebar = colors.panel
	}
	if found.Text.Reset {
		colors.text = terminal.Foreground
	}
	return colors
}

// stateColor returns the color of a state of an agent.
func (c chrome) stateColor(status string) ui.Color {
	switch status {
	case "idle", "done":
		return c.green
	case "working", "running":
		return c.yellow
	case "blocked", "needs attention", "error":
		return c.red
	default:
		return c.dim
	}
}

// stateMark returns the mark of a state of an agent.
func stateMark(status string) *ui.SVG {
	switch status {
	case "idle":
		return icon("status-mark-idle")
	case "done":
		return icon("status-mark-done")
	case "working", "running":
		return icon("status-mark-working")
	case "blocked", "needs attention", "error":
		return icon("status-mark-blocked")
	default:
		return icon("status-mark-unknown")
	}
}

// agentIcon returns the icon of an agent, by its name.
func agentIcon(name string) *ui.SVG {
	switch strings.ToLower(name) {
	case "claude":
		return icon("agent-claude")
	case "codex":
		return icon("agent-codex")
	case "copilot":
		return icon("agent-copilot")
	case "grok":
		return icon("agent-grok")
	case "omp":
		return icon("agent-omp")
	case "pi":
		return icon("agent-pi")
	case "kilo":
		return icon("agent-kilo")
	default:
		return icon("agent-generic")
	}
}

// colorOf turns a color of a palette into one the window paints.
func colorOf(value palette.Color) ui.Color {
	if value.Reset {
		return ui.RGBA(0, 0, 0, 0)
	}
	return ui.RGB(value.R, value.G, value.B)
}

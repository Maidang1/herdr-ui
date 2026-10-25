// Package design maps the main specification's semantic tokens to native UI.
package design

import "github.com/egoist/mygo/ui"

type Palette struct {
	Shell, Top, Middle, Bottom, Sidebar, Content, Inset, Text, Secondary, Tertiary, Border, Selected, Hover, Accent, Working, Idle, Warning, Danger, Success ui.Color
	Elevated, ControlBorder, Focus, InactiveSelected, SidebarTint, ContentShadow, Disabled, Pressed                                                          ui.Color
	// RowSelected is the selected sidebar row's wash: a neutral fill, as
	// a Mac sidebar's, that the material under it tints.
	RowSelected ui.Color
	// Highlight is the hairline of light along a raised surface's top
	// edge in dark mode, where a shadow alone does not lift it; light mode
	// leaves it clear.
	Highlight ui.Color
	// Grouped is the opaque page a settings window's grouped cards sit on.
	Grouped ui.Color
}

// Colors is the window's palette. Surfaces are neutral, as Apple's, so the
// behind-window material carries the wallpaper's tint rather than fighting
// it; labels are white or black at an opacity, as AppKit's label colors,
// so whatever they sit on reads through them.
func Colors(dark bool) Palette {
	if dark {
		return Palette{
			Shell: ui.Hex("#2A2A2D"), Top: ui.Hex("#2D2D30"), Middle: ui.Hex("#2A2A2D"), Bottom: ui.Hex("#27272A"), Sidebar: ui.Hex("#2A2A2D"), Content: ui.Hex("#1E1E20"), Inset: ui.Hex("#FFFFFF0F"), Text: ui.Hex("#FFFFFFE0"), Secondary: ui.Hex("#FFFFFFA6"), Tertiary: ui.Hex("#FFFFFF85"), Border: ui.Hex("#FFFFFF14"), Selected: ui.Hex("#FFFFFF1A"), Hover: ui.Hex("#FFFFFF0D"), Accent: ui.Hex("#78B7FF"), Working: ui.Hex("#C4A6EA"), Idle: ui.Hex("#A1A1A8"), Warning: ui.Hex("#E7BF72"), Danger: ui.Hex("#FF9CA5"), Success: ui.Hex("#8EE0AB"),
			Elevated: ui.Hex("#2C2C2F"), ControlBorder: ui.Hex("#8E8E95"), Focus: ui.Hex("#78B7FF"), InactiveSelected: ui.Hex("#FFFFFF12"), SidebarTint: ui.Hex("#1E1E2026"), ContentShadow: ui.Hex("#00000059"), Disabled: ui.Hex("#6E6E75"), Pressed: ui.Hex("#FFFFFF17"), RowSelected: ui.Hex("#FFFFFF1C"), Highlight: ui.Hex("#FFFFFF12"), Grouped: ui.Hex("#1E1E20"),
		}
	}
	return Palette{
		Shell: ui.Hex("#ECECEF"), Top: ui.Hex("#EFEFF2"), Middle: ui.Hex("#ECECEF"), Bottom: ui.Hex("#E8E8EB"), Sidebar: ui.Hex("#ECECEF"), Content: ui.Hex("#FFFFFF"), Inset: ui.Hex("#0000000A"), Text: ui.Hex("#000000DE"), Secondary: ui.Hex("#000000A8"), Tertiary: ui.Hex("#0000008F"), Border: ui.Hex("#00000014"), Selected: ui.Hex("#00000012"), Hover: ui.Hex("#0000000A"), Accent: ui.Hex("#0057A6"), Working: ui.Hex("#68448F"), Idle: ui.Hex("#5C5C63"), Warning: ui.Hex("#795000"), Danger: ui.Hex("#9F2934"), Success: ui.Hex("#1E7546"),
		Elevated: ui.Hex("#FFFFFF"), ControlBorder: ui.Hex("#76767D"), Focus: ui.Hex("#0057A6"), InactiveSelected: ui.Hex("#0000000D"), SidebarTint: ui.Hex("#F6F6F840"), ContentShadow: ui.Hex("#0000001A"), Disabled: ui.Hex("#8E8E94"), Pressed: ui.Hex("#00000017"), RowSelected: ui.Hex("#00000014"), Highlight: ui.Hex("#FFFFFF00"), Grouped: ui.Hex("#F0F0F3"),
	}
}

// Over is fg composited over the opaque bg: the solid color a translucent
// label shows on it, for consumers that take no alpha, as a terminal's
// foreground.
func Over(fg, bg ui.Color) ui.Color {
	a := float32(fg.A) / 255
	mix := func(f, b uint8) uint8 { return uint8(float32(f)*a + float32(b)*(1-a) + .5) }
	return ui.RGBA(mix(fg.R, bg.R), mix(fg.G, bg.G), mix(fg.B, bg.B), 1)
}

func WindowColors(dark, highContrast, inactive bool) Palette {
	p := Colors(dark)
	if inactive {
		p.Selected = p.InactiveSelected
	}
	if highContrast {
		p.Border = p.ControlBorder
	}
	return p
}

// NativeTheme keeps native controls, menus and focus rings on the same tokens.
// An empty Font selects the platform system UI font.
func NativeTheme(dark, highContrast bool) *ui.Theme {
	t := ui.LightTheme()
	if dark {
		t = ui.DarkTheme()
	}
	p := WindowColors(dark, highContrast, false)
	t.Background = p.Shell
	t.Surface = p.Elevated
	t.SurfaceHover = p.Hover
	t.SurfacePressed = p.Pressed
	t.Border = p.ControlBorder
	t.Text = p.Text
	t.TextMuted = p.Secondary
	t.Accent = p.Accent
	t.Focus = p.Focus
	t.Danger = p.Danger
	t.Warning = p.Warning
	t.Success = p.Success
	t.Font = ""
	t.FontSize = 13
	t.Radius = 6
	if dark {
		t.AccentText = ui.Hex("#14263A")
		t.AccentHover = ui.Hex("#92C5FF")
		t.AccentPressed = ui.Hex("#61A5F1")
		t.Selection = ui.Hex("#354B66")
	} else {
		t.AccentText = ui.Hex("#FFFFFF")
		t.AccentHover = ui.Hex("#00529D")
		t.AccentPressed = ui.Hex("#004587")
		t.Selection = ui.Hex("#DCEAF8")
	}
	return t
}

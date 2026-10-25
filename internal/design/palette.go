// Package design maps the main specification's semantic tokens to native UI.
package design

import "github.com/egoist/mygo/ui"

type Palette struct {
	Shell, Top, Middle, Bottom, Sidebar, Content, Inset, Text, Secondary, Tertiary, Border, Selected, Hover, Accent, Working, Idle, Warning, Danger, Success ui.Color
	Elevated, ControlBorder, Focus, InactiveSelected, SidebarTint, ContentShadow, Disabled, Pressed                                                          ui.Color
	// RowSelected is the selected sidebar row's wash, in the shell's own
	// lavender hue rather than the action blue.
	RowSelected ui.Color
}

func Colors(dark bool) Palette {
	if dark {
		return Palette{
			Shell: ui.Hex("#211D28"), Top: ui.Hex("#2C2435"), Middle: ui.Hex("#2C252C"), Bottom: ui.Hex("#302D25"), Sidebar: ui.Hex("#201D24"), Content: ui.Hex("#211F25"), Inset: ui.Hex("#29262F"), Text: ui.Hex("#F2EFF5"), Secondary: ui.Hex("#C2BBCB"), Tertiary: ui.Hex("#B0A7BD"), Border: ui.Hex("#FFFFFF12"), Selected: ui.Hex("#FFFFFF1F"), Hover: ui.Hex("#FFFFFF0A"), Accent: ui.Hex("#78B7FF"), Working: ui.Hex("#C4A6EA"), Idle: ui.Hex("#B0A7BD"), Warning: ui.Hex("#E7BF72"), Danger: ui.Hex("#FF9CA5"), Success: ui.Hex("#8EE0AB"),
			Elevated: ui.Hex("#302D35"), ControlBorder: ui.Hex("#8F859B"), Focus: ui.Hex("#78B7FF"), InactiveSelected: ui.Hex("#FFFFFF14"), SidebarTint: ui.Hex("#1D1A2166"), ContentShadow: ui.Hex("#00000029"), Disabled: ui.Hex("#776E82"), Pressed: ui.Hex("#FFFFFF14"), RowSelected: ui.Hex("#C4A6EA2E"),
		}
	}
	return Palette{
		Shell: ui.Hex("#EEE7EF"), Top: ui.Hex("#EAE0EF"), Middle: ui.Hex("#F3E8E7"), Bottom: ui.Hex("#F4EED3"), Sidebar: ui.Hex("#F2F0F4"), Content: ui.Hex("#FFFFFF"), Inset: ui.Hex("#F5F3F7"), Text: ui.Hex("#252329"), Secondary: ui.Hex("#5D5866"), Tertiary: ui.Hex("#625D6B"), Border: ui.Hex("#3026380F"), Selected: ui.Hex("#30263814"), Hover: ui.Hex("#3026380B"), Accent: ui.Hex("#0057A6"), Working: ui.Hex("#68448F"), Idle: ui.Hex("#5D5866"), Warning: ui.Hex("#795000"), Danger: ui.Hex("#9F2934"), Success: ui.Hex("#1E7546"),
		Elevated: ui.Hex("#FFFFFF"), ControlBorder: ui.Hex("#756D80"), Focus: ui.Hex("#0057A6"), InactiveSelected: ui.Hex("#3026380F"), SidebarTint: ui.Hex("#F9F7FB73"), ContentShadow: ui.Hex("#3026380A"), Disabled: ui.Hex("#8C8595"), Pressed: ui.Hex("#30263817"), RowSelected: ui.Hex("#7A55A82B"),
	}
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

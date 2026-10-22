package design

import "github.com/egoist/mygo/ui"

// MygoTheme projects the tokens onto the theme of the toolkit.
//
// The theme of the toolkit has fewer slots than the design file has tokens,
// so it cannot hold them all: it is what the widgets of the toolkit fall
// back on, and what the window draws its own parts with. The window's own
// parts read the tokens, never the theme.
//
// Every color of the theme comes from a token, so that a change of the
// tokens reaches the toolkit's widgets too.
func (t Tokens) MygoTheme() *ui.Theme {
	surfaces := t.Surfaces()
	theme := &ui.Theme{
		Dark: t.Dark,

		Background: t.WinBg,

		Surface:        t.Surface,
		SurfaceHover:   surfaces.Solid.Hover,
		SurfacePressed: surfaces.Solid.Pressed,

		Border: t.Separator,

		Text:      t.TextPrimary,
		TextMuted: t.TextSecondary,

		Accent:        t.Accent,
		AccentHover:   t.accentHover(),
		AccentPressed: t.accentPressed(),
		AccentText:    t.OnBright,

		Danger:  t.Red,
		Warning: t.Yellow,
		Success: t.Green,

		Selection: t.Selection,
		Focus:     t.FocusRing(),

		// The toolkit turns the theme over for a tooltip or a toast: the
		// text becomes the ground and the ground becomes the text.
		Inverse:     t.TextPrimary,
		InverseText: t.WinBg,

		Scrollbar:      t.KnobOff,
		ScrollbarWidth: 6,

		Radius:   RadiusMD,
		Spacing:  SpaceSM,
		FontSize: FontUI,
		Font:     FamilyUI,
	}
	return theme
}

// accentHover returns the accent a little brighter than it is.
func (t Tokens) accentHover() ui.Color {
	if t.Dark {
		return lighten(t.Accent, 0.12)
	}
	return darken(t.Accent, 0.06)
}

// accentPressed returns the accent a little darker than it is.
func (t Tokens) accentPressed() ui.Color {
	if t.Dark {
		return darken(t.Accent, 0.10)
	}
	return darken(t.Accent, 0.16)
}

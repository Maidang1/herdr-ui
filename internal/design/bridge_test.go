package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheThemeIsMadeOfTokens checks that every slot of the theme of the
// toolkit holds the color of a token, and not a color of its own.
func TestTheThemeIsMadeOfTokens(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		theme := mode.toks.MygoTheme()
		slots := []struct {
			name string
			got  ui.Color
			want ui.Color
		}{
			{"the ground of the window", theme.Background, mode.toks.WinBg},
			{"the face of a control", theme.Surface, mode.toks.Surface},
			{"the line of a control", theme.Border, mode.toks.Separator},
			{"the text", theme.Text, mode.toks.TextPrimary},
			{"the text that says less", theme.TextMuted, mode.toks.TextSecondary},
			{"the accent", theme.Accent, mode.toks.Accent},
			{"the text on the accent", theme.AccentText, mode.toks.OnBright},
			{"what is wrong", theme.Danger, mode.toks.Red},
			{"what wants attention", theme.Warning, mode.toks.Yellow},
			{"what went well", theme.Success, mode.toks.Green},
			{"what is chosen", theme.Selection, mode.toks.Selection},
			{"the ring of the keyboard", theme.Focus, mode.toks.Accent},
			{"the scroll bar", theme.Scrollbar, mode.toks.KnobOff},
		}
		for _, slot := range slots {
			if slot.got != slot.want {
				t.Errorf("%s: %s is %v, the token says %v", mode.name, slot.name, slot.got, slot.want)
			}
			if slot.got.A == 0 {
				t.Errorf("%s: %s has no color at all", mode.name, slot.name)
			}
		}
	}
}

// TestTheThemeFollowsTheMode checks that the theme says which mode it is
// of, so that the toolkit's own widgets can tell.
func TestTheThemeFollowsTheMode(t *testing.T) {
	if Light().MygoTheme().Dark {
		t.Error("the light theme says it is dark")
	}
	if !Dark().MygoTheme().Dark {
		t.Error("the dark theme says it is light")
	}
}

// TestTheThemeMeasures checks that the measures of the theme come from the
// measures of the design.
func TestTheThemeMeasures(t *testing.T) {
	theme := Light().MygoTheme()
	if theme.Radius != RadiusMD {
		t.Errorf("the corners of the toolkit's widgets are %v, the design file says %v", theme.Radius, RadiusMD)
	}
	if theme.Spacing != SpaceSM {
		t.Errorf("the space of the toolkit's widgets is %v, the design file says %v", theme.Spacing, SpaceSM)
	}
	if theme.FontSize != FontUI {
		t.Errorf("the text of the toolkit's widgets is %v, the design file says %v", theme.FontSize, FontUI)
	}
	if theme.Font != FamilyUI {
		t.Errorf("the family of the toolkit's widgets is %q, the design file says %q", theme.Font, FamilyUI)
	}
}

// TestTheAccentMoves checks that the accent has a lighter and a darker
// shade, so that a button can be hovered and pressed.
func TestTheAccentMoves(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		theme := mode.toks.MygoTheme()
		if theme.AccentHover == theme.Accent {
			t.Errorf("%s: the accent does not change under the pointer", mode.name)
		}
		if theme.AccentPressed == theme.Accent {
			t.Errorf("%s: the accent does not change when it is pressed", mode.name)
		}
		if theme.AccentPressed == theme.AccentHover {
			t.Errorf("%s: the accent is the same hovered and pressed", mode.name)
		}
	}
}

package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTheIconRules checks the rule of the icons: an icon at rest is the
// third grade of text, and hovering brings it up to the first grade.
func TestTheIconRules(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		if got := mode.toks.Icon(StateNormal); got != mode.toks.TextTertiary {
			t.Errorf("%s: the icon at rest is %v, the design file says the third grade of text", mode.name, got)
		}
		for _, state := range []Interaction{StateHover, StatePress, StateFocus} {
			if got := mode.toks.Icon(state); got != mode.toks.TextPrimary {
				t.Errorf("%s: the icon %s is %v, the design file says the first grade of text", mode.name, state, got)
			}
		}
	}
}

// TestThePressRule checks that a pressed control is six percent darker
// than a hovered one.
func TestThePressRule(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		base := mode.toks.Surface
		hover := mode.toks.Fill(base, StateHover)
		press := mode.toks.Fill(base, StatePress)
		if hover == base {
			t.Errorf("%s: the control does not change under the pointer", mode.name)
		}
		if press == hover {
			t.Errorf("%s: the control does not change when it is pressed", mode.name)
		}
		want := darken(hover, PressDarken)
		if press != want {
			t.Errorf("%s: the pressed control is %v, the design file says %v", mode.name, press, want)
		}
	}
}

// TestTheChipHover checks the rule of the chip: under the pointer it takes
// the color of a warm paper.
func TestTheChipHover(t *testing.T) {
	light := Light()
	if got := light.Fill(light.Chip, StateHover); got != ui.Hex("#FFEFE2") {
		t.Errorf("the hovered chip is %v, the design file says #FFEFE2", got)
	}
	dark := Dark()
	if got := dark.Fill(dark.Chip, StateHover); got == dark.Chip {
		t.Error("the dark chip does not change under the pointer")
	}
}

// TestTheDisabledRule checks that a control that cannot be used keeps its
// geometry and only its colors change.
func TestTheDisabledRule(t *testing.T) {
	light := Light()
	if got := light.Label(light.TextPrimary, StateDisabled); got != ui.Hex("#6B5C58") {
		t.Errorf("the disabled label is %v, the design file says #6B5C58", got)
	}
	dark := Dark()
	if got := dark.Label(dark.TextPrimary, StateDisabled); got == dark.TextPrimary {
		t.Error("the dark disabled label is the label of a control that can be used")
	}
}

// TestTheFocusRing checks that the ring of the keyboard is the accent and
// that nothing takes it away.
func TestTheFocusRing(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		if got := mode.toks.FocusRing(); got != mode.toks.Accent {
			t.Errorf("%s: the ring is %v, the design file says the accent", mode.name, got)
		}
		if FocusRingWidth != 2 || FocusRingOffset != 2 {
			t.Errorf("the ring is %v wide and %v away, the design file says 2 and 2", FocusRingWidth, FocusRingOffset)
		}
	}
}

// TestTheSelectedRule checks that a chosen thing wears the color of the
// selection.
func TestTheSelectedRule(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
	}{
		{"light", Light()},
		{"dark", Dark()},
	} {
		if got := mode.toks.Fill(mode.toks.Surface, StateSelected); got != mode.toks.Selection {
			t.Errorf("%s: the chosen control is %v, the design file says the selection", mode.name, got)
		}
	}
}

// TestTheHitArea checks that every control keeps a place to hit, whether
// or not its icon is big enough to fill it.
func TestTheHitArea(t *testing.T) {
	if HitArea != 24 {
		t.Errorf("the smallest place to hit is %d, the design file says 24", HitArea)
	}
}

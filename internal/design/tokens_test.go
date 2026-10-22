package design

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The variables of the design file, docs.pen/01.pen, as they are there.
//
// The light mode has 30 colors and the dark mode 38, and the two of them
// share 28; the ten the dark mode has of its own are marked here by their
// missing light value, and the two the light mode has of its own by their
// missing dark value.
//
// This table is the copy of the design file the tests hold the tokens
// against: when a token and the design file disagree, one of the two is
// wrong and the test says so.
var designFileVariables = map[string]struct{ light, dark string }{
	"win-bg":         {"#FDF3EC", "#221F2E"},
	"titlebar":       {"#FBF1E8", "#2A2636"},
	"panel":          {"#FFFFFFB3", "#2A2636CC"},
	"surface":        {"#FFFFFF", "#1B1922"},
	"sidebar-bg":     {"#F3E9FB", ""},
	"rail-bg":        {"#FFFFFF73", "#FFFFFF0F"},
	"chip":           {"#FFF4EA", "#FFFFFF14"},
	"selection":      {"#FFFFFFD9", "#3E3A52"},
	"bar":            {"#ECE5DB", "#FFFFFF26"},
	"separator":      {"#ECE5DB", "#FFFFFF1A"},
	"terminal-bg":    {"#FFFFFF", "#16141C"},
	"text-primary":   {"#1A1A1F", "#F3F1F8"},
	"text-secondary": {"#4A4A51", "#C3BED3"},
	"text-tertiary":  {"#5E5E67", "#ADA8C0"},
	"chiptext":       {"#54545A", ""},
	"accent":         {"#FF5F5F", "#FF7A6E"},
	"accent-deep":    {"#C8382F", "#C93A31"},
	"accent-text":    {"#A62B1E", "#FF9E93"},
	"accent-wash":    {"#FF5F5F29", "#FF7A6E29"},
	"accent-track":   {"#E84545", "#E05B4E"},
	"green":          {"#48BB78", "#4ADE80"},
	"green-text":     {"#1C6B3D", "#7BE8A8"},
	"green-deep":     {"#1C6B3D", "#2F7D4F"},
	"yellow":         {"#F6AD55", "#F5C26B"},
	"yellow-text":    {"#7A4E0C", "#F7CE85"},
	"red":            {"#F56565", "#FF7B72"},
	"red-text":       {"#B3261E", "#FF9D95"},
	"mauve":          {"#B794F4", "#C9A0FF"},
	"track-off":      {"#E4E0D9", "#3A3648"},
	"knob-off":       {"#6E6A63", "#B6B1C4"},
	"elevated":       {"", "#2E2A3A"},
	"overlay":        {"", "#1E1C28D9"},
	"on-bright":      {"", "#2A1010"},
	"blue":           {"", "#6FB3FF"},
	"cyan":           {"", "#6FD6E8"},
	"yellow-wash":    {"", "#F5C26B1A"},
	"green-wash":     {"", "#4ADE801A"},
	"red-wash":       {"", "#FF7B721A"},
	"blue-wash":      {"", "#6FB3FF2E"},
	"red-stroke":     {"", "#FF7B7280"},
}

// TestTokensMatchDesignFile checks every token against the design file.
func TestTokensMatchDesignFile(t *testing.T) {
	for _, def := range tokenDefs {
		want, ok := designFileVariables[def.name]
		if !ok {
			t.Errorf("token %q is not in the design file", def.name)
			continue
		}
		if want.light != "" && def.light != want.light {
			t.Errorf("token %q: light value is %s, the design file says %s", def.name, def.light, want.light)
		}
		if want.dark != "" && def.dark != want.dark {
			t.Errorf("token %q: dark value is %s, the design file says %s", def.name, def.dark, want.dark)
		}
		// A token the design file gives in one mode only must be marked
		// as derived, so that the stand-in in the other mode is known
		// to be one.
		if (want.light == "" || want.dark == "") && !def.derived {
			t.Errorf("token %q: the design file gives it in one mode only, so its other value must be marked derived", def.name)
		}
		if want.light != "" && want.dark != "" && def.derived {
			t.Errorf("token %q: the design file gives it in both modes, so it must not be marked derived", def.name)
		}
	}
}

// TestEveryDesignVariableHasAToken checks that no variable of the design
// file is left out.
func TestEveryDesignVariableHasAToken(t *testing.T) {
	for name := range designFileVariables {
		if _, ok := tokenIndex[name]; !ok {
			t.Errorf("the design file has a token %q that the package does not", name)
		}
	}
}

// TestTokenCount checks the number of the tokens: the design file has 40
// colors, 30 of the light mode and 38 of the dark mode.
func TestTokenCount(t *testing.T) {
	if len(tokenDefs) != 40 {
		t.Errorf("the package holds %d tokens, the design file has 40", len(tokenDefs))
	}
	light, dark := 0, 0
	for _, def := range designFileVariables {
		if def.light != "" {
			light++
		}
		if def.dark != "" {
			dark++
		}
	}
	if light != 30 {
		t.Errorf("the design file has %d light colors, the copy says 30", light)
	}
	if dark != 38 {
		t.Errorf("the design file has %d dark colors, the copy says 38", dark)
	}
}

// TestTokenNamesAreUnique checks that no name is taken twice.
func TestTokenNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, def := range tokenDefs {
		if seen[def.name] {
			t.Errorf("the token %q is defined twice", def.name)
		}
		seen[def.name] = true
	}
}

// TestTokensHoldTheirColors checks that the tokens of a mode hold the
// colors of the design file.
func TestTokensHoldTheirColors(t *testing.T) {
	for _, mode := range []struct {
		name string
		toks Tokens
		dark bool
	}{
		{"light", Light(), false},
		{"dark", Dark(), true},
	} {
		for name, want := range designFileVariables {
			value := want.light
			if mode.dark {
				value = want.dark
			}
			if value == "" {
				continue
			}
			got, ok := mode.toks.Token(name)
			if !ok {
				t.Errorf("%s: no token named %q", mode.name, name)
				continue
			}
			if got != ui.Hex(value) {
				t.Errorf("%s: token %q is %v, the design file says %s", mode.name, name, got, value)
			}
		}
	}
}

// TestEveryTokenHasAField checks that the tokens are reachable through the
// fields of the struct and not only through the table.
func TestEveryTokenHasAField(t *testing.T) {
	light, dark := Light(), Dark()
	for _, name := range TokenNames() {
		if _, ok := light.Token(name); !ok {
			t.Errorf("the light tokens have no %q", name)
		}
		if _, ok := dark.Token(name); !ok {
			t.Errorf("the dark tokens have no %q", name)
		}
	}
}

// TestTheModesDiffer checks that the two modes are two windows and not one
// window twice.
func TestTheModesDiffer(t *testing.T) {
	light, dark := Light(), Dark()
	if light.Dark || !dark.Dark {
		t.Fatal("the tokens do not say which mode they are of")
	}
	same := 0
	for _, name := range TokenNames() {
		l, _ := light.Token(name)
		d, _ := dark.Token(name)
		if l == d {
			same++
		}
	}
	// The tokens the two modes share are the two the design file gives
	// in one mode only, plus the few that happen to be the same color.
	if same > 6 {
		t.Errorf("%d tokens are the same in both modes, which is more than the design file has", same)
	}
}

// TestMustTokenPanicsOnAnUnknownName checks that a missing token is a
// mistake in the program and says so.
func TestMustTokenPanicsOnAnUnknownName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a missing token did not stop the program")
		}
	}()
	MustToken("no-such-token", false)
}

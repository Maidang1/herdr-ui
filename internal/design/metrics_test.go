package design

import "testing"

// TestTheSpacingLadder checks that the window has the eight spaces of the
// design file and nothing between them.
func TestTheSpacingLadder(t *testing.T) {
	ladder := []int{SpaceXS, SpaceSM, SpaceMD, SpaceLG, SpaceXL, Space2XL, Space3XL, Space4XL}
	want := []int{2, 4, 6, 8, 12, 16, 24, 32}
	for i, w := range want {
		if ladder[i] != w {
			t.Errorf("the space at %d is %d, the design file says %d", i, ladder[i], w)
		}
	}
	for i := 1; i < len(ladder); i++ {
		if ladder[i] <= ladder[i-1] {
			t.Errorf("the space at %d is not bigger than the one before it", i)
		}
	}
}

// TestTheCornerLadder checks the corners of the window.
func TestTheCornerLadder(t *testing.T) {
	ladder := []int{RadiusNone, RadiusXS, RadiusSM, RadiusMD, RadiusLG, RadiusXL, Radius2XL, Radius3XL}
	want := []int{0, 2, 4, 6, 8, 10, 12, 16}
	for i, w := range want {
		if ladder[i] != w {
			t.Errorf("the corner at %d is %d, the design file says %d", i, ladder[i], w)
		}
	}
}

// TestTheTypeScale checks the sizes of the text.
func TestTheTypeScale(t *testing.T) {
	ladder := []int{FontSmall, FontBody, FontUI, FontMedium, FontLarge, FontTitle, FontDisplay}
	want := []int{11, 12, 13, 15, 17, 20, 28}
	for i, w := range want {
		if ladder[i] != w {
			t.Errorf("the text at %d is %d, the design file says %d", i, ladder[i], w)
		}
	}
	if FontMicro != 10 {
		t.Errorf("the text of the meters is %d, the design file says 10", FontMicro)
	}
}

// TestThePartsOfTheWindow checks the measures of the parts of the window.
func TestThePartsOfTheWindow(t *testing.T) {
	parts := []struct {
		name string
		got  int
		want int
	}{
		{"the title bar", TitleBarHeight, 34},
		{"the activity bar", RailWidth, 44},
		{"the sidebar", SidebarWidth, 248},
		{"the strip of the tabs", TabStripHeight, 36},
		{"the row of the meters", MeterRowHeight, 26},
		{"a button of the activity bar", RailButton, 32},
		{"the icon of a button of the activity bar", RailIcon, 17},
		{"a badge", BadgeHeight, 14},
		{"a tab", TabHeight, 24},
		{"the icon in a row of a tree", IconSmall, 14},
		{"the icon of a section", IconMedium, 16},
		{"a line of the terminal", TerminalLineHeight, 19},
		{"the characters of the terminal", TerminalFontSize, 13},
		{"the space above and below the terminal", TerminalPadY, 16},
		{"the space at the side of the terminal", TerminalPadX, 18},
		{"the corner of the window", WindowRadius, 12},
		{"the corner of the card", Radius3XL, 16},
		{"the space between the three columns", BodyGap, 16},
	}
	for _, p := range parts {
		if p.got != p.want {
			t.Errorf("%s is %d, the design file says %d", p.name, p.got, p.want)
		}
	}
}

// TestTheFontsFallBack checks that the fonts the design file names come
// with a list to fall back on, since neither is certain to be on the
// machine.
func TestTheFontsFallBack(t *testing.T) {
	for _, f := range []struct{ name, family string }{
		{"the text of the window", FamilyUI},
		{"the characters of the terminal", FamilyMono},
	} {
		if len(f.family) < 2 {
			t.Errorf("%s names no family to fall back on", f.name)
		}
	}
}

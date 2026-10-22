package ui

import (
	"testing"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

func TestTheColorOfACellFollowsTheDaemon(t *testing.T) {
	// A named color of the palette of sixteen.
	colors := defaultColors()
	if got := resolveColor(3, colors.Foreground, &colors.Palette); got != palette16[2] {
		t.Fatalf("the color came out as %+v, want %+v", got, palette16[2])
	}
	// An index of the palette of 256.
	if got := resolveColor(0x01000005, colors.Foreground, &colors.Palette); got != palette256[5] {
		t.Fatalf("the color came out as %+v, want %+v", got, palette256[5])
	}
	// A red, green and blue triple.
	if got := resolveColor(0x02FF8000, colors.Foreground, &colors.Palette); got != ui.RGB(255, 128, 0) {
		t.Fatalf("the color came out as %+v", got)
	}
	// Anything else is the color of the terminal itself.
	if got := resolveColor(0, colors.Foreground, &colors.Palette); got != colors.Foreground {
		t.Fatalf("the color came out as %+v, want %+v", got, defaultForeground)
	}
	if got := resolveColor(0x03000000, colors.Foreground, &colors.Palette); got != colors.Foreground {
		t.Fatalf("the color came out as %+v, want %+v", got, defaultForeground)
	}
}

func TestReversedCellsSwapTheirColors(t *testing.T) {
	cell := herdr.Cell{
		Symbol:   "x",
		FG:       0x02FF0000,
		BG:       0x020000FF,
		Modifier: modifierReversed,
	}
	colors := defaultColors()
	fg, bg := cellColors(cell, colors.Foreground, colors.Background, &colors.Palette)
	if fg != ui.RGB(0, 0, 255) || bg != ui.RGB(255, 0, 0) {
		t.Fatalf("the colors came out as %+v and %+v", fg, bg)
	}
}

func TestHiddenCellsTakeTheBackground(t *testing.T) {
	cell := herdr.Cell{
		Symbol:   "x",
		FG:       0x02FF0000,
		BG:       0x020000FF,
		Modifier: modifierHidden,
	}
	colors := defaultColors()
	fg, _ := cellColors(cell, colors.Foreground, colors.Background, &colors.Palette)
	if fg != ui.RGB(0, 0, 255) {
		t.Fatalf("the color came out as %+v", fg)
	}
}

func TestALineBecomesOneSpanForEachRun(t *testing.T) {
	grid := herdr.Grid{
		Width:  5,
		Height: 1,
		Cells: []herdr.Cell{
			{Symbol: "a", FG: 0x02FF0000},
			{Symbol: "b", FG: 0x02FF0000},
			{Symbol: " ", FG: 0x02FF0000},
			{Symbol: "c", FG: 0x0200FF00},
			{Symbol: "d", FG: 0x0200FF00},
		},
	}
	spans, _ := rowSpans(grid, 0, 5, 13, defaultColors())
	if len(spans) != 2 {
		t.Fatalf("the line became %d spans: %+v", len(spans), spans)
	}
	if spans[0].Text != "ab" || spans[1].Text != "cd" {
		t.Fatalf("the spans came out as %q and %q", spans[0].Text, spans[1].Text)
	}
	if spans[0].Color != ui.RGB(255, 0, 0) || spans[1].Color != ui.RGB(0, 255, 0) {
		t.Fatalf("the colors came out as %+v and %+v", spans[0].Color, spans[1].Color)
	}
}

func TestCellsWithTheirOwnBackgroundAreMarked(t *testing.T) {
	grid := herdr.Grid{
		Width:  3,
		Height: 1,
		Cells: []herdr.Cell{
			{Symbol: "a"},
			{Symbol: "b", BG: 0x020000FF},
			{Symbol: "c"},
		},
	}
	_, backgrounds := rowSpans(grid, 0, 3, 13, defaultColors())
	if len(backgrounds) != 1 {
		t.Fatalf("the line has %d backgrounds: %+v", len(backgrounds), backgrounds)
	}
	if backgrounds[0].Column != 1 || backgrounds[0].Color != ui.RGB(0, 0, 255) {
		t.Fatalf("the background came out as %+v", backgrounds[0])
	}
}

func TestWideCellsAreSkipped(t *testing.T) {
	grid := herdr.Grid{
		Width:  3,
		Height: 1,
		Cells: []herdr.Cell{
			{Symbol: "漢"},
			{Symbol: "", Skip: true},
			{Symbol: "x"},
		},
	}
	spans, _ := rowSpans(grid, 0, 3, 13, defaultColors())
	if len(spans) != 1 {
		t.Fatalf("the line became %d spans: %+v", len(spans), spans)
	}
	// The cell that continues the wide symbol adds nothing.
	if spans[0].Text != "漢x" {
		t.Fatalf("the span came out as %q", spans[0].Text)
	}
}

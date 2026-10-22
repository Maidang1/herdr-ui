package ui

import (
	"testing"

	"herdr-ui/internal/herdr"
)

func TestTheTextOfTheGridIsOneStringPerRow(t *testing.T) {
	grid := herdr.Grid{
		Width:  5,
		Height: 2,
		Cells: []herdr.Cell{
			{Symbol: "h"}, {Symbol: "i"}, {Symbol: " "}, {Symbol: "y"}, {Symbol: "o"},
			{Symbol: "!"}, {Symbol: " "}, {Symbol: " "}, {Symbol: " "}, {Symbol: " "},
		},
	}
	rows := gridText(grid)
	if len(rows) != 2 {
		t.Fatalf("the grid has %d rows", len(rows))
	}
	if rows[0] != "hi yo" {
		t.Fatalf("the first row came out as %q", rows[0])
	}
}

func TestTheWideCellsAreLeftOut(t *testing.T) {
	grid := herdr.Grid{
		Width:  3,
		Height: 1,
		Cells: []herdr.Cell{
			{Symbol: "漢"},
			{Symbol: "", Skip: true},
			{Symbol: "x"},
		},
	}
	rows := gridText(grid)
	if len(rows) != 1 || rows[0] != "漢x" {
		t.Fatalf("the row came out as %+v", rows)
	}
}

func TestTheFindPicksOutWhereTheTextStands(t *testing.T) {
	rows := []string{"the quick brown", "fox", "the end"}
	matches := findText(rows, "the")
	if len(matches) != 2 {
		t.Fatalf("the find kept %+v", matches)
	}
	if matches[0].Start.Row != 0 || matches[0].Start.Col != 0 {
		t.Fatalf("the first match came out as %+v", matches[0])
	}
	if matches[1].Start.Row != 2 || matches[1].End.Col != 2 {
		t.Fatalf("the second match came out as %+v", matches[1])
	}
}

func TestTheFindIgnoresTheCase(t *testing.T) {
	matches := findText([]string{"THE End"}, "the")
	if len(matches) != 1 {
		t.Fatalf("the find kept %+v", matches)
	}
	if matches[0].Start.Col != 0 || matches[0].End.Col != 2 {
		t.Fatalf("the match came out as %+v", matches[0])
	}
}

func TestTheFindFindsNothingWhenThereIsNothing(t *testing.T) {
	if matches := findText([]string{"the end"}, "zzz"); len(matches) != 0 {
		t.Fatalf("the find kept %+v", matches)
	}
	if matches := findText([]string{"the end"}, ""); len(matches) != 0 {
		t.Fatalf("the find kept %+v", matches)
	}
}

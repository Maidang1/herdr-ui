package ui

import (
	"strings"
	"testing"

	"herdr-ui/internal/herdr"
)

// selectionFrame builds a frame with one pane, so that a selection has
// something to choose.
func selectionFrame(rows []string) *herdr.SurfaceFrame {
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	grid := herdr.Grid{Width: uint16(width), Height: uint16(len(rows))}
	for _, row := range rows {
		for c := 0; c < width; c++ {
			symbol := " "
			if c < len(row) {
				symbol = string(row[c])
			}
			grid.Cells = append(grid.Cells, herdr.Cell{Symbol: symbol})
		}
	}
	return &herdr.SurfaceFrame{
		Grid: grid,
		Panes: []herdr.PaneSurface{{
			PaneID:    "w1:p1",
			Rect:      herdr.SurfaceRect{X: 0, Y: 0, Width: uint16(width), Height: uint16(len(rows))},
			InnerRect: herdr.SurfaceRect{X: 0, Y: 0, Width: uint16(width), Height: uint16(len(rows))},
		}},
	}
}

func TestSelectionTakesTheCellsItCovers(t *testing.T) {
	frame := selectionFrame([]string{"hello world", "second line"})
	pane := frame.Panes[0]

	// One row, from the fifth column to the last.
	text := selectionText(frame.Grid, pane, gridPoint{Row: 0, Col: 4}, gridPoint{Row: 0, Col: 10})
	if text != "o world" {
		t.Fatalf("the selection came out as %q", text)
	}

	// Two rows: the first ends where the drag began, the second ends
	// where the drag ended.
	text = selectionText(frame.Grid, pane, gridPoint{Row: 0, Col: 6}, gridPoint{Row: 1, Col: 5})
	if text != "world\nsecond" {
		t.Fatalf("the selection came out as %q", text)
	}

	// The selection runs backwards as well.
	text = selectionText(frame.Grid, pane, gridPoint{Row: 1, Col: 5}, gridPoint{Row: 0, Col: 6})
	if text != "world\nsecond" {
		t.Fatalf("the selection came out as %q", text)
	}
}

func TestSelectionDropsTheBlanksItCovers(t *testing.T) {
	frame := selectionFrame([]string{"a  ", "   "})
	pane := frame.Panes[0]
	text := selectionText(frame.Grid, pane, gridPoint{Row: 0, Col: 0}, gridPoint{Row: 1, Col: 2})
	if text != "a" {
		t.Fatalf("the selection came out as %q", text)
	}
}

func TestSelectionStaysInsideThePane(t *testing.T) {
	frame := selectionFrame([]string{"abc", "def"})
	// A pane that holds only the second column of the grid.
	frame.Panes[0].InnerRect = herdr.SurfaceRect{X: 1, Y: 0, Width: 2, Height: 2}
	pane := frame.Panes[0]
	text := selectionText(frame.Grid, pane, gridPoint{Row: 0, Col: 0}, gridPoint{Row: 1, Col: 2})
	if text != "bc\nef" {
		t.Fatalf("the selection came out as %q", text)
	}
}

func TestTheDragChoosesCellsAndKeepsThem(t *testing.T) {
	app := &App{}
	app.surface = selectionFrame([]string{"hello", "world"})

	app.beginSelection("w1:p1", gridPoint{Row: 0, Col: 0})
	selection := app.selectionOf()
	if !selection.dragging || selection.paneID != "w1:p1" {
		t.Fatalf("the selection came out as %+v", selection)
	}

	app.extendSelection(gridPoint{Row: 1, Col: 4})
	selection = app.selectionOf()
	if selection.cursor != (gridPoint{Row: 1, Col: 4}) {
		t.Fatalf("the cursor came out as %+v", selection.cursor)
	}

	text := app.endSelection(gridPoint{Row: 1, Col: 4})
	if text != "hello\nworld" {
		t.Fatalf("the selection read %q", text)
	}
	selection = app.selectionOf()
	if selection.dragging {
		t.Fatalf("the selection is still dragging: %+v", selection)
	}
	if !selection.retained {
		t.Fatalf("the selection was not kept: %+v", selection)
	}
	if app.selectedText() != "hello\nworld" {
		t.Fatalf("the window holds %q", app.selectedText())
	}
}

func TestAClickChoosesNothing(t *testing.T) {
	app := &App{}
	app.surface = selectionFrame([]string{"hello"})

	app.beginSelection("w1:p1", gridPoint{Row: 0, Col: 2})
	text := app.endSelection(gridPoint{Row: 0, Col: 2})
	if text != "" {
		t.Fatalf("the selection read %q", text)
	}
	selection := app.selectionOf()
	if selection.retained {
		t.Fatalf("the selection was kept: %+v", selection)
	}
}

func TestAPressOutsideThePaneClearsTheSelection(t *testing.T) {
	app := &App{}
	app.surface = selectionFrame([]string{"hello"})

	app.beginSelection("w1:p1", gridPoint{Row: 0, Col: 0})
	app.beginSelection("w1:p1", gridPoint{Row: 4, Col: 0})
	selection := app.selectionOf()
	if selection.dragging || selection.retained {
		t.Fatalf("the selection came out as %+v", selection)
	}
}

func TestTheCellUnderThePointerIsFound(t *testing.T) {
	frame := selectionFrame([]string{"hello", "world"})
	point, ok := cellAt(frame, 3*7.5, 1*18, 7.5, 18)
	if !ok {
		t.Fatalf("the cell was not found")
	}
	if point.Row != 1 || point.Col != 3 {
		t.Fatalf("the cell came out as %+v", point)
	}
	if _, ok := cellAt(frame, 3*7.5, 9*18, 7.5, 18); ok {
		t.Fatalf("a cell below the grid was found")
	}
	if _, ok := cellAt(frame, 30*7.5, 0, 7.5, 18); ok {
		t.Fatalf("a cell beside the grid was found")
	}
}

func TestTheSelectionOfTwoLinesKeepsTheLineBreak(t *testing.T) {
	app := &App{}
	app.surface = selectionFrame([]string{"one", "two"})
	app.beginSelection("w1:p1", gridPoint{Row: 0, Col: 0})
	text := app.endSelection(gridPoint{Row: 1, Col: 2})
	if !strings.Contains(text, "\n") {
		t.Fatalf("the selection read %q", text)
	}
}

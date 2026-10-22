package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// Column is a column of a table.
type Column struct {
	// Title is the name of the column.
	Title string
	// Width is the width of the column. A column of no width takes the
	// room that is left.
	Width float32
	// Mono writes the column in the family of the terminal.
	Mono bool
}

// Table is a list of rows of columns.
//
// The design file has no table of its own yet, so this is the plainest one
// that the panels can share: a head of columns, and rows under it, with a
// hairline between them.
func Table(c *ui.Context, t design.Tokens, columns []Column, rows [][]string) ui.Element {
	return ui.Column(c).
		Children(func() {
			ui.Row(c).
				Gap(design.SpaceLG).
				Padding(0, design.SpaceMD, design.SpaceSM, design.SpaceMD).
				Children(func() {
					for _, column := range columns {
						element := Label(c, t, column.Title, LabelOptions{
							Size:   design.FontBody,
							Color:  t.TextTertiary,
							Weight: design.WeightMedium,
						})
						if column.Width > 0 {
							element.Width(column.Width)
						}
					}
				})
			for _, row := range rows {
				ui.Row(c).
					Gap(design.SpaceLG).
					Padding(design.SpaceSM, design.SpaceMD).
					Border(0, t.Separator).
					Children(func() {
						for i, column := range columns {
							if i >= len(row) {
								continue
							}
							element := Label(c, t, row[i], LabelOptions{
								Size:  design.FontUI,
								Color: t.TextPrimary,
								Mono:  column.Mono,
							})
							if column.Width > 0 {
								element.Width(column.Width)
							}
						}
					})
			}
		})
}

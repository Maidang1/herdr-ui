package ui

import (
	"fmt"
	"testing"

	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// TestTheTerminalCellsArePainted draws a few cells the way the terminal
// draws them, and looks at the colors that come out.
//
// It is here because a color that is right on paper and wrong on the
// screen is a color nobody sees.
func TestTheTerminalCellsArePainted(t *testing.T) {
	tokens := design.Dark()
	foreground, background := tokens.TerminalCellColors(0x02D79921, 0, 0)

	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).
			Fill().
			Background(background).
			Children(func() {
				ui.Row(c).
					Background(background).
					Children(func() {
						ui.Text(c, "abc").
							Font(design.FamilyMono).
							FontSize(13).
							TextColor(foreground)
					})
			})
	}, 200, 40)
	tester.Frame()

	img := tester.Image()
	counts := map[string]int{}
	for y := 0; y < 40; y++ {
		for x := 0; x < 200; x++ {
			c := img.RGBAAt(x, y)
			counts[fmt.Sprintf("%v", c)]++
		}
	}
	for color, count := range counts {
		if count > 10 {
			fmt.Printf("color %v appears %d times\n", color, count)
		}
	}
}

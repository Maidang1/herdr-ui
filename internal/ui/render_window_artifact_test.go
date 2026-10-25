package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
)

// TestRenderWindowArtifacts exports the main window over the material and
// without it, with and without a menu open, in both appearances, for
// review of its surfaces.
func TestRenderWindowArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export window renders")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		for _, material := range []bool{false, true} {
			for _, menu := range []bool{false, true} {
				w := populatedChromeWindow(t)
				w.tabMenuOpen = menu
				tester := native.NewTester(w.View, 1000, 667)
				tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
				tester.SetDark(dark)
				tester.SetVibrancy(material)
				tester.Frame()
				tester.Frame()
				f, err := os.Create(filepath.Join(dir, fmt.Sprintf("window-dark-%t-material-%t-menu-%t.png", dark, material, menu)))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tester.Image())
				f.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

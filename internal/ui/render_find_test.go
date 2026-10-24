package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

func TestRenderFindFixture(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export native chrome renders")
	}
	for _, dark := range []bool{false, true} {
		w := findWindow(t)
		tester := native.NewTester(w.View, 1200, 800)
		tester.SetDark(dark)
		tester.Frame()
		w.Dispatch("find")
		tester.Frame()
		tester.Type("error")
		w.SetSearchStatus(w.find.binding, terminal.SearchStatus{Text: "error", Total: 3, Selected: 1})
		tester.Frame()
		name := "find-bar-light.png"
		if dark {
			name = "find-bar-dark.png"
		}
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tester.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}

package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

func TestRenderSettingsArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export settings renders")
	}
	for _, dark := range []bool{false, true} {
		w := referenceWindow(t)
		w.SetInitialSettings(SettingsSnapshot{Theme: "dark", Font: terminal.Font{Size: 13, Weight: 400, LineHeight: 1}, MinimumContrast: 1, Cursor: "block", CursorBlink: true, CopyOnSelect: true, Toast: ToastSettings{Delivery: "herdr", Position: "bottom-right", DelaySeconds: 1}})
		for _, page := range settingsPages {
			for _, size := range [][2]int{{720, 520}, {1000, 1160}} {
				w.settings.page = indexOfPage(page)
				tester := native.NewTester(w.SettingsView, size[0], size[1])
				tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
				tester.SetDark(dark)
				tester.Frame()
				name := fmt.Sprintf("settings-%s-%dx%d-dark-%t.png", page, size[0], size[1], dark)
				file, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(file, tester.Image())
				file.Close()
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	_ = design.Colors
}

func indexOfPage(name string) int {
	for i, p := range settingsPages {
		if p == name {
			return i
		}
	}
	return 0
}

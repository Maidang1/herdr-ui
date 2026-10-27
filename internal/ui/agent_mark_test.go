package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	native "github.com/egoist/mygo/ui"
)

func TestAgentMarksKeepBrandColor(t *testing.T) {
	dir := t.TempDir()
	if extra := os.Getenv("HERDR_UI_RENDER_DIR"); extra != "" {
		dir = extra
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name string
		dark bool
		want color.RGBA
	}{
		{"claude", false, color.RGBA{R: 217, G: 119, B: 87, A: 255}},
		{"codex", false, color.RGBA{R: 57, G: 65, B: 255, A: 255}},
		{"codex", true, color.RGBA{R: 122, G: 157, B: 255, A: 255}},
		{"grok", false, color.RGBA{R: 0, G: 0, B: 0, A: 255}},
		{"cursor", true, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"copilot", false, color.RGBA{R: 16, G: 157, B: 242, A: 255}},
		{"gemini", false, color.RGBA{R: 49, G: 134, B: 255, A: 255}},
		{"opencode", false, color.RGBA{R: 19, G: 16, B: 16, A: 255}},
		{"kimi", false, color.RGBA{R: 23, G: 131, B: 255, A: 255}},
		{"kimi", true, color.RGBA{R: 255, G: 255, B: 255, A: 255}},
		{"shell", false, color.RGBA{R: 136, G: 136, B: 136, A: 255}},
	}
	for _, tc := range cases {
		img := renderMark(t, tc.name, tc.dark)
		name := tc.name + "-light.png"
		if tc.dark {
			name = tc.name + "-dark.png"
		}
		writePNG(t, filepath.Join(dir, name), img)
		if !hasPixelNear(img, tc.want, 36) {
			t.Errorf("%s dark=%v has no pixel near #%02x%02x%02x", tc.name, tc.dark, tc.want.R, tc.want.G, tc.want.B)
		}
	}
	t.Log(dir)
}

func renderMark(t *testing.T, name string, dark bool) image.Image {
	t.Helper()
	bg := native.Hex("#FFFFFF")
	if dark {
		bg = native.Hex("#1E1E20")
	}
	tint := native.Hex("#888888")
	tester := native.NewTester(func(c *native.Context) {
		native.Box(c).Fill().Background(bg).Center().Children(func() {
			agentMark(c, name, 32, tint)
		})
	}, 48, 48)
	tester.SetDark(dark)
	tester.Frame()
	return tester.Image()
}

func hasPixelNear(img image.Image, want color.RGBA, tol int) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if c.A < 200 {
				continue
			}
			if abs(int(c.R)-int(want.R)) <= tol && abs(int(c.G)-int(want.G)) <= tol && abs(int(c.B)-int(want.B)) <= tol {
				return true
			}
		}
	}
	return false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

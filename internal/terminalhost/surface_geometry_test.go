package terminalhost

import (
	"math"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/Maidang1/herdr-ui/internal/surfaceview"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func stackedSurfacePanes(t *testing.T, count, sourceRows int) []layout.PaneRect {
	t.Helper()
	panes := make([]layout.PaneRect, count)
	y := 0
	for i := range panes {
		end := (i + 1) * sourceRows / count
		panes[i] = layout.PaneRect{PaneID: string(rune('a' + i)), Rect: layout.Rect{Y: float64(y), W: 20, H: float64(end - y)}}
		y = end
	}
	projected, err := layout.Project(layout.Rect{W: 20, H: float64(sourceRows)}, panes, nil, layout.Rect{W: 400, H: 600})
	if err != nil {
		t.Fatal(err)
	}
	return projected.Panes
}
func TestSurfaceGeometryFitsEveryNativeBody(t *testing.T) {
	for _, tc := range []struct {
		name               string
		panes              []layout.PaneRect
		header             float64
		wantCols, wantRows uint16
	}{
		{"single", []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 600}}}, 0, 56, 28},
		{"two-stacked", []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 300}}, {Rect: layout.Rect{Y: 300, W: 600, H: 300}}}, 32, 56, 24},
		{"four-stacked", []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 150}}, {Rect: layout.Rect{Y: 150, W: 600, H: 150}}, {Rect: layout.Rect{Y: 300, W: 600, H: 150}}, {Rect: layout.Rect{Y: 450, W: 600, H: 150}}}, 32, 56, 16},
		{"unequal", []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 200}}, {Rect: layout.Rect{Y: 200, W: 600, H: 400}}}, 32, 56, 21},
		{"side-by-side", []layout.PaneRect{{Rect: layout.Rect{W: 300, H: 600}}, {Rect: layout.Rect{X: 300, W: 300, H: 600}}}, 32, 56, 28},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cols, rows, valid := surfaceLayoutSize(600, 600, 10, 20, tc.panes, tc.header)
			if !valid || cols != tc.wantCols || rows != tc.wantRows {
				t.Fatalf("canvas %dx%d valid=%v, want %dx%d", cols, rows, valid, tc.wantCols, tc.wantRows)
			}
			for _, pane := range tc.panes {
				r := pane.Rect
				if math.Ceil(float64(cols)*r.W/600)*10 > r.W || math.Ceil(float64(rows)*r.H/600)*20 > r.H-tc.header {
					t.Fatalf("canvas exceeds pane body %+v", r)
				}
			}
		})
	}
}
func TestSurfaceGeometryRejectsIncompleteLayoutWithoutResizing(t *testing.T) {
	h := &Host{surface: &surfaceBackend{wake: make(chan struct{}, 1)}}
	h.ConfigureSurface("tab", "pane", 600, 600, true)
	before := h.surface.selection()
	if before.cols <= 1 || before.rows <= 1 {
		t.Fatal("valid canvas was not configured")
	}
	for _, tc := range []struct {
		width, height float32
		panes         []layout.PaneRect
		header        float32
	}{
		{0, 0, nil, 0}, {33, 25, nil, 0}, {600, 600, []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 32}}}, 32},
		{600, 600, []layout.PaneRect{{Rect: layout.Rect{W: 600, H: 33}}}, 32},
		{600, 600, []layout.PaneRect{{Rect: layout.Rect{W: 601, H: 600}}}, 0},
		{float32(math.NaN()), 600, nil, 0},
	} {
		h.ConfigureSurfaceLayout("other", "other", tc.width, tc.height, false, tc.panes, tc.header)
		if got := h.surface.selection(); got != before {
			t.Fatalf("incomplete layout replaced stable canvas: %+v", got)
		}
	}
	h.ConfigureSurface("tab", "pane", 600, 600, true)
	if got := h.surface.selection(); got != before {
		t.Fatal("identical geometry changed selection generation")
	}
}
func TestSurfaceGeometryRoundingAndRepeatedConfigurationStayStable(t *testing.T) {
	font := terminal.Font{Size: 13}
	cw, ch := surfaceview.CellMetrics(font)
	sourceRows := int((600 - 24) / ch)
	panes := stackedSurfacePanes(t, 2, sourceRows)
	h := &Host{cfg: Config{Font: font}, surface: &surfaceBackend{wake: make(chan struct{}, 1)}}
	h.ConfigureSurfaceLayout("tab", "pane", 400, 600, true, panes, 32)
	before := h.surface.selection()
	if before.cols <= 1 || before.rows <= 1 {
		t.Fatal("split layout collapsed")
	}
	for range 20 {
		h.ConfigureSurfaceLayout("tab", "pane", 400, 600, true, panes, 32)
		if h.surface.selection() != before {
			t.Fatal("unchanged projected layout oscillated canvas size")
		}
	}
	for _, p := range panes {
		if math.Ceil(float64(before.rows)*p.Rect.H/600)*float64(ch) > p.Rect.H-32 || math.Ceil(float64(before.cols)*p.Rect.W/400)*float64(cw) > p.Rect.W {
			t.Fatal("rounded cells exceed native body")
		}
	}
}

func geometryCursorVisible(t *testing.T, font terminal.Font, frameRows, innerRows int, cardHeight float64) bool {
	t.Helper()
	cells := make([]endpoint.SurfaceCell, 20*frameRows)
	for i := range cells {
		cells[i].Symbol = " "
	}
	frame := endpoint.SurfaceFrame{BootID: "geometry", Frame: endpoint.SurfaceGrid{Width: 20, Height: uint16(frameRows), Cells: cells, Cursor: &endpoint.SurfaceCursor{X: 0, Y: uint16(innerRows - 1), Visible: true, Shape: 4}}, Panes: []endpoint.SurfacePane{{PaneID: "pane", Focused: true, Rect: endpoint.SurfaceRect{Width: 20, Height: uint16(innerRows)}, InnerRect: endpoint.SurfaceRect{Width: 20, Height: uint16(innerRows)}}}}
	theme := *terminal.LightTheme()
	theme.Background = ui.RGB(0, 0, 0)
	theme.Cursor = ui.RGB(0, 255, 0)
	view := surfaceview.New(surfaceview.Config{Font: font, Theme: &theme})
	view.SetFrame(frame, "pane")
	// Native paneCard uses this same fixed, nonshrinking header and Fill body.
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Clip().Children(func() { ui.Box(c).Height(32).Shrink(0).FillWidth(); view.View(c).Fill() })
	}, 400, int(cardHeight))
	img := tester.Image()
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			p := img.RGBAAt(x, y)
			if p.G > 200 && p.R < 20 && p.B < 20 {
				return true
			}
		}
	}
	return false
}
func TestSurfaceGeometryKeepsBottomCursorVisibleAcrossNativeHeaders(t *testing.T) {
	// These are the two reproduced failures: disabled server borders, and
	// four stacked default-bordered panes at a supported smaller font size.
	for _, tc := range []struct {
		name           string
		font           float32
		count, borders int
	}{{"borderless-two", 13, 2, 0}, {"default-bordered-four-small-font", 9, 4, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			font := terminal.Font{Size: tc.font}
			cw, ch := surfaceview.CellMetrics(font)
			oldRows := int((600 - 24) / ch)
			panes := stackedSurfacePanes(t, tc.count, oldRows)
			largest := panes[len(panes)-1].Rect.H
			for _, p := range panes {
				largest = max(largest, p.Rect.H)
			}
			oldPaneRows := (oldRows + tc.count - 1) / tc.count
			if geometryCursorVisible(t, font, oldRows, oldPaneRows-tc.borders, largest) {
				t.Fatal("fixture did not reproduce original clipped bottom cursor")
			}
			_, rows, valid := surfaceLayoutSize(400, 600, float64(cw), float64(ch), panes, 32)
			if !valid || rows <= 1 {
				t.Fatal("valid stacked canvas collapsed")
			}
			newPaneRows := (int(rows) + tc.count - 1) / tc.count
			for _, p := range panes {
				if !geometryCursorVisible(t, font, int(rows), newPaneRows-tc.borders, p.Rect.H) {
					t.Fatalf("bottom cursor remains clipped with %d rows and native body %.3f", rows, p.Rect.H-32)
				}
			}
		})
	}
}

func projectedBSPPanes(rows int, width, height, ratio float64) []layout.PaneRect {
	// Herdr layout.rs::split_rect rounds the float32 multiplication; it does
	// not floor a double-precision projected coordinate.
	first := int(math.Round(float64(float32(rows) * float32(ratio))))
	edge := height * float64(first) / float64(rows)
	return []layout.PaneRect{{PaneID: "first", Rect: layout.Rect{W: width, H: edge}}, {PaneID: "second", Rect: layout.Rect{Y: edge, W: width, H: height - edge}}}
}
func TestSurfaceGeometryDaemonFeedbackConverges(t *testing.T) {
	cases := []struct {
		name              string
		height, ch, ratio float64
	}{
		{"four-state-cycle", 403.9376223654524, 21.138100229979187, 0.46060845255851746},
		{"small-viewport-cycle", 165.33079396325078, 10.09359590935727, 0.5705482959747314},
		{"three-state-cycle", 257.4364489944503, 9.595016796775194, 0.5295149683952332},
		{"one-cell-cycle", 893.3669778609956, 8.795377537872596, 0.48398447036743164},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			initial := int((tc.height - 24) / tc.ch)
			source := initial
			seen := map[int]bool{}
			cycled := false
			for range 20 {
				panes := projectedBSPPanes(source, 600, tc.height, tc.ratio)
				_, rows, valid := surfaceLayoutSize(600, tc.height, 10, tc.ch, panes, 32)
				if !valid {
					t.Fatal("feedback fixture collapsed")
				}
				next := int(rows)
				if next == source {
					break
				}
				if seen[next] {
					cycled = true
					break
				}
				seen[source] = true
				source = next
			}
			if !cycled {
				t.Fatal("fixture no longer reproduces naive resize oscillation")
			}
			var state surfaceGeometryState
			source = initial
			last := 0
			splits := []layout.SplitRect{{ID: "root", Direction: "down", Ratio: tc.ratio}}
			for step := range 20 {
				panes := projectedBSPPanes(source, 600, tc.height, tc.ratio)
				cols, rows, valid := state.fit(600, tc.height, 10, tc.ch, panes, 32, splits)
				if !valid {
					t.Fatal("stable feedback collapsed")
				}
				if last > 0 && int(rows) > last {
					t.Fatalf("feedback expanded at step %d: %d -> %d", step, last, rows)
				}
				first := int(math.Round(float64(float32(rows) * float32(tc.ratio))))
				actual := []int{first, int(rows) - first}
				for i, p := range panes {
					if math.Ceil(float64(rows)*p.Rect.H/tc.height)*tc.ch > p.Rect.H-32+1e-6 || float64(actual[i])*tc.ch > p.Rect.H-32+1e-6 || math.Ceil(float64(cols)*p.Rect.W/600)*10 > p.Rect.W+1e-6 {
						t.Fatalf("step %d clipped projected body %+v with %d rows", step, p.Rect, rows)
					}
				}
				if step >= 10 && int(rows) != last {
					t.Fatalf("feedback still shrinking after ten reprojections: %d -> %d", last, rows)
				}
				last, source = int(rows), int(rows)
			}
		})
	}
}
func TestSurfaceGeometryAllowsExpansionAfterRealLayoutChanges(t *testing.T) {
	for _, change := range []string{"viewport", "font", "ratio"} {
		t.Run(change, func(t *testing.T) {
			var state surfaceGeometryState
			unequal := []layout.PaneRect{{PaneID: "first", Rect: layout.Rect{W: 600, H: 200}}, {PaneID: "second", Rect: layout.Rect{Y: 200, W: 600, H: 400}}}
			splits := []layout.SplitRect{{ID: "root", Direction: "down", Ratio: 1.0 / 3}}
			_, before, ok := state.fit(600, 600, 10, 20, unequal, 32, splits)
			if !ok {
				t.Fatal("initial layout failed")
			}
			width, height, ch, ratio := 600.0, 600.0, 20.0, 1.0/3
			switch change {
			case "viewport":
				height = 800
			case "font":
				ch = 15
			case "ratio":
				ratio = .5
			}
			panes := projectedBSPPanes(60, width, height, ratio)
			splits[0].Ratio = ratio
			_, after, ok := state.fit(width, height, 10, ch, panes, 32, splits)
			if !ok || after <= before {
				t.Fatalf("%s failed to release prior bound: %d -> %d,valid=%v", change, before, after, ok)
			}
		})
	}
}

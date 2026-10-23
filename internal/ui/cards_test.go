package ui

import (
	"fmt"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/layout"
	native "github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitCardsInsetOnlySharedEdges(t *testing.T) {
	viewport := layout.Rect{W: 1000, H: 600}
	left := cardBounds(layout.Rect{W: 500, H: 600}, viewport)
	top := cardBounds(layout.Rect{X: 500, W: 500, H: 300}, viewport)
	bottom := cardBounds(layout.Rect{X: 500, Y: 300, W: 500, H: 300}, viewport)
	if left.X != 0 || left.Y != 0 || left.H != 600 || top.X-left.X-left.W != 8 || bottom.Y-top.Y-top.H != 8 || bottom.X+bottom.W != 1000 || bottom.Y+bottom.H != 600 {
		t.Fatal("card edges/gaps drifted", left, top, bottom)
	}
	if single := cardBounds(viewport, viewport); single != viewport {
		t.Fatal("outer card gained empty margins", single)
	}
}
func TestCardHeaderOmitsSyntheticTitlesAndUsesRealAgentBadge(t *testing.T) {
	w := New(app.New(), nil)
	for _, pane := range []app.PaneRow{{ID: "w13:p8", Title: "w13:p8"}, {ID: "agent", Title: "user@HOST:/repo", Agent: "codex"}} {
		tester := native.NewTester(func(c *native.Context) {
			w.paneCard(c, layout.Rect{W: 300, H: 200}, pane, app.View{}, false, true, design.Colors(false), func() { native.Box(c).Grow(1).FillWidth().Label("Terminal viewport") })
		}, 300, 200)
		if tester.HasText(pane.Title) {
			t.Fatal("synthetic/title metadata returned", pane.Title)
		}
		if pane.Agent != "" && !tester.HasText(pane.Agent) {
			t.Fatal("real agent badge missing")
		}
		if pane.Agent == "" && !tester.HasText("Terminal") {
			t.Fatal("ordinary terminal has no generic caption")
		}
		body, ok := tester.Find("Terminal viewport")
		if !ok || body.H < 160 {
			t.Fatal("card chrome consumed terminal space", body)
		}
	}
}

func TestCardContentAccountsForHeaderAndBorders(t *testing.T) {
	if hasCardContent(layout.Rect{W: 34, H: 80}, true) || hasCardContent(layout.Rect{W: 100, H: 58}, true) {
		t.Fatal("card allows zero terminal content")
	}
	if !hasCardContent(layout.Rect{W: 35, H: 59}, true) {
		t.Fatal("card hides positive terminal content")
	}
	if !hasCardContent(layout.Rect{W: 33, H: 57}, false) {
		t.Fatal("single terminal has no positive content after controls")
	}
}
func TestSplitCardRenderArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set render directory")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		w := New(app.New(), nil)
		tester := native.NewTester(func(c *native.Context) {
			p := w.appearancePalette(c)
			root := native.Column(c).Fill().Padding(8)
			w.styleShell(c, root, p)
			root.Children(func() {
				area := native.Box(c).Grow(1).FillWidth()
				area.Children(func() {
					b := area.Bounds()
					viewport := layout.Rect{W: float64(b.W), H: float64(b.H)}
					panes := []app.PaneRow{{ID: "left-top", Title: "w13:p8"}, {ID: "left-bottom", Agent: "claude"}, {ID: "right", Agent: "codex"}}
					bounds := []layout.Rect{{W: viewport.W * .45, H: viewport.H * .55}, {Y: viewport.H * .55, W: viewport.W * .45, H: viewport.H * .45}, {X: viewport.W * .45, W: viewport.W * .55, H: viewport.H}}
					for i, pane := range panes {
						w.paneCard(c, cardBounds(bounds[i], viewport), pane, app.View{}, i == 2, true, p, func() { native.Box(c).Grow(1).FillWidth() })
					}
				})
			})
		}, 1000, 700)
		tester.SetDark(dark)
		tester.Frame()
		file, err := os.Create(filepath.Join(dir, fmt.Sprintf("split-cards-dark-%t.png", dark)))
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

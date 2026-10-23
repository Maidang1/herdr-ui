package ui

import (
	"fmt"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/input"
	"github.com/bytedance/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func referenceWindow(t *testing.T) *Window {
	t.Helper()
	w := New(app.New(), nil)
	bindings, err := input.Resolve(nil, "client test config")
	if err != nil {
		t.Fatal(err)
	}
	w.ConfigureBindings(bindings, nil)
	return w
}
func TestReferencePanelIsRightHand344AndPreservesSidebarPreference(t *testing.T) {
	w := referenceWindow(t)
	w.Dispatch("help")
	tester := native.NewTester(w.View, 820, 560)
	panel, ok := tester.Find("Keyboard reference panel")
	if !ok {
		t.Fatal("missing nonmodal right reference panel")
	}
	if panel.W != 344 || panel.X+panel.W > 813 {
		t.Fatalf("reference bounds=%+v", panel)
	}
	terminal, ok := tester.Find("Terminal content")
	if !ok || terminal.W < 400 {
		t.Fatalf("terminal width=%+v present=%v", terminal, ok)
	}
	if w.hidden {
		t.Fatal("temporary compact sidebar overwrote manual preference")
	}
	if _, ok := tester.Find("Workspace sidebar"); ok {
		t.Fatal("sidebar should temporarily collapse")
	}
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if _, ok := tester.Find("Workspace sidebar"); !ok {
		t.Fatal("closing reference failed to restore sidebar")
	}
}

func TestReferenceDoesNotCloseOnTerminalEscape(t *testing.T) {
	w := referenceWindow(t)
	w.Dispatch("help")
	terminalEscape := false
	tester := native.NewTester(func(c *native.Context) {
		native.Row(c).Fill().Children(func() {
			term := native.Button(c, "Terminal focus").Grow(1)
			if term.Shortcut(0, native.KeyEscape) {
				terminalEscape = true
			}
			if w.referenceOpen() {
				w.referencePanel(c, w.appearancePalette(c))
			}
		})
	}, 820, 560)
	if !tester.Focused("Close") {
		t.Fatal("reference close did not get focus on open")
	}
	if err := tester.Click("Terminal focus"); err != nil {
		t.Fatal(err)
	}
	tester.Key(0, native.KeyEscape)
	if !w.referenceOpen() || !terminalEscape {
		t.Fatal("reference intercepted terminal Escape")
	}
	// Reopening focuses the panel once; Escape inside the panel closes it.
	w.Dispatch("help")
	w.Dispatch("help")
	tester.Frame()
	tester.Key(0, native.KeyEscape)
	if w.referenceOpen() {
		t.Fatal("panel Escape did not close reference")
	}
}
func TestSidebarDragClampsWidthAndReferenceResizeRestoresIt(t *testing.T) {
	w := referenceWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	handle, ok := tester.Find("Resize sidebar")
	if !ok {
		t.Fatal("missing resize handle")
	}
	if handle.W < 8 {
		t.Fatal("resize handle too small", handle)
	}
	x, y := handle.X+handle.W/2, handle.Y+50
	tester.Press(x, y)
	tester.Move(x+150, y)
	tester.Release(x+150, y)
	if w.sidebarWidth != 320 {
		t.Fatal("sidebar maximum", w.sidebarWidth)
	}
	handle, _ = tester.Find("Resize sidebar")
	x = handle.X + handle.W/2
	tester.Press(x, y)
	tester.Move(x-200, y)
	tester.Release(x-200, y)
	if w.sidebarWidth != 220 {
		t.Fatal("sidebar minimum", w.sidebarWidth)
	}
	w.Dispatch("help")
	tester.SetSize(820, 560)
	tester.Frame()
	if w.sidebarVisible || w.hidden {
		t.Fatal("compact reference changed sidebar preference")
	}
	tester.SetSize(1200, 800)
	tester.Frame()
	if !w.sidebarVisible || w.sidebarWidth != 220 {
		t.Fatal("expanding failed to restore sidebar width")
	}
}
func TestChromeDimensionsRetainMinimumTerminalAndManualHidden(t *testing.T) {
	for _, width := range []float32{820, 900, 1200} {
		for _, sidebar := range []float32{220, 248, 320} {
			dimensions := chromeLayout(width, sidebar, false, true)
			if dimensions.terminalWidth < 400 {
				t.Fatal(width, sidebar, dimensions)
			}
		}
	}
	if chromeLayout(1200, 248, true, false).sidebarVisible {
		t.Fatal("manual hidden sidebar restored")
	}
}

func populatedChromeWindow(t *testing.T) *Window {
	t.Helper()
	w := referenceWindow(t)
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Very long workspace name for narrow headers","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t","number":1,"label":"Development with a deliberately long name"},{"workspace_id":"w","tab_id":"other","number":2,"label":"Server"}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","title":"Shell"},{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"other","title":"Other shell"}],"agents":[],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w.app.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: snapshot})
	return w
}
func TestCompactHeaderReservesTrafficLightsAndKeepsLayoutMenu(t *testing.T) {
	w := populatedChromeWindow(t)
	w.Dispatch("help")
	tester := native.NewTester(w.View, 820, 560)
	tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
	tester.Frame()
	content, _ := tester.Find("Terminal content")
	tabs, ok := tester.Find("Current tab")
	if !ok || tabs.W < 64 {
		t.Fatal("tab selector missing or squeezed", tabs)
	}
	layout, ok := tester.Find("Terminal layout")
	if !ok || layout.X+layout.W > content.X+content.W {
		t.Fatal("layout action clipped", layout, content)
	}
	if tabs.X < 80 {
		t.Fatal("tab selector overlaps traffic lights", tabs)
	}
	if err := tester.Click("Current tab"); err != nil {
		t.Fatal(err)
	}
	menu := tester.Menu()
	if len(menu) != 2 || !strings.Contains(menu[0], "1 panes") || !strings.Contains(menu[1], "Server") {
		t.Fatal("tab menu did not show real names/counts", menu)
	}
	if err := tester.ChooseMenuItem("2 · Server · 1 panes"); err != nil {
		t.Fatal(err)
	}
	if w.app.View().TabID != "other" {
		t.Fatal("native menu did not select tab")
	}
}
func TestShowTemporarilyCollapsedSidebarRestoresIt(t *testing.T) {
	w := referenceWindow(t)
	w.Dispatch("help")
	tester := native.NewTester(w.View, 820, 560)
	if w.ActionLabel("toggle_sidebar") != "Show sidebar" {
		t.Fatal("wrong dynamic sidebar menu label")
	}
	if err := tester.Click("Show sidebar"); err != nil {
		t.Fatal(err)
	}
	if !w.sidebarVisible || w.hidden || w.referenceOpen() {
		t.Fatal("show sidebar did not reveal sidebar")
	}
	if w.ActionLabel("toggle_sidebar") != "Hide sidebar" {
		t.Fatal("sidebar menu label not updated")
	}
}

func TestM4ChromeRenderArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export native chrome renders")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{820, 1200} {
		for _, dark := range []bool{false, true} {
			for _, reference := range []bool{false, true} {
				w := populatedChromeWindow(t)
				if reference {
					w.Dispatch("help")
				}
				height := 800
				if width == 820 {
					height = 560
				}
				tester := native.NewTester(w.View, width, height)
				tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
				tester.SetDark(dark)
				tester.Frame()
				name := fmt.Sprintf("m4-%dx%d-dark-%t-reference-%t.png", width, height, dark, reference)
				f, err := os.Create(filepath.Join(dir, name))
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

func TestShowManuallyHiddenSidebarRevealsItOnFirstClick(t *testing.T) {
	for _, width := range []int{820, 1200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			w := referenceWindow(t)
			w.hidden = true
			w.Dispatch("help")
			tester := native.NewTester(w.View, width, 560)
			if err := tester.Click("Show sidebar"); err != nil {
				t.Fatal(err)
			}
			if !w.sidebarVisible || w.hidden {
				t.Fatal("Show sidebar required a second click")
			}
			if w.referenceOpen() != (width == 1200) {
				t.Fatal("reference should close only when it prevents sidebar fitting")
			}
		})
	}
}

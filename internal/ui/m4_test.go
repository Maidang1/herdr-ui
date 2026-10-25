package ui

import (
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
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
func TestReferenceDialogPreservesLayoutAndDismisses(t *testing.T) {
	w := referenceWindow(t)
	tester := native.NewTester(w.View, 820, 560)
	before, _ := tester.Find("Terminal content")
	w.Dispatch("help")
	tester.Frame()
	panel, ok := tester.Find("Keyboard reference panel")
	if !ok || panel.X != (820-panel.W)/2 || panel.Y != (560-panel.H)/2 {
		t.Fatalf("dialog is not centered: %+v", panel)
	}
	after, _ := tester.Find("Terminal content")
	if before != after || !w.sidebarVisible || w.hidden {
		t.Fatal("dialog changed terminal/sidebar layout", before, after)
	}
	if !tester.Focused("Close") || !w.modalOpen() {
		t.Fatal("dialog did not take focus")
	}
	// Background sidebar control must not receive the click.
	if err := tester.Click("Hide sidebar"); err != nil {
		t.Fatal(err)
	}
	if w.hidden {
		t.Fatal("dialog allowed background interaction")
	}
	if !w.referenceOpen() {
		w.Dispatch("help")
		tester.Frame()
	}
	tester.Key(0, native.KeyEscape)
	if w.referenceOpen() {
		t.Fatal("Escape did not close dialog")
	}
	w.Dispatch("help")
	tester.Frame()
	if err := tester.Click("Close"); err != nil {
		t.Fatal(err)
	}
	if w.referenceOpen() {
		t.Fatal("close button did not close dialog")
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
	if !w.sidebarVisible || w.hidden {
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
			dimensions := chromeLayout(width, sidebar, false)
			if dimensions.terminalWidth < 400 {
				t.Fatal(width, sidebar, dimensions)
			}
		}
	}
	if chromeLayout(1200, 248, true).sidebarVisible {
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
func TestMinimalControlsReserveTrafficLightsWithoutInformationHeader(t *testing.T) {
	w := populatedChromeWindow(t)
	w.hidden = true
	tester := native.NewTester(w.View, 820, 560)
	tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
	tester.Frame()
	toggle, ok := tester.Find("Show sidebar")
	if !ok || toggle.X < 80 {
		t.Fatal("sidebar control overlaps traffic lights", toggle)
	}
	if _, ok := tester.Find("Toggle sidebar"); ok {
		t.Fatal("bottom tab bar rendered while the sidebar is hidden")
	}
	if _, ok := tester.Find("Tab Server"); !ok {
		t.Fatal("tab pills did not move into the title strip")
	}
	for _, label := range []string{"Current tab", "Terminal layout"} {
		if _, ok := tester.Find(label); ok {
			t.Fatal("removed information header visible", label)
		}
	}
	w.Dispatch("next_tab")
	tester.Frame()
	if w.app.View().TabID != "other" {
		t.Fatal("tab switching action no longer available")
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
			tester := native.NewTester(w.View, width, 560)
			if err := tester.Click("Show sidebar"); err != nil {
				t.Fatal(err)
			}
			if !w.sidebarVisible || w.hidden {
				t.Fatal("Show sidebar required a second click")
			}
		})
	}
}

func TestShortcutDialogPreservesLayoutAndConfirmation(t *testing.T) {
	for _, action := range []string{"Close Tab", "Cancel", "Escape", "Backdrop"} {
		t.Run(action, func(t *testing.T) {
			w := referenceWindow(t)
			tester := native.NewTester(w.View, 820, 560)
			before, _ := tester.Find("Terminal content")
			submits := 0
			w.controller.shortcut.prompt = &shortcutPrompt{title: "Close tab?", action: "Close Tab", destructive: true, confirm: true, focus: true, submit: func(string) { submits++ }}
			tester.Frame()
			panel, ok := tester.Find("Action confirmation dialog")
			if !ok || panel.X != (820-panel.W)/2 || panel.Y != (560-panel.H)/2 {
				t.Fatal("confirmation is not centered", panel)
			}
			after, _ := tester.Find("Terminal content")
			if before != after {
				t.Fatal("confirmation resized terminal", before, after)
			}
			switch action {
			case "Escape":
				tester.Key(0, native.KeyEscape)
			case "Backdrop":
				tester.Press(10, 550)
				tester.Release(10, 550)
			default:
				if err := tester.Click(action); err != nil {
					t.Fatal(err)
				}
			}
			if w.controller.shortcut.prompt != nil {
				t.Fatal("dialog stayed open")
			}
			want := 0
			if action == "Close Tab" {
				want = 1
			}
			if submits != want {
				t.Fatalf("submits=%d want=%d", submits, want)
			}
		})
	}
}

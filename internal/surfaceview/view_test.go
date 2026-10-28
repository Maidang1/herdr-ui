package surfaceview

import (
	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"testing"
)

func fixture() endpoint.SurfaceFrame {
	return endpoint.SurfaceFrame{BootID: "boot", Frame: endpoint.SurfaceGrid{Width: 6, Height: 2, Cells: []endpoint.SurfaceCell{
		{Symbol: "border"}, {Symbol: "你", BG: 0x02010203}, {Skip: true}, {Symbol: "a"}, {Symbol: " "}, {Symbol: "border"},
		{Symbol: "border"}, {Symbol: "b"}, {Symbol: "c"}, {Symbol: "d"}, {Symbol: " "}, {Symbol: "border"},
	}}, Panes: []endpoint.SurfacePane{{PaneID: "p", InnerRect: endpoint.SurfaceRect{X: 1, Width: 4, Height: 2}}}}
}
func TestNativeCropColorsWideAndSelection(t *testing.T) {
	v := New(Config{Theme: terminal.LightTheme(), DarkTheme: terminal.LightTheme()})
	v.SetFrame(fixture(), "p")
	tt := ui.NewTester(func(c *ui.Context) { v.View(c).Fill() }, 120, 60)
	cw, ch := CellMetrics(terminal.Font{})
	img := tt.Image()
	for _, x := range []int{1, int(cw) + 1} {
		got := img.RGBAAt(x, int(ch)-2)
		if got.R != 1 || got.G != 2 || got.B != 3 {
			t.Fatalf("wide cell background at %d: %v", x, got)
		}
	}
	v.selectAll()
	if got := v.SelectedText(); got != "你a\nbcd" {
		t.Fatalf("cropped selection %q", got)
	}
	v.Search("你a")
	if len(v.matches) != 1 || v.matches[0].start != 0 || v.matches[0].end != 3 {
		t.Fatalf("wide match %+v", v.matches)
	}
}
func TestInputIMEAndClipboard(t *testing.T) {
	var events []ui.InputEvent
	var copied, pasted string
	v := New(Config{Copy: func(s string) { copied = s }, Paste: func() string { return "x\ny" }, OnPaste: func(s string) bool { pasted = s; return true }, OnInput: func(e ui.InputEvent) bool { events = append(events, e); return true }})
	v.SetFrame(fixture(), "p")
	tt := ui.NewTester(func(c *ui.Context) { v.View(c).Fill().AutoFocus() }, 120, 60)
	tt.Compose("拼", 1)
	if !v.Composing() {
		t.Fatal("composition missing")
	}
	if len(events) != 0 {
		t.Fatal("preedit sent to server")
	}
	tt.Type("拼音")
	if v.Composing() || len(events) != 1 || events[0].Text != "拼音" {
		t.Fatalf("commit: %+v", events)
	}
	v.selectAll()
	v.input(ui.InputEvent{Kind: ui.InputCommand, Text: "copy"})
	if copied != "你a\nbcd" {
		t.Fatal(copied)
	}
	v.input(ui.InputEvent{Kind: ui.InputCommand, Text: "paste"})
	if pasted != "x\ny" {
		t.Fatal(pasted)
	}
	tt.RightClickAt(2, 2)
	if len(tt.Menu()) != 3 {
		t.Fatalf("context menu %v", tt.Menu())
	}
}
func TestColorTagsAndModifiers(t *testing.T) {
	v := New(Config{})
	v.theme = terminal.DarkTheme()
	if got := v.color(0x02ff0080, ui.Color{}); got != ui.RGB(255, 0, 128) {
		t.Fatal(got)
	}
	if got := v.color(0x010000e8, ui.Color{}); got != ui.RGB(8, 8, 8) {
		t.Fatal(got)
	}
	fg, bg := v.colors(endpoint.SurfaceCell{FG: 0x02010203, BG: 0x02040506, Modifier: 64})
	if fg != ui.RGB(4, 5, 6) || bg != ui.RGB(1, 2, 3) {
		t.Fatal(fg, bg)
	}
}

func TestMouseReportingAndShiftSelection(t *testing.T) {
	var forwarded []ui.InputEvent
	v := New(Config{OnInput: func(e ui.InputEvent) bool { forwarded = append(forwarded, e); return true }})
	frame := fixture()
	frame.Panes[0].MouseReporting = true
	v.SetFrame(frame, "p")
	tt := ui.NewTester(func(c *ui.Context) { v.View(c).Fill() }, 120, 60)
	tt.ClickAt(2, 2)
	if len(forwarded) < 2 || forwarded[len(forwarded)-2].Kind != ui.InputPointerDown || forwarded[len(forwarded)-1].Kind != ui.InputPointerUp {
		t.Fatalf("mouse reporting: %+v", forwarded)
	}
	count := len(forwarded)
	cw, _ := CellMetrics(terminal.Font{})
	v.input(ui.InputEvent{Kind: ui.InputPointerDown, Button: 0, X: 1, Y: 1, Mods: ui.Shift})
	v.input(ui.InputEvent{Kind: ui.InputPointerMove, Button: 0, X: 2.5 * cw, Y: 1, Mods: ui.Shift})
	v.input(ui.InputEvent{Kind: ui.InputPointerUp, Button: 0, X: 2.5 * cw, Y: 1, Mods: ui.Shift})
	if v.SelectedText() != "你a" || len(forwarded) != count {
		t.Fatalf("shift selection %q, forwarded %d", v.SelectedText(), len(forwarded))
	}
}

func TestReportedDragKeepsReleaseAfterShiftChanges(t *testing.T) {
	for _, popup := range []bool{false, true} {
		var forwarded []ui.InputEvent
		v := New(Config{OnInput: func(e ui.InputEvent) bool { forwarded = append(forwarded, e); return true }})
		frame := fixture()
		frame.Panes[0].MouseReporting = !popup
		frame.Panes[0].Focused = true
		if popup {
			frame.Popup = &endpoint.SurfacePopup{TerminalID: "popup", MouseReporting: true, Frame: frame.Frame}
		}
		v.SetFrame(frame, "p")
		_ = ui.NewTester(func(c *ui.Context) { v.View(c).Fill() }, 120, 60)
		// Exercise the widget's handler, including its local-selection gate,
		// rather than calling the downstream transport handler directly.
		for _, event := range []ui.InputEvent{
			{Kind: ui.InputPointerDown, Button: 0, X: 1, Y: 1},
			{Kind: ui.InputPointerMove, Button: 0, X: 20, Y: 1, Mods: ui.Shift},
			{Kind: ui.InputPointerUp, Button: 0, X: 20, Y: 1, Mods: ui.Shift},
		} {
			v.input(event)
		}
		if len(forwarded) != 3 || forwarded[1].Kind != ui.InputPointerMove || forwarded[2].Kind != ui.InputPointerUp || v.selecting || v.reportingPointer {
			t.Fatalf("popup=%v: reported drag lost release or became selection: %+v", popup, forwarded)
		}
	}
}

func TestPopupAndGraphicsCapability(t *testing.T) {
	v := New(Config{})
	f := fixture()
	f.Panes[0].Focused = true
	f.Popup = &endpoint.SurfacePopup{Frame: endpoint.SurfaceGrid{Width: 1, Height: 1, Cells: []endpoint.SurfaceCell{{Symbol: "弹"}}}}
	f.Graphics.Placements = []endpoint.SurfaceGraphicsPlacement{{}}
	v.SetFrame(f, "p")
	tt := ui.NewTester(func(c *ui.Context) { v.View(c).Fill() }, 120, 60)
	if !v.popup || v.offsetX <= 0 || !v.UnsupportedGraphics() {
		t.Fatalf("popup/capability: %+v", v)
	}
	v.selectAll()
	if v.SelectedText() != "弹" {
		t.Fatal(v.SelectedText())
	}
	if tt.Image().Bounds().Dx() != 120 {
		t.Fatal("render failed")
	}
}

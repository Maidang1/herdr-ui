package ui

import (
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/layout"
	"github.com/bytedance/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
	"reflect"
	"testing"
)

func nestedTree() *layout.Node {
	return &layout.Node{Type: "split", Direction: "right", Ratio: .4, First: &layout.Node{Type: "pane", PaneID: "left"}, Second: &layout.Node{Type: "split", Direction: "down", Ratio: .6, First: &layout.Node{Type: "pane", PaneID: "top"}, Second: &layout.Node{Type: "pane", PaneID: "bottom"}}}
}
func TestDividerMatchingUsesSubtreeGeometryNotOpaqueID(t *testing.T) {
	root := nestedTree()
	// Grid rounding differs from floating-point ratios, so use authoritative
	// descendant unions rather than recomputing pane edges from the tree.
	panes := []layout.PaneRect{{PaneID: "left", Rect: layout.Rect{W: 39, H: 80}}, {PaneID: "top", Rect: layout.Rect{X: 39, W: 61, H: 47}}, {PaneID: "bottom", Rect: layout.Rect{X: 39, Y: 47, W: 61, H: 33}}}
	hit := layout.SplitRect{ID: "arbitrary:not:a:path", Direction: "down", Ratio: .6, Rect: layout.Rect{X: 39, W: 61, H: 80}}
	path, err := matchSplit(root, hit, panes)
	if err != nil || !reflect.DeepEqual(path, []bool{true}) {
		t.Fatalf("path=%v err=%v", path, err)
	}
	hit.Rect.W = 60
	if _, err := matchSplit(root, hit, panes); err == nil {
		t.Fatal("accepted geometry from another topology")
	}
}
func TestPreviewResizesOnlySelectedNestedSplit(t *testing.T) {
	root := nestedTree()
	copy := layout.Clone(root)
	node, _ := layout.ResolvePath(copy, []bool{true})
	node.Ratio = .8
	panes, splits := preview(copy, layout.Rect{W: 100, H: 80})
	if len(panes) != 3 || len(splits) != 2 {
		t.Fatal(panes, splits)
	}
	if panes[0].Rect != (layout.Rect{W: 40, H: 80}) || panes[1].Rect.H != 64 || panes[2].Rect.Y != 64 {
		t.Fatal(panes)
	}
	if root.Second.Ratio != .6 {
		t.Fatal("preview mutated authoritative tree")
	}
}

func TestGridDividerTracksSharedPaneEdgeBeforeProjection(t *testing.T) {
	source := []layout.SplitRect{{ID: "opaque", Direction: "right", Ratio: .5, Rect: layout.Rect{W: 81, H: 24}}}
	splits := gridDividers(source)
	projection, err := layout.Project(layout.Rect{W: 81, H: 24}, []layout.PaneRect{{PaneID: "left", Rect: layout.Rect{W: 41, H: 24}}, {PaneID: "right", Rect: layout.Rect{X: 41, W: 40, H: 24}}}, splits, layout.Rect{W: 810, H: 240})
	if err != nil {
		t.Fatal(err)
	}
	hit, ok := layout.HitSplit(projection.Splits, 410, 120, 8)
	if !ok || hit.ID != "opaque" {
		t.Fatal("divider missed actual shared grid edge")
	}
	if source[0].Ratio != .5 {
		t.Fatal("changed authoritative ratio")
	}
}

func TestDividerPointerKeepsExistingTerminalFocus(t *testing.T) {
	w := New(app.New(), nil)
	w.controller = &actionController{}
	split := layout.SplitRect{ID: "split", Direction: "right", Ratio: .5, Rect: layout.Rect{W: 200, H: 100}}
	w.projected = app.View{Layout: session.PaneLayout{Splits: []layout.SplitRect{split}}}
	w.projectionValid = true
	w.projectionVersion = w.app.Version()
	focused := false
	tester := native.NewTester(func(c *native.Context) {
		native.Box(c).Fill().Children(func() {
			terminal := native.Box(c.Key("simulated-terminal")).Fill().Focusable().AutoFocus().Label("Terminal")
			terminal.HandleInput(func(ev native.InputEvent) bool { return ev.Kind == native.InputKeyDown })
			focused = terminal.Focused()
			w.dividers(c, layout.Projection{Splits: []layout.SplitRect{split}}, []layout.SplitRect{split}, nil, design.Colors(false))
		})
	}, 200, 100)
	tester.Frame()
	if !focused {
		t.Fatal("terminal did not receive initial focus")
	}
	if err := tester.Click("Resize split"); err != nil {
		t.Fatal(err)
	}
	if !focused {
		t.Fatal("divider pointer interaction cleared terminal focus")
	}
}

func TestDividersKeepLayoutCapturedBeforeSelectionChanges(t *testing.T) {
	w := New(app.New(), nil)
	w.controller = &actionController{}
	split := layout.SplitRect{ID: "old-tab-split", Direction: "right", Ratio: .5, Rect: layout.Rect{W: 200, H: 100}}
	frame := app.View{Layout: session.PaneLayout{Splits: []layout.SplitRect{split}}}
	projection := layout.Projection{Splits: []layout.SplitRect{split}}
	tester := native.NewTester(func(c *native.Context) {
		// Selection changes during a frame, after its geometry was projected.
		// The newly selected single-pane/zoomed layout has no dividers.
		w.projected = app.View{}
		w.projectionValid = true
		w.projectionVersion = w.app.Version()
		native.Box(c).Fill().Children(func() { w.dividers(c, projection, frame.Layout.Splits, nil, design.Colors(false)) })
	}, 200, 100)
	if _, ok := tester.Find("Resize split"); !ok {
		t.Fatal("lost divider from captured frame")
	}
}

func TestDividerSourceMatchesIdentityInsteadOfSlicePosition(t *testing.T) {
	a := layout.SplitRect{ID: "opaque-A", Direction: "right", Ratio: .3}
	b := layout.SplitRect{ID: "opaque-B", Direction: "down", Ratio: .7}
	source, ok := dividerSource("opaque-A", []layout.SplitRect{b, a}, nil)
	if !ok || source != a {
		t.Fatal("matched another split by position", source)
	}
	if _, ok := dividerSource("removed", []layout.SplitRect{b, a}, nil); ok {
		t.Fatal("rendered a split absent from captured layout")
	}
}

func TestPreviewDividerUsesCapturedDragWithoutAuthoritativeSplits(t *testing.T) {
	split := layout.SplitRect{ID: "opaque-original", Direction: "down", Ratio: .6, Rect: layout.Rect{W: 200, H: 100}}
	drag := &ratioDrag{ready: true, split: split, op: session.Operation{Path: []bool{true}}}
	source, ok := dividerSource("root1", nil, drag)
	if !ok || source != split {
		t.Fatal("preview lost captured drag source", source)
	}
	if _, ok := dividerSource("root", nil, drag); ok {
		t.Fatal("rendered inactive preview split")
	}
	w := New(app.New(), nil)
	w.controller = &actionController{}
	tester := native.NewTester(func(c *native.Context) {
		native.Box(c).Fill().Children(func() {
			w.dividers(c, layout.Projection{Splits: []layout.SplitRect{{ID: "root1", Direction: "down", Ratio: .6, Rect: split.Rect}}}, nil, drag, design.Colors(false))
		})
	}, 200, 100)
	if _, ok := tester.Find("Resize split"); !ok {
		t.Fatal("preview divider not rendered")
	}
}

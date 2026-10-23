package layout

import (
	"math"
	"reflect"
	"testing"
)

func TestProjectSharedEdgesAndOffset(t *testing.T) {
	panes := []PaneRect{{"a", Rect{10, 20, 33, 24}}, {"b", Rect{43, 20, 67, 24}}}
	splits := []SplitRect{{"root", "right", .33, Rect{10, 20, 100, 24}}}
	p, err := Project(Rect{10, 20, 100, 24}, panes, splits, Rect{7, 9, 701, 351})
	if err != nil {
		t.Fatal(err)
	}
	a, b := p.Panes[0].Rect, p.Panes[1].Rect
	if a.X != 7 || a.Y != 9 || a.X+a.W != b.X || b.X+b.W != 708 || b.Y+b.H != 360 {
		t.Fatalf("cracked or out of bounds: %+v", p)
	}
	if p.Splits[0].Rect != (Rect{7, 9, 701, 351}) {
		t.Fatal(p.Splits)
	}
	p.Panes[0].PaneID = "changed"
	if panes[0].PaneID != "a" {
		t.Fatal("input aliased")
	}
}

func TestProjectRejectsInvalidGeometry(t *testing.T) {
	for _, r := range []Rect{{0, 0, 0, 10}, {0, 0, -1, 10}, {math.NaN(), 0, 10, 10}, {0, 0, math.Inf(1), 10}, {math.MaxFloat64, 0, math.MaxFloat64, 10}} {
		if _, err := Project(r, nil, nil, Rect{0, 0, 10, 10}); err == nil {
			t.Fatalf("accepted area %+v", r)
		}
		if _, err := Project(Rect{0, 0, 10, 10}, nil, nil, r); err == nil {
			t.Fatalf("accepted viewport %+v", r)
		}
	}
	for _, r := range []Rect{{-1, 0, 5, 5}, {0, 0, 11, 5}, {0, 0, 0, 5}, {0, 0, 5, math.NaN()}} {
		if _, err := Project(Rect{0, 0, 10, 10}, []PaneRect{{"p", r}}, nil, Rect{0, 0, 20, 20}); err == nil {
			t.Fatalf("accepted pane %+v", r)
		}
	}
	for _, split := range []SplitRect{
		{"s", "left", .5, Rect{0, 0, 10, 10}},
		{"s", "right", math.NaN(), Rect{0, 0, 10, 10}},
		{"s", "right", 1, Rect{0, 0, 10, 10}},
		{"s", "right", .5, Rect{0, 0, 11, 10}},
	} {
		if _, err := Project(Rect{0, 0, 10, 10}, nil, []SplitRect{split}, Rect{0, 0, 20, 20}); err == nil {
			t.Fatalf("accepted split %+v", split)
		}
	}
}

func TestHitSplitUsesDividerAndNearestLine(t *testing.T) {
	s := []SplitRect{{"root", "right", .5, Rect{0, 0, 100, 100}}, {"child", "down", .5, Rect{50, 0, 50, 100}}}
	if _, ok := HitSplit(s, 10, 10, 8); ok {
		t.Fatal("whole parent rectangle was hit")
	}
	if hit, ok := HitSplit(s, 53, 40, 8); !ok || hit.ID != "root" {
		t.Fatalf("hit=%+v %v", hit, ok)
	}
	if hit, ok := HitSplit(s, 53, 51, 8); !ok || hit.ID != "child" {
		t.Fatalf("nearest hit=%+v %v", hit, ok)
	}
	if _, ok := HitSplit(s, 50, 101, 8); ok {
		t.Fatal("hit beyond divider endpoint")
	}
	if _, ok := HitSplit(s, math.NaN(), 50, 8); ok {
		t.Fatal("hit nonfinite pointer")
	}
}

func TestFingerprintCloneAndAncestorPaths(t *testing.T) {
	root := &Node{Type: "split", Direction: "right", Ratio: .5, First: &Node{Type: "pane", PaneID: "a"}, Second: &Node{Type: "split", Direction: "down", Ratio: .6, First: &Node{Type: "pane", PaneID: "b"}, Second: &Node{Type: "pane", PaneID: "c"}}}
	f, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	copy := Clone(root)
	copy.Second.Ratio = .7
	g, err := Fingerprint(copy)
	if err != nil || f != g {
		t.Fatal("ratio changed fingerprint", err)
	}
	copy.Second.Second.PaneID = "d"
	g, _ = Fingerprint(copy)
	if f == g || root.Second.Second.PaneID != "c" {
		t.Fatal("identity ignored or clone aliased")
	}
	copy = Clone(root)
	copy.Second.Direction = "right"
	g, _ = Fingerprint(copy)
	if f == g {
		t.Fatal("topology ignored")
	}
	copy = Clone(root)
	copy.First, copy.Second = copy.Second, copy.First
	g, _ = Fingerprint(copy)
	if f == g {
		t.Fatal("child ordering ignored")
	}
	p, err := FindPath(root, "c")
	if err != nil || !reflect.DeepEqual(p, []bool{true, true}) {
		t.Fatal(p, err)
	}
	p, err = SplitForPane(root, "c", "up")
	if err != nil || !reflect.DeepEqual(p, []bool{true}) {
		t.Fatal(p, err)
	}
	p, err = SplitForPane(root, "c", "left")
	if err != nil || len(p) != 0 {
		t.Fatal(p, err)
	}
	if _, err = SplitForPane(root, "a", "down"); err == nil {
		t.Fatal("accepted missing axis")
	}
	if _, err = FindPath(root, "missing"); err == nil {
		t.Fatal("accepted missing pane")
	}
	copy = Clone(root)
	copy.Second.Second = copy
	if _, err := Fingerprint(copy); err == nil {
		t.Fatal("accepted cyclic tree")
	}
	copy = Clone(root)
	copy.First.First = &Node{Type: "pane", PaneID: "extra"}
	if _, err := Fingerprint(copy); err == nil {
		t.Fatal("accepted child-bearing pane")
	}
	root.Second.Second.PaneID = "a"
	if _, err = Fingerprint(root); err == nil {
		t.Fatal("accepted invalid tree")
	}
}

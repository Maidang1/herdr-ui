package layout

import (
	"fmt"
	"math"
)

// Rect is an axis-aligned rectangle in source grid units or projected DIP.
type Rect struct{ X, Y, W, H float64 }
type PaneRect struct {
	PaneID string
	Rect   Rect
}
type SplitRect struct {
	ID, Direction string
	Ratio         float64
	Rect          Rect
}
type Projection struct {
	Panes  []PaneRect
	Splits []SplitRect
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validRect(r Rect) bool {
	return finite(r.X) && finite(r.Y) && finite(r.W) && finite(r.H) && r.W > 0 && r.H > 0 && finite(r.X+r.W) && finite(r.Y+r.H) && r.X+r.W > r.X && r.Y+r.H > r.Y
}
func inside(r, area Rect) bool {
	return validRect(r) && r.X >= area.X && r.Y >= area.Y && r.X+r.W <= area.X+area.W && r.Y+r.H <= area.Y+area.H
}
func validSplit(s SplitRect) bool {
	return s.ID != "" && (s.Direction == "right" || s.Direction == "down") && finite(s.Ratio) && s.Ratio > 0 && s.Ratio < 1 && validRect(s.Rect)
}

// Project maps all received visible geometry to viewport. It transforms shared
// edges once, then derives widths from those edges, never rounding widths apart.
// Empty source/viewport dimensions are errors so hidden views cannot attach.
func Project(area Rect, panes []PaneRect, splits []SplitRect, viewport Rect) (Projection, error) {
	if !validRect(area) || !validRect(viewport) {
		return Projection{}, fmt.Errorf("invalid layout area or viewport")
	}
	xs, ys := map[float64]float64{}, map[float64]float64{}
	edge := func(cache map[float64]float64, v, start, size, dest, span float64) float64 {
		if result, ok := cache[v]; ok {
			return result
		}
		var result float64
		switch v {
		case start:
			result = dest
		case start + size:
			result = dest + span
		default:
			result = dest + (v-start)/size*span
		}
		cache[v] = result
		return result
	}
	project := func(r Rect) (Rect, error) {
		if !inside(r, area) {
			return Rect{}, fmt.Errorf("rectangle outside layout area")
		}
		x1 := edge(xs, r.X, area.X, area.W, viewport.X, viewport.W)
		x2 := edge(xs, r.X+r.W, area.X, area.W, viewport.X, viewport.W)
		y1 := edge(ys, r.Y, area.Y, area.H, viewport.Y, viewport.H)
		y2 := edge(ys, r.Y+r.H, area.Y, area.H, viewport.Y, viewport.H)
		out := Rect{x1, y1, x2 - x1, y2 - y1}
		if !validRect(out) {
			return Rect{}, fmt.Errorf("unrepresentable projected rectangle")
		}
		return out, nil
	}
	out := Projection{Panes: make([]PaneRect, 0, len(panes)), Splits: make([]SplitRect, 0, len(splits))}
	seen := map[string]bool{}
	for _, p := range panes {
		if p.PaneID == "" || seen[p.PaneID] {
			return Projection{}, fmt.Errorf("missing or duplicate pane ID")
		}
		seen[p.PaneID] = true
		r, err := project(p.Rect)
		if err != nil {
			return Projection{}, err
		}
		out.Panes = append(out.Panes, PaneRect{p.PaneID, r})
	}
	seen = map[string]bool{}
	for _, s := range splits {
		if !validSplit(s) || seen[s.ID] {
			return Projection{}, fmt.Errorf("invalid or duplicate split")
		}
		seen[s.ID] = true
		r, err := project(s.Rect)
		if err != nil {
			return Projection{}, err
		}
		s.Rect = r
		out.Splits = append(out.Splits, s)
	}
	return out, nil
}

// HitSplit finds the nearest actual divider within hitWidth, with stable input
// order breaking ties. The perpendicular coordinate must lie on the divider.
func HitSplit(projected []SplitRect, x, y, hitWidth float64) (SplitRect, bool) {
	if !finite(x) || !finite(y) || !finite(hitWidth) || hitWidth <= 0 {
		return SplitRect{}, false
	}
	best := math.Inf(1)
	var hit SplitRect
	found := false
	for _, s := range projected {
		if !validSplit(s) {
			continue
		}
		r := s.Rect
		var distance float64
		if s.Direction == "right" {
			if y < r.Y || y > r.Y+r.H {
				continue
			}
			distance = math.Abs(x - (r.X + r.W*s.Ratio))
		} else {
			if x < r.X || x > r.X+r.W {
				continue
			}
			distance = math.Abs(y - (r.Y + r.H*s.Ratio))
		}
		if distance <= hitWidth/2 && distance < best {
			best = distance
			hit = s
			found = true
		}
	}
	return hit, found
}

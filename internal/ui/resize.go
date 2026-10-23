package ui

import (
	"fmt"
	"math"
	"time"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/layout"
	"github.com/bytedance/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

type ratioDrag struct {
	op              session.Operation
	revision        uint64
	ready, released bool
	root            *layout.Node
	split           layout.SplitRect
	screen          layout.Rect
	last            time.Time
	ratio           float64
	lastRatio       float64
	panes           string
	publication     uint64
	checking        bool
	generation      uint64
}

func paneSignature(v app.View) string {
	s := ""
	for _, p := range v.Layout.Panes {
		s += p.PaneID + ";"
	}
	return s
}
func (w *Window) updateDrag(v app.View) {
	if w.controller == nil || w.controller.drag == nil {
		return
	}
	d := w.controller.drag
	if !w.sameIntent(d.op, d.revision) || v.Layout.Zoomed || paneSignature(v) != d.panes {
		w.controller.drag = nil
		return
	}
	latest := w.terminal.session.Latest()
	canSend := latest.State == session.Ready && !latest.Stale
	if d.ready && canSend && (d.released || d.ratio != d.lastRatio) {
		w.moveDrag(d.ratio, d.released)
		if w.controller.drag != d {
			return
		}
	}
	if d.ready && canSend && d.publication != v.Revision && !d.checking {
		d.publication = v.Revision
		d.checking = true
		op := d.op
		op.Kind = session.ExportLayout
		w.execute(op, func(r session.OperationResult) {
			d.checking = false
			if w.controller.drag != d {
				return
			}
			if r.Status != session.Succeeded || r.Fingerprint != d.op.Fingerprint {
				w.controller.drag = nil
				w.controller.message = "Layout changed; resize canceled."
			}
		})
	}
}
func subtreeRect(n *layout.Node, panes []layout.PaneRect) (layout.Rect, bool) {
	if n.Type == "pane" {
		for _, p := range panes {
			if p.PaneID == n.PaneID {
				return p.Rect, true
			}
		}
		return layout.Rect{}, false
	}
	a, ok := subtreeRect(n.First, panes)
	if !ok {
		return layout.Rect{}, false
	}
	b, ok := subtreeRect(n.Second, panes)
	if !ok {
		return layout.Rect{}, false
	}
	x, y := math.Min(a.X, b.X), math.Min(a.Y, b.Y)
	return layout.Rect{X: x, Y: y, W: math.Max(a.X+a.W, b.X+b.W) - x, H: math.Max(a.Y+a.H, b.Y+b.H) - y}, true
}
func matchSplit(root *layout.Node, hit layout.SplitRect, panes []layout.PaneRect) ([]bool, error) {
	paths, err := layout.SplitPaths(root)
	if err != nil {
		return nil, err
	}
	var match []bool
	count := 0
	for _, path := range paths {
		n, _ := layout.ResolvePath(root, path)
		r, ok := subtreeRect(n, panes)
		if ok && n.Direction == hit.Direction && r == hit.Rect {
			match = path
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("layout changed before divider selection")
	}
	return match, nil
}
func (w *Window) beginDrag(split layout.SplitRect, screen layout.Rect) {
	if w.controller == nil || w.controller.drag != nil {
		return
	}
	op, ok := w.operation(session.ExportLayout, "")
	if !ok {
		return
	}
	v := w.currentView()
	w.controller.ratioGeneration++
	d := &ratioDrag{generation: w.controller.ratioGeneration, op: op, revision: w.app.ViewRevision(), split: split, screen: screen, ratio: split.Ratio, panes: paneSignature(v), publication: v.Revision}
	w.controller.drag = d
	w.execute(op, func(r session.OperationResult) {
		if w.controller.drag != d || !w.sameIntent(op, d.revision) {
			return
		}
		if r.Status != session.Succeeded {
			w.controller.message = r.Error
			w.controller.drag = nil
			return
		}
		path, err := matchSplit(r.Root, split, v.Layout.Panes)
		if err != nil {
			w.controller.message = err.Error()
			w.controller.drag = nil
			return
		}
		d.root = layout.Clone(r.Root)
		d.op.Kind = session.SetRatio
		d.op.Path = path
		d.op.Fingerprint = r.Fingerprint
		d.ready = true
		w.moveDrag(d.ratio, d.released)
	})
}
func (w *Window) moveDrag(ratio float64, final bool) {
	d := w.controller.drag
	if d == nil {
		return
	}
	d.ratio = clampRatio(ratio)
	d.released = final
	if !d.ready {
		return
	}
	if !w.sameIntent(d.op, d.revision) {
		w.controller.drag = nil
		return
	}
	node, err := layout.ResolvePath(d.root, d.op.Path)
	if err != nil {
		w.controller.drag = nil
		return
	}
	node.Ratio = d.ratio
	latest := w.terminal.session.Latest()
	if latest.State != session.Ready || latest.Stale {
		return
	}
	if final || time.Since(d.last) >= 50*time.Millisecond {
		d.last = time.Now()
		d.lastRatio = d.ratio
		op := d.op
		op.Ratio = d.ratio
		w.sendRatio(op, d, d.generation)
	}
	if final {
		w.controller.drag = nil
	}
}

// Preview is derived from the exported topology. It never invents terminal IDs.
func preview(root *layout.Node, area layout.Rect) ([]layout.PaneRect, []layout.SplitRect) {
	var panes []layout.PaneRect
	var splits []layout.SplitRect
	var walk func(*layout.Node, layout.Rect, string)
	walk = func(n *layout.Node, r layout.Rect, id string) {
		if n.Type == "pane" {
			panes = append(panes, layout.PaneRect{PaneID: n.PaneID, Rect: r})
			return
		}
		splits = append(splits, layout.SplitRect{ID: id, Direction: n.Direction, Ratio: n.Ratio, Rect: r})
		a, b := r, r
		if n.Direction == "right" {
			a.W = r.W * n.Ratio
			b.X = r.X + a.W
			b.W = r.W - a.W
		} else {
			a.H = r.H * n.Ratio
			b.Y = r.Y + a.H
			b.H = r.H - a.H
		}
		walk(n.First, a, id+"0")
		walk(n.Second, b, id+"1")
	}
	walk(root, area, "root")
	return panes, splits
}
func (w *Window) dividers(c *native.Context, projection layout.Projection, p design.Palette) {
	if w.controller == nil {
		return
	}
	v := w.currentView()
	for i, s := range projection.Splits {
		s := s
		source := v.Layout.Splits[i]
		if d := w.controller.drag; d != nil && d.ready {
			id := "root"
			for _, second := range d.op.Path {
				if second {
					id += "1"
				} else {
					id += "0"
				}
			}
			if s.ID != id {
				continue
			}
			source = d.split
		}
		r := s.Rect
		x, y, width, height := r.X, r.Y, r.W, r.H
		if s.Direction == "right" {
			x += r.W*s.Ratio - 4
			width = 8
		} else {
			y += r.H*s.Ratio - 4
			height = 8
		}
		divider := native.Box(c.Key("divider:"+source.ID)).Absolute().Left(float32(x)).Top(float32(y)).Size(float32(width), float32(height)).Label("Resize split").KeepFocus()
		divider.HandleInput(func(ev native.InputEvent) bool {
			switch ev.Kind {
			case native.InputPointerDown:
				if ev.Button != 0 {
					return false
				}
				w.beginDrag(source, r)
				return true
			case native.InputPointerMove, native.InputPointerUp:
				d := w.controller.drag
				if d == nil {
					return false
				}
				ratio := d.ratio
				// Pointer coordinates remain relative to the currently rendered hit box.
				if s.Direction == "right" {
					ratio = (x + float64(ev.X) - d.screen.X) / d.screen.W
				} else {
					ratio = (y + float64(ev.Y) - d.screen.Y) / d.screen.H
				}
				w.moveDrag(ratio, ev.Kind == native.InputPointerUp)
				return true
			}
			return false
		})
		divider.Children(func() {
			line := native.Box(c).Absolute().Background(p.Border)
			if s.Direction == "right" {
				line.Left(3.5).Top(0).Size(1, float32(height))
			} else {
				line.Top(3.5).Left(0).Size(float32(width), 1)
			}
		})
	}
}

// Herdr split_rect multiplies f32 grid dimensions and rounds before projecting
// into DIP. Keep the true ratio on the source snapshot for mutations.
func gridDividers(splits []layout.SplitRect) []layout.SplitRect {
	out := append([]layout.SplitRect(nil), splits...)
	for i := range out {
		s := &out[i]
		span := s.Rect.W
		if s.Direction == "down" {
			span = s.Rect.H
		}
		if span > 0 {
			s.Ratio = math.Round(float64(float32(span)*float32(s.Ratio))) / span
		}
	}
	return out
}

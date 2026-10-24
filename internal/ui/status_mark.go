package ui

import (
	"math"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

// Herdr's "dots" status marks, in one painter: a ring with a turning arc
// while an agent works, a solid dot with a slow halo while it waits for
// you, a solid dot once done, a hollow ring while idle, and a speck while
// nothing is known. Motion only repaints the mark; it never rebuilds the
// view, and it stops when the system asks for reduced motion.
const (
	statusSpinPeriod  = time.Second
	statusPulsePeriod = 1600 * time.Millisecond
)

// statusMark draws the mark for value in a size×size box.
func statusMark(c *native.Context, key, value string, size float32, p design.Palette) native.Element {
	lower := strings.ToLower(value)
	_, color, label := status(value, c.Theme().Dark)
	still := c.Preferences().ReduceMotion
	return native.Box(c.Key("status-mark:"+key)).Size(size, size).Shrink(0).Label(label).Draw(func(pt *native.Painter, r native.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		ring := r.W/2 - 1
		switch lower {
		case "working":
			var track native.Path
			track.Circle(cx, cy, ring)
			pt.StrokePath(&track, 1.5, color.Alpha(.25))
			turn := float32(0)
			if !still {
				turn = statusPhase(pt.Now(), statusSpinPeriod)
				pt.AnimationFrame()
			}
			statusArc(pt, cx, cy, ring, turn, .34, color)
		case "blocked":
			if !still {
				t := statusPhase(pt.Now(), statusPulsePeriod)
				var halo native.Path
				halo.Circle(cx, cy, r.W*.22+r.W*.28*t)
				pt.FillPath(&halo, color.Alpha(.35*(1-t)))
				pt.AnimationFrame()
			}
			var dot native.Path
			pt.FillPath(dot.Circle(cx, cy, r.W*.3), color)
		case "done":
			var dot native.Path
			pt.FillPath(dot.Circle(cx, cy, r.W*.3), color)
		case "idle":
			var circle native.Path
			pt.StrokePath(circle.Circle(cx, cy, r.W*.3), 1.25, color)
		default:
			var speck native.Path
			pt.FillPath(speck.Circle(cx, cy, max(1, r.W*.12)), p.Tertiary.Alpha(.7))
		}
	})
}

// statusPhase is how far through its period the animation is at now.
func statusPhase(now time.Time, period time.Duration) float32 {
	return float32(now.UnixMilli()%period.Milliseconds()) / float32(period.Milliseconds())
}

// statusArc strokes length of the ring's circumference starting at from,
// both as fractions of a turn, splitting it where it crosses the start.
func statusArc(pt *native.Painter, cx, cy, radius, from, length float32, color native.Color) {
	from = float32(math.Mod(float64(from), 1))
	to := from + length
	stroke := func(a, b float32) {
		var arc native.Path
		arc.Circle(cx, cy, radius).Trim(a, b)
		pt.StrokePath(&arc, 1.75, color)
	}
	if to <= 1 {
		stroke(from, to)
		return
	}
	stroke(from, 1)
	stroke(0, to-1)
}

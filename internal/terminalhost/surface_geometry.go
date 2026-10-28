package terminalhost

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/Maidang1/herdr-ui/internal/surfaceview"
)

// ConfigureSurface preserves the whole-canvas sizing contract for callers
// without native pane geometry.
func (h *Host) ConfigureSurface(tab, pane string, width, height float32, focused bool) {
	h.ConfigureSurfaceLayout(tab, pane, width, height, focused, nil, 0)
}

// ConfigureSurfaceLayout fits the complete server canvas into every projected
// native pane body. Headers consume pixels independently in each split pane;
// subtracting padding only once from the whole canvas cannot account for them.
// Server borders further reduce terminal content and are deliberately ignored
// here, so changing the daemon's border preference cannot hide terminal rows.
func (h *Host) ConfigureSurfaceLayout(tab, pane string, width, height float32, focused bool, panes []layout.PaneRect, header float32, splits ...layout.SplitRect) {
	if h.surface == nil {
		return
	}
	cw, ch := surfaceview.CellMetrics(h.cfg.Font)
	h.surface.mu.Lock()
	cols, rows, valid := h.surface.geometry.fit(float64(width), float64(height), float64(cw), float64(ch), panes, float64(header), splits)
	h.surface.mu.Unlock()
	if !valid {
		return
	}
	h.surface.update(surfaceSelection{tab: tab, pane: pane, cols: cols, rows: rows, focused: focused})
}

func surfaceLayoutSize(width, height, cw, ch float64, panes []layout.PaneRect, header float64) (uint16, uint16, bool) {
	finite := func(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
	if !finite(width) || !finite(height) || !finite(cw) || !finite(ch) || !finite(header) || width <= 32 || height <= 24 || cw <= 0 || ch <= 0 || header < 0 {
		return 0, 0, false
	}
	cols := math.Min(1000, math.Floor((width-32)/cw))
	rows := math.Min(1000, math.Floor((height-24)/ch))
	for _, p := range panes {
		r := p.Rect
		if !finite(r.X) || !finite(r.Y) || !finite(r.W) || !finite(r.H) || r.X < 0 || r.Y < 0 || r.W <= 0 || r.H <= header || r.X+r.W > width+0.01 || r.Y+r.H > height+0.01 {
			return 0, 0, false
		}
		fits := func(pixels, cell, fraction float64) float64 {
			capacity := math.Floor(pixels / cell)
			if fraction < 1-1e-6 {
				// The server rounds BSP split boundaries to cells independently of the
				// native projection. Keep one cell for that boundary rounding.
				capacity--
			}
			return math.Floor(capacity / fraction)
		}
		cols = math.Min(cols, fits(r.W, cw, r.W/width))
		rows = math.Min(rows, fits(r.H-header, ch, r.H/height))
	}
	// A first layout pass or a collapsed native body must not replace the last
	// valid canvas with a transient 1x1 PTY resize.
	if cols < 1 || rows < 1 {
		return 0, 0, false
	}
	return uint16(cols), uint16(rows), true
}

// The raw BSP ratios identify a user layout change. Projected pane fractions
// must not enter this key: their integer rounding changes after every resize.
type surfaceGeometryKey struct {
	width, height, cw, ch, header float64
	topology                      string
}
type surfaceGeometryState struct {
	key        surfaceGeometryKey
	cols, rows uint16
}

func (s *surfaceGeometryState) fit(width, height, cw, ch float64, panes []layout.PaneRect, header float64, splits []layout.SplitRect) (uint16, uint16, bool) {
	cols, rows, valid := surfaceLayoutSize(width, height, cw, ch, panes, header)
	if !valid {
		return 0, 0, false
	}
	parts := make([]string, 0, len(panes)+len(splits))
	for _, p := range panes {
		parts = append(parts, "pane:"+strconv.Quote(p.PaneID))
	}
	for _, split := range splits {
		parts = append(parts, "split:"+strconv.Quote(split.ID)+":"+strconv.Quote(split.Direction)+":"+strconv.FormatFloat(split.Ratio, 'g', -1, 64))
	}
	sort.Strings(parts)
	key := surfaceGeometryKey{width, height, cw, ch, header, strings.Join(parts, ";")}
	if s.key == key && s.cols > 0 && s.rows > 0 {
		// The daemon reprojections can alternate around a cell boundary. Within
		// one viewport/font/topology, only tighten the accepted fit. Real resize,
		// font changes, and immutable BSP-ratio changes can expand it again.
		cols = min(cols, s.cols)
		rows = min(rows, s.rows)
	}
	s.key, s.cols, s.rows = key, cols, rows
	return cols, rows, true
}

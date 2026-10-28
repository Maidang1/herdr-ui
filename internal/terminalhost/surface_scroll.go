package terminalhost

import (
	"github.com/egoist/mygo/ui"
	"math"
	"runtime"
)

func surfaceScrollRows(ev ui.InputEvent, height float32, remainder *float32) int {
	return surfaceScrollRowsFor(runtime.GOOS, ev, height, remainder)
}
func surfaceScrollRowsFor(platform string, ev ui.InputEvent, height float32, remainder *float32) int {
	delta := ev.DY
	if delta == 0 {
		delta = ev.DX
	}
	if math.IsNaN(float64(delta)) || math.IsInf(float64(delta), 0) || height <= 0 {
		return 0
	}
	if ev.Precise {
		*remainder += delta / height
	} else if platform == "windows" {
		// mygo scales Windows wheel deltas by the user's configured line
		// count and 33 DIPs per line. Undo that scale, not the line setting.
		*remainder += delta / 33
	} else {
		*remainder += delta / 40 * 3
	}
	rows := int(*remainder)
	*remainder -= float32(rows)
	return max(-1000, min(1000, rows))
}

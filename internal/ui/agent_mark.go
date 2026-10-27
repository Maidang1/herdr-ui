package ui

import (
	"github.com/Maidang1/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
)

// agentMark draws a known agent's logo in its own colors. A terminal or an
// unknown agent stays a monochrome icon in tint.
func agentMark(c *native.Context, name string, size float32, tint native.Color) {
	if mark, ok := icons.Brand(name, c.Theme().Dark); ok {
		native.Image(c, mark).Size(size, size).Shrink(0)
		return
	}
	native.Icon(c, icons.Agent(name)).Size(size, size).Shrink(0).TextColor(tint)
}

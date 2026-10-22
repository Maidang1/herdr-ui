package widgets

import (
	"herdr-ui/internal/design"
	"herdr-ui/internal/ui/icons"

	"github.com/egoist/mygo/ui"
)

// Icon draws one of the icons of the design.
//
// The name is the name the design file gives the icon: "panel-left",
// "git-branch". An icon that is not there is nothing, so that a window
// that asks for one does not stop.
func Icon(c *ui.Context, t design.Tokens, name string, size float32, color ui.Color) ui.Element {
	art := icons.Icon(name)
	if art == nil {
		return ui.Element{}
	}
	return ui.Icon(c, art).
		Size(size, size).
		TextColor(color)
}

package ui

import (
	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

type chromeDimensions struct {
	sidebarVisible bool
	terminalWidth  float32
}

func chromeLayout(width, sidebar float32, hidden bool) chromeDimensions {
	available := width - 16 - 2
	visible := !hidden
	if visible {
		available -= sidebar + 8
	}
	return chromeDimensions{visible, available}
}
func (w *Window) referenceOpen() bool { return w.controller != nil && w.controller.help }
func (w *Window) controlState(button native.Element, p design.Palette) {
	if button.Pressed() {
		button.Background(p.Pressed)
	} else if button.Hovered() {
		button.Background(p.Hover)
	}
	if button.FocusVisible() {
		button.Border(2, p.Focus)
	}
}
func (w *Window) iconButton(c *native.Context, key string, icon *native.SVG, label string, p design.Palette) native.Element {
	button := native.ButtonBase(c.Key(key)).Size(28, 28).Shrink(0).Radius(14).Center().Label(label)
	w.controlState(button, p)
	button.Children(func() { native.Icon(c, icon).Size(16, 16).TextColor(p.Secondary) })
	return button
}

// popoverPanel styles a popover's panel: its items sit MenuPadding in,
// rounded concentrically with it (shape.MenuItem).
func (w *Window) popoverPanel(panel native.Element, p design.Palette) {
	panel.MinWidth(200).Background(p.Elevated).Border(1, p.Border).Radius(w.shape.Popover).Shadow(0, 8, 24, 0, p.ContentShadow).Padding(design.MenuPadding)
}

func (w *Window) sidebarResizeHandle(c *native.Context) {
	handle := native.Box(c.Key("sidebar-resize")).Absolute().Top(0).Left(w.sidebarWidth - 4).Width(8).FillHeight().KeepFocus().Cursor(native.CursorResizeEW).Label("Resize sidebar")
	handle.HandleInput(func(ev native.InputEvent) bool {
		switch ev.Kind {
		case native.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			w.sidebarDragging = true
			w.sidebarDragOffset = ev.X
			return true
		case native.InputPointerMove, native.InputPointerUp:
			if !w.sidebarDragging {
				return false
			}
			w.sidebarWidth = max(220, min(320, w.sidebarWidth+ev.X-w.sidebarDragOffset))
			if ev.Kind == native.InputPointerUp {
				w.sidebarDragging = false
			}
			return true
		}
		return false
	})
}

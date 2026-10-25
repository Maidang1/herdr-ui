package ui

import (
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/egoist/mygo/plugins/glass"
	native "github.com/egoist/mygo/ui"
)

// How much of the raised surface's own color frosts a floating glass:
// a little for a bar or a toast, enough that controls passing under its
// few words do not break them up; most for a menu or a dialog, whose
// lists of text must read as steadily as on paper.
const (
	glassThin  float32 = .42
	glassDense float32 = .62
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

// floating fills a surface that floats over the window's content, as a
// menu, a bar, a toast or a dialog, with Liquid Glass frosted by frost of
// the raised surface's color, as macOS 26 draws them. Where the user asks
// for less transparency or more contrast it is the opaque raised surface,
// outlined.
func (w *Window) floating(c *native.Context, surface native.Element, p design.Palette, frost float32) native.Element {
	if w.glassy(c) {
		return surface.Material(glass.Glass{Tint: p.Elevated.Alpha(frost)})
	}
	return surface.Background(p.Elevated).Border(1, p.Border)
}

// popoverPanel styles a popover's panel: its items sit MenuPadding in,
// rounded concentrically with it (shape.MenuItem).
func (w *Window) popoverPanel(c *native.Context, panel native.Element, p design.Palette) {
	w.floating(c, panel.MinWidth(200).Radius(w.shape.Popover).Shadow(0, 8, 24, 0, p.ContentShadow).Padding(design.MenuPadding), p, glassDense)
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

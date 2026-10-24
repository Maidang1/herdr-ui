package design

// Shape is the window's corner system. Following Apple's concentric
// design, every nested surface derives its radius from the one around it:
// an inner radius is the outer radius less the inset between them, so the
// curves share one center and no square corner sits inside a round one.
type Shape struct {
	// Window is the system's own window corner radius.
	Window float32
	// Gap is the inset between the window edge and its panels, and
	// between neighboring panels.
	Gap float32
	// Panel rounds the floating sidebar and content panels.
	Panel float32
	// Row rounds a list row inset by Gap inside a panel.
	Row float32
	// Card rounds the inner corners of split terminal cards.
	Card float32
	// Popover rounds menus and popovers; MenuItem the items inset in them.
	Popover, MenuItem float32
	// Dialog rounds modal sheets and panels floating over the content.
	Dialog float32
}

// MenuPadding is the inset between a popover's edge and its items.
const MenuPadding float32 = 4

// Shapes derives the corner system from the window's corner radius.
func Shapes(window float32) Shape {
	const gap = 8
	panel := max(10, window-gap)
	popover := float32(12)
	return Shape{
		Window:   window,
		Gap:      gap,
		Panel:    panel,
		Row:      max(6, panel-gap),
		Card:     max(8, panel-gap),
		Popover:  popover,
		MenuItem: popover - MenuPadding,
		Dialog:   max(16, panel),
	}
}

// SystemShapes is the corner system for the running system's windows.
func SystemShapes() Shape { return Shapes(WindowCornerRadius()) }

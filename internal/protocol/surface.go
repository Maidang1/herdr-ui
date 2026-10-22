package protocol

// The parts of a pane's surface.
//
// The daemon sends the surface of the pane that is in front as a grid of
// cells: every cell says what character is in it, what color it is, and
// what the modifiers of the text are.

// The bits of the modifiers of a cell.
const (
	ModifierBold          = 1 << 0
	ModifierDim           = 1 << 1
	ModifierItalic        = 1 << 2
	ModifierUnderline     = 1 << 3
	ModifierReversed      = 1 << 6
	ModifierHidden        = 1 << 7
	ModifierStrikethrough = 1 << 8
)

// Cell is one character of the terminal.
type Cell struct {
	// Symbol is the character in the cell.
	Symbol string
	// Fg is the color of the character.
	Fg uint32
	// Bg is the color behind the character.
	Bg uint32
	// Modifier is the modifiers of the character.
	Modifier uint16
	// Skip tells that the cell is the second half of a wide character.
	Skip bool
	// Hyperlink is the number of the link of the cell, when it has one.
	Hyperlink *uint32
}

// Cursor is where the cursor of the terminal is.
type Cursor struct {
	// X is the column of the cursor.
	X uint16
	// Y is the row of the cursor.
	Y uint16
	// Visible tells whether the cursor is shown.
	Visible bool
	// Shape is the shape of the cursor.
	Shape uint8
}

// Rect is a place on the surface.
type Rect struct {
	// X is the column the place starts at.
	X uint16
	// Y is the row the place starts at.
	Y uint16
	// Width is the number of columns.
	Width uint16
	// Height is the number of rows.
	Height uint16
}

// PaneSurface is the place of one pane on the surface of the shell.
type PaneSurface struct {
	// PaneID is the pane.
	PaneID string
	// ContentRevision is how many times the content of the pane changed.
	ContentRevision uint64
	// Rect is where the pane is.
	Rect Rect
	// InnerRect is where the content of the pane is.
	InnerRect Rect
	// ScrollbarRect is where the scroll bar is, when there is one.
	ScrollbarRect *Rect
	// Focused tells that the pane has the keyboard.
	Focused bool
	// MouseReporting tells that the program of the pane wants the mouse.
	MouseReporting bool
	// AlternateScreenActive tells that the program of the pane is on the
	// other screen.
	AlternateScreenActive bool
	// PixelWidth is the width of the pane in pixels.
	PixelWidth uint32
	// PixelHeight is the height of the pane in pixels.
	PixelHeight uint32
}

// SurfaceFrame is the whole surface of the shell.
type SurfaceFrame struct {
	// BootID is the boot the surface belongs to.
	BootID string
	// ProjectionRevision is how many times the daemon rebuilt the
	// projection of the session.
	ProjectionRevision uint64
	// SurfaceRevision is how many times the surface changed.
	SurfaceRevision uint64
	// Cells are the characters, row by row.
	Cells []Cell
	// Width is the number of columns.
	Width uint16
	// Height is the number of rows.
	Height uint16
	// Cursor is where the cursor is.
	Cursor *Cursor
	// Hyperlinks are the links of the cells, by their number.
	Hyperlinks []string
	// Panes are the places of the panes on the surface.
	Panes []PaneSurface
}

// Row returns the cells of one row of the surface.
func (f *SurfaceFrame) Row(y int) []Cell {
	if y < 0 || f.Width == 0 {
		return nil
	}
	start := y * int(f.Width)
	if start >= len(f.Cells) {
		return nil
	}
	end := start + int(f.Width)
	if end > len(f.Cells) {
		end = len(f.Cells)
	}
	return f.Cells[start:end]
}

// Text returns the text of one row of the surface, without the spaces at
// its end.
func (f *SurfaceFrame) Text(y int) string {
	row := f.Row(y)
	var text []byte
	for _, cell := range row {
		if cell.Skip {
			continue
		}
		text = append(text, cell.Symbol...)
	}
	return trimRight(string(text))
}

// trimRight takes the spaces off the end of a line.
func trimRight(s string) string {
	end := len(s)
	for end > 0 && s[end-1] == ' ' {
		end--
	}
	return s[:end]
}

// decodeSurfaceFrame reads a surface.
func decodeSurfaceFrame(r *Reader) (*SurfaceFrame, error) {
	frame := &SurfaceFrame{}
	var err error
	if frame.BootID, err = r.Str(); err != nil {
		return nil, err
	}
	if frame.ProjectionRevision, err = r.U64(); err != nil {
		return nil, err
	}
	if frame.SurfaceRevision, err = r.U64(); err != nil {
		return nil, err
	}
	count, err := r.unsigned()
	if err != nil {
		return nil, err
	}
	frame.Cells = make([]Cell, 0, count)
	for i := uint64(0); i < count; i++ {
		cell, err := decodeCell(r)
		if err != nil {
			return nil, err
		}
		frame.Cells = append(frame.Cells, cell)
	}
	if frame.Width, err = r.U16(); err != nil {
		return nil, err
	}
	if frame.Height, err = r.U16(); err != nil {
		return nil, err
	}
	if frame.Cursor, err = decodeCursor(r); err != nil {
		return nil, err
	}
	links, err := r.unsigned()
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < links; i++ {
		link, err := r.Str()
		if err != nil {
			return nil, err
		}
		frame.Hyperlinks = append(frame.Hyperlinks, link)
	}
	// The graphics of the frame are not drawn by this client.
	if _, err := r.ByteSlice(); err != nil {
		return nil, err
	}
	panes, err := r.unsigned()
	if err != nil {
		return nil, err
	}
	for i := uint64(0); i < panes; i++ {
		pane, err := decodePaneSurface(r)
		if err != nil {
			return nil, err
		}
		frame.Panes = append(frame.Panes, pane)
	}
	// The splits, the popup and the graphics of the scene are not drawn
	// by this client, but they must be read, for they are part of the
	// frame.
	if err := skipSplits(r); err != nil {
		return nil, err
	}
	if err := skipPopup(r); err != nil {
		return nil, err
	}
	if err := skipGraphicsScene(r); err != nil {
		return nil, err
	}
	return frame, nil
}

// skipSplits reads the splits of the panes.
func skipSplits(r *Reader) error {
	count, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		// The direction of the split.
		if _, err := r.Variant(); err != nil {
			return err
		}
		if _, err := r.U16(); err != nil {
			return err
		}
		if _, err := decodeRect(r); err != nil {
			return err
		}
		if _, err := decodeRect(r); err != nil {
			return err
		}
		path, err := r.unsigned()
		if err != nil {
			return err
		}
		for j := uint64(0); j < path; j++ {
			if _, err := r.Bool(); err != nil {
				return err
			}
		}
	}
	return nil
}

// skipPopup reads the popup of the shell, when there is one.
func skipPopup(r *Reader) error {
	present, err := r.readOptionalTag()
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	if _, err := r.Str(); err != nil {
		return err
	}
	if _, err := r.Str(); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		size, err := r.readOptionalTag()
		if err != nil {
			return err
		}
		if !size {
			continue
		}
		// The size of the popup is in cells or in percent.
		if _, err := r.Variant(); err != nil {
			return err
		}
	}
	if err := skipFrameData(r); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		if _, err := r.Bool(); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := r.U32(); err != nil {
			return err
		}
	}
	return nil
}

// skipFrameData reads the grid of a frame.
func skipFrameData(r *Reader) error {
	cells, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < cells; i++ {
		if err := skipCell(r); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := r.U16(); err != nil {
			return err
		}
	}
	present, err := r.readOptionalTag()
	if err != nil {
		return err
	}
	if present {
		if _, err := r.U16(); err != nil {
			return err
		}
		if _, err := r.U16(); err != nil {
			return err
		}
		if _, err := r.Bool(); err != nil {
			return err
		}
		if _, err := r.U8(); err != nil {
			return err
		}
	}
	links, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < links; i++ {
		if _, err := r.Str(); err != nil {
			return err
		}
	}
	_, err = r.ByteSlice()
	return err
}

// skipCell reads one cell.
func skipCell(r *Reader) error {
	if _, err := r.Str(); err != nil {
		return err
	}
	if _, err := r.U32(); err != nil {
		return err
	}
	if _, err := r.U32(); err != nil {
		return err
	}
	if _, err := r.U16(); err != nil {
		return err
	}
	if _, err := r.Bool(); err != nil {
		return err
	}
	link, err := r.readOptionalTag()
	if err != nil {
		return err
	}
	if link {
		if _, err := r.U32(); err != nil {
			return err
		}
	}
	return nil
}

// skipGraphicsScene reads the graphics of the shell.
func skipGraphicsScene(r *Reader) error {
	assets, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < assets; i++ {
		if err := skipGraphicsKey(r); err != nil {
			return err
		}
		if _, err := r.ByteSlice(); err != nil {
			return err
		}
	}
	placements, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < placements; i++ {
		if err := skipGraphicsKey(r); err != nil {
			return err
		}
		if _, err := r.U32(); err != nil {
			return err
		}
		if _, err := r.U16(); err != nil {
			return err
		}
		if _, err := r.U16(); err != nil {
			return err
		}
		for j := 0; j < 6; j++ {
			if _, err := r.U32(); err != nil {
				return err
			}
		}
		if _, err := r.I32(); err != nil {
			return err
		}
		if _, err := r.U32(); err != nil {
			return err
		}
	}
	retained, err := r.unsigned()
	if err != nil {
		return err
	}
	for i := uint64(0); i < retained; i++ {
		if err := skipGraphicsKey(r); err != nil {
			return err
		}
	}
	return nil
}

// skipGraphicsKey reads the key of a picture.
func skipGraphicsKey(r *Reader) error {
	source, err := r.Variant()
	if err != nil {
		return err
	}
	switch source {
	case 0:
		// The picture comes from a terminal.
		target, err := r.Variant()
		if err != nil {
			return err
		}
		if _, err := r.Str(); err != nil {
			return err
		}
		if target == 0 {
			// The target is a pane.
			if _, err := r.Str(); err != nil {
				return err
			}
		}
		if _, err := r.U32(); err != nil {
			return err
		}
	case 1:
		// The picture comes from a layer of a pane.
		if _, err := r.Str(); err != nil {
			return err
		}
		if _, err := r.Str(); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		if _, err := r.U32(); err != nil {
			return err
		}
	}
	if _, err := r.Variant(); err != nil {
		return err
	}
	for i := 0; i < 2; i++ {
		if _, err := r.U64(); err != nil {
			return err
		}
	}
	return nil
}

// decodeCell reads one cell.
func decodeCell(r *Reader) (Cell, error) {
	var cell Cell
	var err error
	if cell.Symbol, err = r.Str(); err != nil {
		return cell, err
	}
	if cell.Fg, err = r.U32(); err != nil {
		return cell, err
	}
	if cell.Bg, err = r.U32(); err != nil {
		return cell, err
	}
	if cell.Modifier, err = r.U16(); err != nil {
		return cell, err
	}
	if cell.Skip, err = r.Bool(); err != nil {
		return cell, err
	}
	link, err := r.readOptionalTag()
	if err != nil {
		return cell, err
	}
	if link {
		value, err := r.U32()
		if err != nil {
			return cell, err
		}
		cell.Hyperlink = &value
	}
	return cell, nil
}

// decodeCursor reads the cursor.
func decodeCursor(r *Reader) (*Cursor, error) {
	present, err := r.readOptionalTag()
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	cursor := &Cursor{}
	if cursor.X, err = r.U16(); err != nil {
		return nil, err
	}
	if cursor.Y, err = r.U16(); err != nil {
		return nil, err
	}
	if cursor.Visible, err = r.Bool(); err != nil {
		return nil, err
	}
	if cursor.Shape, err = r.U8(); err != nil {
		return nil, err
	}
	return cursor, nil
}

// decodePaneSurface reads the place of one pane.
func decodePaneSurface(r *Reader) (PaneSurface, error) {
	var pane PaneSurface
	var err error
	if pane.PaneID, err = r.Str(); err != nil {
		return pane, err
	}
	if pane.ContentRevision, err = r.U64(); err != nil {
		return pane, err
	}
	if pane.Rect, err = decodeRect(r); err != nil {
		return pane, err
	}
	if pane.InnerRect, err = decodeRect(r); err != nil {
		return pane, err
	}
	bar, err := r.readOptionalTag()
	if err != nil {
		return pane, err
	}
	if bar {
		rect, err := decodeRect(r)
		if err != nil {
			return pane, err
		}
		pane.ScrollbarRect = &rect
	}
	// The scroll metrics of the pane say how far the pane is scrolled.
	scroll, err := r.readOptionalTag()
	if err != nil {
		return pane, err
	}
	if scroll {
		for i := 0; i < 3; i++ {
			if _, err := r.U64(); err != nil {
				return pane, err
			}
		}
	}
	if pane.Focused, err = r.Bool(); err != nil {
		return pane, err
	}
	if pane.MouseReporting, err = r.Bool(); err != nil {
		return pane, err
	}
	if _, err := r.Bool(); err != nil {
		return pane, err
	}
	if pane.AlternateScreenActive, err = r.Bool(); err != nil {
		return pane, err
	}
	if pane.PixelWidth, err = r.U32(); err != nil {
		return pane, err
	}
	if pane.PixelHeight, err = r.U32(); err != nil {
		return pane, err
	}
	return pane, nil
}

// decodeRect reads a place.
func decodeRect(r *Reader) (Rect, error) {
	var rect Rect
	var err error
	if rect.X, err = r.U16(); err != nil {
		return rect, err
	}
	if rect.Y, err = r.U16(); err != nil {
		return rect, err
	}
	if rect.Width, err = r.U16(); err != nil {
		return rect, err
	}
	if rect.Height, err = r.U16(); err != nil {
		return rect, err
	}
	return rect, nil
}

// readOptionalTag reads the tag of something that may be missing. It
// tells whether the thing is there.
func (r *Reader) readOptionalTag() (bool, error) {
	tag, err := r.U8()
	if err != nil {
		return false, err
	}
	switch tag {
	case 0:
		return false, nil
	case 1:
		return true, nil
	}
	return false, errBadOption
}

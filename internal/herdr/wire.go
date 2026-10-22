package herdr

import (
	"encoding/binary"
	"fmt"
	"io"

	"herdr-ui/internal/bincode"
)

// The wire model is bincode, framed by a little endian u32 that counts the
// bytes of the message. The variant numbers below are part of the contract
// with the daemon: they are the order the Rust model declares its variants
// in, and the daemon reads them the same way.

// Client message variants.
const (
	msgClientShellHello           = 11
	msgClientShellPaneInput       = 13
	msgClientShellEndpointRequest = 15
	msgClientShellFocus           = 18
	msgEndpointControl            = 20
)

// Server message variants.
const (
	msgServerWelcome                     = 0
	msgServerTerminalBell                = 9
	msgServerWindowTitle                 = 6
	msgServerClientShellSnapshot         = 12
	msgServerPaneSurface                 = 13
	msgServerSemanticNotification        = 14
	msgServerClientShellError            = 15
	msgServerClientShellEndpointResponse = 18
	msgServerEndpointControl             = 20
)

// Pane input event variants.
const (
	inputKey        = 0
	inputTextCommit = 1
	inputMouse      = 2
	inputPaste      = 3
)

// maxFrameSize bounds a single frame so a broken peer cannot make the client
// allocate without end.
const maxFrameSize = 64 << 20

// writeFrame writes one length prefixed message.
func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxFrameSize {
		return fmt.Errorf("herdr: frame of %d bytes is over the limit", len(payload))
	}
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// readFrame reads one length prefixed message.
func readFrame(r io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	size := binary.LittleEndian.Uint32(header[:])
	if size > maxFrameSize {
		return nil, fmt.Errorf("herdr: frame of %d bytes is over the limit", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// encodeEndpointControl builds the message that carries a named control.
func encodeEndpointControl(kind, data string) []byte {
	enc := bincode.NewEncoder()
	enc.Variant(msgEndpointControl)
	enc.String(kind)
	enc.String(data)
	return enc.Bytes()
}

// encodeEndpointRequest builds the message that carries one API request.
func encodeEndpointRequest(bootID, request string) []byte {
	enc := bincode.NewEncoder()
	enc.Variant(msgClientShellEndpointRequest)
	enc.String(bootID)
	enc.String(request)
	return enc.Bytes()
}

// encodeFocus builds the message that says whether the window has the
// keyboard: the daemon routes input to the pane of a focused client.
func encodeFocus(focused bool) []byte {
	enc := bincode.NewEncoder()
	enc.Variant(msgClientShellFocus)
	enc.Bool(focused)
	return enc.Bytes()
}

// encodePaneInput builds the message that carries input for a pane.
func encodePaneInput(paneID string, text string) []byte {
	enc := bincode.NewEncoder()
	enc.Variant(msgClientShellPaneInput)
	enc.String(paneID)
	enc.U64(1) // one event
	enc.Variant(inputTextCommit)
	enc.String(text)
	return enc.Bytes()
}

// The key codes the daemon knows, in the order the Rust model declares them.
const (
	KeyBackspace = 0
	KeyEnter     = 1
	KeyLeft      = 2
	KeyRight     = 3
	KeyUp        = 4
	KeyDown      = 5
	KeyHome      = 6
	KeyEnd       = 7
	KeyPageUp    = 8
	KeyPageDown  = 9
	KeyTab       = 10
	KeyBackTab   = 11
	KeyDelete    = 12
	KeyInsert    = 13
	KeyEsc       = 14
	KeyChar      = 15
	KeyFunction  = 16
	KeyNull      = 17
)

// The modifier bits the daemon reads.
const (
	KeyModShift   = 1
	KeyModControl = 2
	KeyModAlt     = 4
)

// The kinds of a key press.
const (
	keyKindPress   = 0
	keyKindRepeat  = 1
	keyKindRelease = 2
)

// PaneKey is one key press for a pane.
type PaneKey struct {
	// Code is the key the daemon knows.
	Code uint8
	// Value is the character a key types, or the number of a function key.
	Value uint32
	// HasValue says the key carries a value.
	HasValue bool
	// Modifiers are the bits the daemon reads: shift, control and alt.
	Modifiers uint8
	// Repeat says the key is held down.
	Repeat bool
	// RepeatCount is how many times the key repeated. Zero means once.
	RepeatCount uint16
}

// encodePaneKey builds the message that carries one key press for a pane.
//
// The layout is the one the Rust client writes: the tag of the message, the
// pane, one event, the tag of the key event, the code of the key, the
// character it types when it types one, the modifiers, the kind of the
// press, how many times it repeated, and then the four fields that are
// empty here.
func encodePaneKey(paneID string, key PaneKey) []byte {
	enc := bincode.NewEncoder()
	enc.Variant(msgClientShellPaneInput)
	enc.String(paneID)
	enc.U64(1) // one event
	enc.Variant(inputKey)
	enc.Variant(uint32(key.Code))
	switch key.Code {
	case KeyChar:
		enc.U32(key.Value)
	case KeyFunction:
		enc.U8(uint8(key.Value))
	}
	enc.U8(key.Modifiers)
	if key.Repeat {
		enc.Variant(keyKindRepeat)
	} else {
		enc.Variant(keyKindPress)
	}
	count := key.RepeatCount
	if count == 0 {
		count = 1
	}
	enc.U16(count) // repeat_count
	enc.Option(false)
	enc.Option(false)
	enc.Bool(false)
	enc.Option(false)
	enc.Option(false)
	return enc.Bytes()
}

// bincodeEncoder returns an encoder for one message.
func bincodeEncoder() *bincode.Encoder { return bincode.NewEncoder() }

// newPayloadDecoder returns a decoder over a message's fields, past the
// variant number that comes first.
func newPayloadDecoder(payload []byte) *bincode.Decoder {
	decoder := bincode.NewDecoder(payload)
	_ = decoder.Skip(1)
	return decoder
}

// Notification is what the daemon tells the client about.
type Notification struct {
	// Kind says what the notice is: needs attention, finished, an update
	// was installed, or something of its own.
	Kind string
	// Title is the head of the notice.
	Title string
	// Body is the rest of it.
	Body string
	// Agent is the agent the notice is about.
	Agent string
	// WorkspaceID, TabID and PaneID say where it happened.
	WorkspaceID string
	TabID       string
	PaneID      string
	// Position says where the notice is shown.
	Position string
}

// The kinds of a notice.
var notificationKinds = []string{"needs attention", "finished", "update installed", "custom"}

// The kinds of a sound.
var notificationSounds = []string{"done", "request"}

// The places a notice is shown.
var notificationPositions = []string{"top left", "top right", "bottom left", "bottom right"}

// decodeWindowTitle reads the title the daemon gives the window.
func decodeWindowTitle(payload []byte) (string, error) {
	dec := newPayloadDecoder(payload)
	title, err := optionalString(dec)
	return title, err
}

// decodeNotification reads what the daemon tells the client.
func decodeNotification(payload []byte) (Notification, error) {
	var notice Notification
	dec := newPayloadDecoder(payload)
	kind, err := dec.Variant()
	if err != nil {
		return notice, err
	}
	if int(kind) < len(notificationKinds) {
		notice.Kind = notificationKinds[kind]
	}
	if notice.Title, err = dec.String(); err != nil {
		return notice, err
	}
	if notice.Body, err = optionalString(dec); err != nil {
		return notice, err
	}
	sound, err := optionalVariant(dec)
	if err != nil {
		return notice, err
	}
	_ = sound
	if notice.Agent, err = optionalString(dec); err != nil {
		return notice, err
	}
	if notice.WorkspaceID, err = optionalString(dec); err != nil {
		return notice, err
	}
	if notice.TabID, err = optionalString(dec); err != nil {
		return notice, err
	}
	if notice.PaneID, err = optionalString(dec); err != nil {
		return notice, err
	}
	position, present, err := optionalVariantPresent(dec)
	if err != nil {
		return notice, err
	}
	if present && int(position) < len(notificationPositions) {
		notice.Position = notificationPositions[position]
	}
	return notice, nil
}

// decodeClientError reads what the daemon says went wrong.
func decodeClientError(payload []byte) (string, error) {
	dec := newPayloadDecoder(payload)
	return dec.String()
}

// decodeTerminalBell reads the bell of a pane.
//
// The payload is the number of times the bell rang.
func decodeTerminalBell(payload []byte) (uint16, error) {
	dec := newPayloadDecoder(payload)
	return dec.U16()
}

// optionalString reads a string that may be there.
func optionalString(dec *bincode.Decoder) (string, error) {
	present, err := dec.Option()
	if err != nil || !present {
		return "", err
	}
	return dec.String()
}

// optionalVariant reads the number of a variant that may be there.
func optionalVariant(dec *bincode.Decoder) (uint32, error) {
	value, _, err := optionalVariantPresent(dec)
	return value, err
}

// optionalVariantPresent reads the number of a variant that may be there, and
// says whether it was there at all.
func optionalVariantPresent(dec *bincode.Decoder) (uint32, bool, error) {
	present, err := dec.Option()
	if err != nil || !present {
		return 0, false, err
	}
	value, err := dec.Variant()
	return value, true, err
}

// endpointControl is the payload of a named control message.
type endpointControl struct {
	Kind string
	Data string
}

// decodeEndpointControl reads a named control payload.
func decodeEndpointControl(payload []byte) (endpointControl, error) {
	dec := newPayloadDecoder(payload)
	kind, err := dec.String()
	if err != nil {
		return endpointControl{}, err
	}
	data, err := dec.String()
	if err != nil {
		return endpointControl{}, err
	}
	return endpointControl{Kind: kind, Data: data}, nil
}

// responseChunk is one piece of an API answer.
type responseChunk struct {
	BootID     string
	RequestID  string
	FinalChunk bool
	Data       []byte
}

// decodeResponseChunk reads the answer to one API request.
func decodeResponseChunk(payload []byte) (responseChunk, error) {
	dec := newPayloadDecoder(payload)
	bootID, err := dec.String()
	if err != nil {
		return responseChunk{}, err
	}
	requestID, err := dec.String()
	if err != nil {
		return responseChunk{}, err
	}
	final, err := dec.Bool()
	if err != nil {
		return responseChunk{}, err
	}
	data, err := dec.BytesOf()
	if err != nil {
		return responseChunk{}, err
	}
	return responseChunk{BootID: bootID, RequestID: requestID, FinalChunk: final, Data: data}, nil
}

// Cell is one character of a terminal surface.
type Cell struct {
	Symbol   string
	FG       uint32
	BG       uint32
	Modifier uint16
	Skip     bool
	Link     *uint32
}

// Cursor says where the terminal's cursor sits.
type Cursor struct {
	X       uint16
	Y       uint16
	Visible bool
	Shape   uint8
}

// Grid is the character grid of a terminal surface.
type Grid struct {
	Cells       []Cell
	Width       uint16
	Height      uint16
	Cursor      *Cursor
	Hyperlinks  []string
	GraphicsLen int
}

// PaneSurface is one pane's place in a surface frame.
type PaneSurface struct {
	PaneID          string
	ContentRevision uint64
	Rect            SurfaceRect
	InnerRect       SurfaceRect
	ScrollbarRect   *SurfaceRect
	Scroll          *ScrollMetrics
	Focused         bool
	MouseReporting  bool
	SGRPixelMouse   bool
	AlternateScreen bool
	PixelWidth      uint32
	PixelHeight     uint32
}

// ScrollMetrics says how far a pane's view sits from the end of its output.
type ScrollMetrics struct {
	OffsetFromBottom    uint64
	MaxOffsetFromBottom uint64
	ViewportRows        uint64
}

// SurfaceRect is a place in the surface's cell grid.
type SurfaceRect struct {
	X      uint16
	Y      uint16
	Width  uint16
	Height uint16
}

// Split is a divider between two panes.
type Split struct {
	Horizontal bool
	Pos        uint16
	Area       SurfaceRect
	HitRect    SurfaceRect
}

// SurfaceFrame is one painted frame of the terminals on screen.
type SurfaceFrame struct {
	BootID             string
	ProjectionRevision uint64
	SurfaceRevision    uint64
	Grid               Grid
	Panes              []PaneSurface
	Splits             []Split
}

// decodeSurfaceFrame reads a surface frame.
//
// The frame ends with a graphics scene this client does not render, so the
// decoder stops once it has read the panes and the splits.
func decodeSurfaceFrame(payload []byte) (SurfaceFrame, error) {
	var frame SurfaceFrame
	dec := newPayloadDecoder(payload)
	var err error
	if frame.BootID, err = dec.String(); err != nil {
		return frame, err
	}
	if frame.ProjectionRevision, err = dec.U64(); err != nil {
		return frame, err
	}
	if frame.SurfaceRevision, err = dec.U64(); err != nil {
		return frame, err
	}
	if frame.Grid, err = decodeGrid(dec); err != nil {
		return frame, err
	}
	panes, err := dec.U64()
	if err != nil {
		return frame, err
	}
	for i := uint64(0); i < panes; i++ {
		pane, err := decodePaneSurface(dec)
		if err != nil {
			return frame, err
		}
		frame.Panes = append(frame.Panes, pane)
	}
	splits, err := dec.U64()
	if err != nil {
		return frame, err
	}
	for i := uint64(0); i < splits; i++ {
		split, err := decodeSplit(dec)
		if err != nil {
			return frame, err
		}
		frame.Splits = append(frame.Splits, split)
	}
	return frame, nil
}

func decodeGrid(dec *bincode.Decoder) (Grid, error) {
	var grid Grid
	cells, err := dec.U64()
	if err != nil {
		return grid, err
	}
	grid.Cells = make([]Cell, 0, cells)
	for i := uint64(0); i < cells; i++ {
		cell, err := decodeCell(dec)
		if err != nil {
			return grid, err
		}
		grid.Cells = append(grid.Cells, cell)
	}
	if grid.Width, err = dec.U16(); err != nil {
		return grid, err
	}
	if grid.Height, err = dec.U16(); err != nil {
		return grid, err
	}
	present, err := dec.Option()
	if err != nil {
		return grid, err
	}
	if present {
		cursor := Cursor{}
		if cursor.X, err = dec.U16(); err != nil {
			return grid, err
		}
		if cursor.Y, err = dec.U16(); err != nil {
			return grid, err
		}
		if cursor.Visible, err = dec.Bool(); err != nil {
			return grid, err
		}
		if cursor.Shape, err = dec.U8(); err != nil {
			return grid, err
		}
		grid.Cursor = &cursor
	}
	links, err := dec.U64()
	if err != nil {
		return grid, err
	}
	for i := uint64(0); i < links; i++ {
		link, err := dec.String()
		if err != nil {
			return grid, err
		}
		grid.Hyperlinks = append(grid.Hyperlinks, link)
	}
	graphics, err := dec.U64()
	if err != nil {
		return grid, err
	}
	grid.GraphicsLen = int(graphics)
	return grid, nil
}

func decodeCell(dec *bincode.Decoder) (Cell, error) {
	var cell Cell
	var err error
	if cell.Symbol, err = dec.String(); err != nil {
		return cell, err
	}
	if cell.FG, err = dec.U32(); err != nil {
		return cell, err
	}
	if cell.BG, err = dec.U32(); err != nil {
		return cell, err
	}
	if cell.Modifier, err = dec.U16(); err != nil {
		return cell, err
	}
	if cell.Skip, err = dec.Bool(); err != nil {
		return cell, err
	}
	link, err := dec.Option()
	if err != nil {
		return cell, err
	}
	if link {
		value, err := dec.U32()
		if err != nil {
			return cell, err
		}
		cell.Link = &value
	}
	return cell, nil
}

func decodePaneSurface(dec *bincode.Decoder) (PaneSurface, error) {
	var pane PaneSurface
	var err error
	if pane.PaneID, err = dec.String(); err != nil {
		return pane, err
	}
	if pane.ContentRevision, err = dec.U64(); err != nil {
		return pane, err
	}
	if pane.Rect, err = decodeRect(dec); err != nil {
		return pane, err
	}
	if pane.InnerRect, err = decodeRect(dec); err != nil {
		return pane, err
	}
	present, err := dec.Option()
	if err != nil {
		return pane, err
	}
	if present {
		rect, err := decodeRect(dec)
		if err != nil {
			return pane, err
		}
		pane.ScrollbarRect = &rect
	}
	present, err = dec.Option()
	if err != nil {
		return pane, err
	}
	if present {
		metrics := ScrollMetrics{}
		if metrics.OffsetFromBottom, err = dec.U64(); err != nil {
			return pane, err
		}
		if metrics.MaxOffsetFromBottom, err = dec.U64(); err != nil {
			return pane, err
		}
		if metrics.ViewportRows, err = dec.U64(); err != nil {
			return pane, err
		}
		pane.Scroll = &metrics
	}
	if pane.Focused, err = dec.Bool(); err != nil {
		return pane, err
	}
	if pane.MouseReporting, err = dec.Bool(); err != nil {
		return pane, err
	}
	if pane.SGRPixelMouse, err = dec.Bool(); err != nil {
		return pane, err
	}
	if pane.AlternateScreen, err = dec.Bool(); err != nil {
		return pane, err
	}
	if pane.PixelWidth, err = dec.U32(); err != nil {
		return pane, err
	}
	if pane.PixelHeight, err = dec.U32(); err != nil {
		return pane, err
	}
	return pane, nil
}

func decodeRect(dec *bincode.Decoder) (SurfaceRect, error) {
	var rect SurfaceRect
	var err error
	if rect.X, err = dec.U16(); err != nil {
		return rect, err
	}
	if rect.Y, err = dec.U16(); err != nil {
		return rect, err
	}
	if rect.Width, err = dec.U16(); err != nil {
		return rect, err
	}
	if rect.Height, err = dec.U16(); err != nil {
		return rect, err
	}
	return rect, nil
}

func decodeSplit(dec *bincode.Decoder) (Split, error) {
	var split Split
	direction, err := dec.Variant()
	if err != nil {
		return split, err
	}
	split.Horizontal = direction == 0
	if split.Pos, err = dec.U16(); err != nil {
		return split, err
	}
	if split.Area, err = decodeRect(dec); err != nil {
		return split, err
	}
	if split.HitRect, err = decodeRect(dec); err != nil {
		return split, err
	}
	path, err := dec.U64()
	if err != nil {
		return split, err
	}
	for i := uint64(0); i < path; i++ {
		if _, err := dec.Bool(); err != nil {
			return split, err
		}
	}
	return split, nil
}

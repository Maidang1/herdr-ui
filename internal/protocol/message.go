package protocol

import (
	"fmt"
)

// The cases of the messages the client sends. Their numbers are the
// numbers the daemon reads them by, and they do not change.
const (
	clientEndpointControl = 20
	clientShellPaneInput  = 13
	clientShellFocus      = 18
	clientShellResize     = 12
	clientShellHello      = 11
)

// The cases of the messages the daemon sends.
const (
	serverWelcome               = 0
	serverNotify                = 4
	serverWindowTitle           = 6
	serverMouseCapture          = 8
	serverTerminalBell          = 9
	serverClientShellSnapshot   = 12
	serverPaneSurface           = 13
	serverSemanticNotification  = 14
	serverClientShellError      = 15
	serverKeyboardReportAll     = 17
	serverEndpointResponseChunk = 18
	serverPaneSurfacePatch      = 19
	serverEndpointControl       = 20
)

// ClientMessage is a message the client sends to the daemon.
type ClientMessage interface {
	// encode writes the message.
	encode(w *Writer)
}

// EndpointControl is a named control message of the endpoint protocol.
//
// The handshake is one of these: the kind says what it is, and the data
// is JSON.
type EndpointControl struct {
	// Kind is the name of the control message.
	Kind string
	// Data is the JSON the control message carries.
	Data string
}

func (m *EndpointControl) encode(w *Writer) {
	w.Variant(clientEndpointControl)
	w.Str(m.Kind)
	w.Str(m.Data)
}

// EndpointRequest asks the daemon to do one thing.
//
// The request is JSON: an id, the name of the method, and its arguments.
type EndpointRequest struct {
	// BootID is the boot the request belongs to.
	BootID string
	// Request is the JSON of the request.
	Request string
}

func (m *EndpointRequest) encode(w *Writer) {
	w.Variant(15)
	w.Str(m.BootID)
	w.Str(m.Request)
}

// PaneInput is what the user does in a pane: keys, text, the mouse.
type PaneInput struct {
	// PaneID is the pane that receives it.
	PaneID string
	// Events are the things that happened, in order.
	Events []PaneInputEvent
}

func (m *PaneInput) encode(w *Writer) {
	w.Variant(clientShellPaneInput)
	w.Str(m.PaneID)
	w.unsigned(uint64(len(m.Events)))
	for _, event := range m.Events {
		event.encode(w)
	}
}

// ShellFocus tells the daemon whether the window that holds the shell has
// the keyboard.
type ShellFocus struct {
	// Focused tells whether the window has the keyboard.
	Focused bool
}

func (m *ShellFocus) encode(w *Writer) {
	w.Variant(clientShellFocus)
	w.Bool(m.Focused)
}

// ShellResize tells the daemon how big the pane viewport is.
type ShellResize struct {
	// CellWidth is the width of a cell in pixels.
	CellWidth uint32
	// CellHeight is the height of a cell in pixels.
	CellHeight uint32
	// Cols is the number of columns of the viewport.
	Cols uint16
	// Rows is the number of rows of the viewport.
	Rows uint16
	// PixelMouse tells whether the mouse is reported in pixels.
	PixelMouse bool
}

func (m *ShellResize) encode(w *Writer) {
	w.Variant(clientShellResize)
	w.U32(m.CellWidth)
	w.U32(m.CellHeight)
	w.U16(m.Cols)
	w.U16(m.Rows)
	w.Bool(m.PixelMouse)
}

// Encode writes the bytes of a message.
func Encode(msg ClientMessage) []byte {
	w := &Writer{}
	msg.encode(w)
	return w.Payload()
}

// PaneInputEvent is one thing the user did.
type PaneInputEvent interface {
	encode(w *Writer)
}

// The cases of the events of a pane.
const (
	inputEventKey        = 0
	inputEventTextCommit = 1
	inputEventMouse      = 2
	inputEventPaste      = 3
)

// The kinds of a key: pressed, held down, let go.
const (
	KeyPress   = 0
	KeyRepeat  = 1
	KeyRelease = 2
)

// The bits of the modifiers of a key.
const (
	ModShift = 1 << 0
	ModCtrl  = 1 << 1
	ModAlt   = 1 << 2
	ModSuper = 1 << 3
	ModHyper = 1 << 4
	ModMeta  = 1 << 5
)

// KeyEvent is a key that was pressed, held down or let go.
type KeyEvent struct {
	// Code is the key.
	Code KeyCode
	// Modifiers are the modifiers that were held down.
	Modifiers uint8
	// Kind tells whether the key was pressed, repeated or let go.
	Kind uint8
	// RepeatCount is how many times the key repeated.
	RepeatCount uint16
	// ShiftedCodepoint is the character the key would give with shift.
	ShiftedCodepoint *uint32
	// GeneratedText is the text the key produced, when it produced any.
	GeneratedText *string
	// TracksRelease tells whether the daemon wants to hear when the key
	// is let go.
	TracksRelease bool
}

func (e *KeyEvent) encode(w *Writer) {
	w.Variant(inputEventKey)
	e.Code.encode(w)
	w.U8(e.Modifiers)
	w.U8(e.Kind)
	w.U16(e.RepeatCount)
	writeOptionalUint32(w, e.ShiftedCodepoint)
	writeOptionalString(w, e.GeneratedText)
	w.Bool(e.TracksRelease)
	// The physical key and the record Windows sends are not used by the
	// client of this window.
	w.Variant(0)
	w.Variant(0)
}

// TextCommit is the text of an input method.
type TextCommit struct {
	// Text is what the input method gave.
	Text string
}

func (e *TextCommit) encode(w *Writer) {
	w.Variant(inputEventTextCommit)
	w.Str(e.Text)
}

// Paste is the text that was pasted.
type Paste struct {
	// Text is what was pasted.
	Text string
}

func (e *Paste) encode(w *Writer) {
	w.Variant(inputEventPaste)
	w.Str(e.Text)
}

// writeOptionalUint32 writes a number that may be missing.
func writeOptionalUint32(w *Writer, v *uint32) {
	if v == nil {
		w.U8(0)
		return
	}
	w.U8(1)
	w.U32(*v)
}

// writeOptionalString writes a string that may be missing.
func writeOptionalString(w *Writer, v *string) {
	if v == nil {
		w.U8(0)
		return
	}
	w.U8(1)
	w.Str(*v)
}

// The cases of a key.
const (
	keyBackspace = 0
	keyEnter     = 1
	keyLeft      = 2
	keyRight     = 3
	keyUp        = 4
	keyDown      = 5
	keyHome      = 6
	keyEnd       = 7
	keyPageUp    = 8
	keyPageDown  = 9
	keyTab       = 10
	keyBackTab   = 11
	keyDelete    = 12
	keyInsert    = 13
	keyEsc       = 14
	keyChar      = 15
	keyF         = 16
	keyNull      = 17
)

// KeyCode is the key that was pressed.
type KeyCode struct {
	// Kind is which key it is: one of the key constants, or keyChar.
	Kind uint8
	// Char is the character, when the kind is keyChar.
	Char rune
	// Number is the number of the function key, when the kind is keyF.
	Number uint8
}

// The keys that are not characters.
var (
	KeyBackspace = KeyCode{Kind: keyBackspace}
	KeyEnter     = KeyCode{Kind: keyEnter}
	KeyLeft      = KeyCode{Kind: keyLeft}
	KeyRight     = KeyCode{Kind: keyRight}
	KeyUp        = KeyCode{Kind: keyUp}
	KeyDown      = KeyCode{Kind: keyDown}
	KeyHome      = KeyCode{Kind: keyHome}
	KeyEnd       = KeyCode{Kind: keyEnd}
	KeyPageUp    = KeyCode{Kind: keyPageUp}
	KeyPageDown  = KeyCode{Kind: keyPageDown}
	KeyTab       = KeyCode{Kind: keyTab}
	KeyBackTab   = KeyCode{Kind: keyBackTab}
	KeyDelete    = KeyCode{Kind: keyDelete}
	KeyInsert    = KeyCode{Kind: keyInsert}
	KeyEsc       = KeyCode{Kind: keyEsc}
	KeyNull      = KeyCode{Kind: keyNull}
)

// CharKey is the key of a character.
func CharKey(r rune) KeyCode { return KeyCode{Kind: keyChar, Char: r} }

// FunctionKey is the key of a function key.
func FunctionKey(n uint8) KeyCode { return KeyCode{Kind: keyF, Number: n} }

func (k KeyCode) encode(w *Writer) {
	w.Variant(uint32(k.Kind))
	switch k.Kind {
	case keyChar:
		w.Rune(k.Char)
	case keyF:
		w.U8(k.Number)
	}
}

// decodeKeyCode reads a key.
func decodeKeyCode(r *Reader) (KeyCode, error) {
	kind, err := r.Variant()
	if err != nil {
		return KeyCode{}, err
	}
	switch kind {
	case keyChar:
		c, err := r.Rune()
		if err != nil {
			return KeyCode{}, err
		}
		return CharKey(c), nil
	case keyF:
		n, err := r.U8()
		if err != nil {
			return KeyCode{}, err
		}
		return FunctionKey(n), nil
	case keyBackspace, keyEnter, keyLeft, keyRight, keyUp, keyDown, keyHome,
		keyEnd, keyPageUp, keyPageDown, keyTab, keyBackTab, keyDelete,
		keyInsert, keyEsc, keyNull:
		return KeyCode{Kind: uint8(kind)}, nil
	}
	return KeyCode{}, fmt.Errorf("protocol: %d is not a key the daemon knows", kind)
}

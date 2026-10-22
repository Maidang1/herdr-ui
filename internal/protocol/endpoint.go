package protocol

import "encoding/json"

// The handshake of the endpoint protocol.
//
// The client says hello in a named control message whose data is JSON, and
// the daemon answers in one of its own. The answer says which methods the
// client may call and which codecs both sides speak.

// The codecs of the endpoint protocol.
const (
	// CodecSnapshot is the codec of the snapshot of the session.
	CodecSnapshot = "shell.snapshot.v1"
	// CodecSurface is the codec of the surface of a pane.
	CodecSurface = "shell.surface.v1"
	// CodecInput is the codec of the input of a pane.
	CodecInput = "shell.input.semantic.v1"
	// CodecBlob is the codec of the pieces of files.
	CodecBlob = "shell.blob.v1"
)

// SurfaceSize is the size of the viewport of a pane.
type SurfaceSize struct {
	// Cols is the number of columns.
	Cols uint16 `json:"cols"`
	// Rows is the number of rows.
	Rows uint16 `json:"rows"`
}

// ClientHello is what the client says when it connects.
//
// It says how big a cell is, how big the viewport is, and which codecs the
// client speaks.
type ClientHello struct {
	// Generation is the generation of the protocol.
	Generation uint32 `json:"generation"`
	// CellWidthPx is the width of a cell in pixels.
	CellWidthPx uint32 `json:"cell_width_px"`
	// CellHeightPx is the height of a cell in pixels.
	CellHeightPx uint32 `json:"cell_height_px"`
	// SurfaceSize is the size of the viewport.
	SurfaceSize SurfaceSize `json:"surface_size"`
	// PixelMouse tells that the mouse is reported in pixels.
	PixelMouse bool `json:"pixel_mouse"`
	// DirectGraphics tells that the client draws the graphics itself.
	DirectGraphics bool `json:"direct_graphics"`
	// EndpointKeybindings tells that the daemon's keymap owns the keys
	// of the shell.
	EndpointKeybindings bool `json:"endpoint_keybindings"`
	// MouseCapture tells that the client wants the mouse of the shell
	// even when no pane asks for it.
	MouseCapture bool `json:"mouse_capture"`
	// SurfaceActive tells that the client wants the surface.
	SurfaceActive bool `json:"surface_active"`
	// SnapshotCodecs are the codecs of the snapshot the client speaks.
	SnapshotCodecs []string `json:"snapshot_codecs"`
	// SurfaceCodecs are the codecs of the surface the client speaks.
	SurfaceCodecs []string `json:"surface_codecs"`
	// InputCodecs are the codecs of the input the client speaks.
	InputCodecs []string `json:"input_codecs"`
	// BlobCodecs are the codecs of the pieces of files the client speaks.
	BlobCodecs []string `json:"blob_codecs"`
}

// DefaultHello returns the handshake of a client that draws a viewport of
// the given size, with cells of the given size.
func DefaultHello(cols, rows uint16, cellWidth, cellHeight uint32) ClientHello {
	return ClientHello{
		Generation:    1,
		CellWidthPx:   cellWidth,
		CellHeightPx:  cellHeight,
		SurfaceSize:   SurfaceSize{Cols: cols, Rows: rows},
		SurfaceActive: true,
		SnapshotCodecs: []string{
			CodecSnapshot,
		},
		SurfaceCodecs: []string{
			CodecSurface,
		},
		InputCodecs: []string{
			CodecInput,
		},
		BlobCodecs: []string{
			CodecBlob,
		},
	}
}

// JSON writes the handshake as the JSON the daemon reads.
func (h ClientHello) JSON() (string, error) {
	data, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ServerWelcome is what the daemon answers to the handshake.
type ServerWelcome struct {
	// Generation is the generation of the protocol.
	Generation uint32 `json:"generation"`
	// ServerVersion is the version of the daemon.
	ServerVersion string `json:"server_version"`
	// SnapshotCodec is the codec of the snapshot the daemon speaks.
	SnapshotCodec string `json:"snapshot_codec"`
	// SurfaceCodec is the codec of the surface the daemon speaks.
	SurfaceCodec string `json:"surface_codec"`
	// InputCodec is the codec of the input the daemon speaks.
	InputCodec string `json:"input_codec"`
	// BlobCodec is the codec of the pieces of files the daemon speaks.
	BlobCodec string `json:"blob_codec"`
	// Methods are the methods the client may call.
	Methods []string `json:"methods"`
	// Capabilities are what the daemon can do.
	Capabilities []string `json:"capabilities"`
	// Error is why the daemon refused the client, when it refused it.
	Error *WelcomeError `json:"error"`
}

// WelcomeError is why the daemon refused the client.
type WelcomeError struct {
	// Kind is the sort of the refusal.
	Kind string `json:"kind"`
	// Message is what the refusal says.
	Message string `json:"message"`
}

// DecodeWelcome reads the answer of the daemon.
func DecodeWelcome(data string) (*ServerWelcome, error) {
	welcome := &ServerWelcome{}
	if err := json.Unmarshal([]byte(data), welcome); err != nil {
		return nil, err
	}
	return welcome, nil
}

// HasMethod tells whether the daemon lets the client call a method.
func (w *ServerWelcome) HasMethod(name string) bool {
	for _, method := range w.Methods {
		if method == name {
			return true
		}
	}
	return false
}

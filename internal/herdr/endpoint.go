package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// The endpoint protocol is how a GUI client talks to the Herdr daemon. The
// daemon owns the terminals and the session; the client sends its wishes
// over one connection and the daemon answers with snapshots and painted
// surfaces.
//
// A frame is bincode behind a little endian u32 length. The first frame a
// client sends is a named hello, and the daemon answers with a welcome that
// lists the methods it serves.

// EndpointProtocolGeneration is the generation of the endpoint protocol.
const EndpointProtocolGeneration = 1

// Codec names of the protocol generation this client speaks.
const (
	SnapshotCodec = "shell.snapshot.v1"
	SurfaceCodec  = "shell.surface.v1"
	InputCodec    = "shell.input.semantic.v1"
	BlobCodec     = "shell.blob.v1"
)

// ErrClosed means the connection to the daemon ended.
var ErrClosed = errors.New("herdr: the connection to the daemon ended")

// APIError is the error the daemon answers a refused request with.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return "herdr: " + e.Code
	}
	return "herdr: " + e.Code + ": " + e.Message
}

// apiEnvelope is the shape the daemon answers a request with.
type apiEnvelope struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *APIError       `json:"error"`
}

// EndpointOptions are the settings a client offers the daemon.
type EndpointOptions struct {
	// SocketPath is the daemon's client socket. Empty means the socket the
	// environment points at.
	SocketPath string
	// CellWidthPx and CellHeightPx are the size of one character cell.
	CellWidthPx  uint32
	CellHeightPx uint32
	// Cols and Rows are how many cells the client shows.
	Cols uint16
	Rows uint16
	// SurfaceActive asks the daemon to paint the terminals.
	SurfaceActive bool
}

// Endpoint is a live connection to the daemon.
// stream is what the endpoint speaks over: a connection that can be given
// a deadline for writing.
type stream interface {
	io.Reader
	io.Writer
	io.Closer
	SetWriteDeadline(time.Time) error
}

type Endpoint struct {
	conn    stream
	writeMu sync.Mutex
	// socket is where the daemon listens, for the times the connection
	// has to be opened again.
	socket string
	// dial opens the connection again, when it has to.
	dial func() (stream, error)
	// opts are the settings of the handshake.
	opts EndpointOptions

	mu      sync.Mutex
	pending map[string]*pendingRequest
	bootID  string
	err     error
	// outage is the break in the connection, when there is one.
	outage *Outage
	// focused says this client holds the keyboard.
	focused bool

	seq atomic.Uint64

	closeOnce     sync.Once
	greeting      *Welcome
	welcome       chan *Welcome
	snapshot      chan *Snapshot
	surface       chan *SurfaceFrame
	titles        chan string
	notifications chan *Notification
	errors        chan string
	bells         chan uint16
	done          chan struct{}
	reconnectNow  chan struct{}
	outageSignal  chan struct{}
}

// Welcome is the daemon's answer to the client's hello.
type Welcome struct {
	Generation    uint32   `json:"generation"`
	ServerVersion string   `json:"server_version"`
	SnapshotCodec string   `json:"snapshot_codec"`
	SurfaceCodec  string   `json:"surface_codec"`
	InputCodec    string   `json:"input_codec"`
	BlobCodec     string   `json:"blob_codec"`
	Methods       []string `json:"methods"`
	Capabilities  []string `json:"capabilities"`
	Error         *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// HasMethod reports whether the daemon serves this method.
func (w *Welcome) HasMethod(method string) bool {
	for _, name := range w.Methods {
		if name == method {
			return true
		}
	}
	return false
}

type pendingRequest struct {
	ch chan []byte
}

// ClientSocketPath returns where the daemon's client socket is.
//
// The client socket sits beside the daemon's own socket: HERDR_CLIENT_SOCKET_PATH
// wins, then the socket beside HERDR_SOCKET_PATH, then the one the daemon
// keeps under its configuration directory.
func ClientSocketPath() (string, error) {
	if path := os.Getenv("HERDR_CLIENT_SOCKET_PATH"); path != "" {
		return path, nil
	}
	if path := os.Getenv("HERDR_SOCKET_PATH"); path != "" {
		return siblingClientSocket(path), nil
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "herdr", "herdr-client.sock"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "herdr", "herdr-client.sock"), nil
}

func siblingClientSocket(socket string) string {
	dir := filepath.Dir(socket)
	stem := filepath.Base(socket)
	stem = stem[:len(stem)-len(filepath.Ext(stem))]
	return filepath.Join(dir, stem+"-client.sock")
}

// Connect opens a connection to the daemon and says hello.
func Connect(ctx context.Context, opts EndpointOptions) (*Endpoint, error) {
	socket := opts.SocketPath
	if socket == "" {
		path, err := ClientSocketPath()
		if err != nil {
			return nil, err
		}
		socket = path
	}
	dial := func() (stream, error) {
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "unix", socket)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("herdr: connect to %s: %w", socket, err)
	}
	return connectStream(ctx, opts, conn, dial, socket)
}

// connectStream says hello on a connection and waits for the answer.
func connectStream(ctx context.Context, opts EndpointOptions, conn stream, dial func() (stream, error), label string) (*Endpoint, error) {
	endpoint := &Endpoint{
		conn:          conn,
		socket:        label,
		dial:          dial,
		opts:          opts,
		pending:       map[string]*pendingRequest{},
		welcome:       make(chan *Welcome, 1),
		snapshot:      make(chan *Snapshot, 8),
		surface:       make(chan *SurfaceFrame, 4),
		titles:        make(chan string, 4),
		notifications: make(chan *Notification, 8),
		errors:        make(chan string, 4),
		bells:         make(chan uint16, 4),
		done:          make(chan struct{}),
		reconnectNow:  make(chan struct{}, 1),
		outageSignal:  make(chan struct{}, 1),
	}

	hello, err := endpoint.helloPayload(opts)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if err := endpoint.write(encodeEndpointControl("endpoint.hello.v1", hello)); err != nil {
		conn.Close()
		return nil, err
	}

	go endpoint.readLoop()

	select {
	case welcome := <-endpoint.welcome:
		if welcome.Error != nil {
			endpoint.Close()
			return nil, fmt.Errorf("herdr: the daemon refused the handshake: %s", welcome.Error.Message)
		}
		endpoint.greeting = welcome
		return endpoint, nil
	case <-ctx.Done():
		endpoint.Close()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		reason := endpoint.readError()
		endpoint.Close()
		return nil, fmt.Errorf("herdr: the daemon did not answer the handshake (%v)", reason)
	}
}

func (e *Endpoint) helloPayload(opts EndpointOptions) (string, error) {
	hello := map[string]any{
		"generation":           EndpointProtocolGeneration,
		"cell_width_px":        opts.CellWidthPx,
		"cell_height_px":       opts.CellHeightPx,
		"surface_size":         map[string]any{"cols": opts.Cols, "rows": opts.Rows},
		"pixel_mouse":          false,
		"direct_graphics":      false,
		"endpoint_keybindings": false,
		"mouse_capture":        false,
		"surface_active":       opts.SurfaceActive,
		"surface_reuse":        true,
		"surface_delta":        true,
		"surface_scroll":       true,
		"snapshot_codecs":      []string{SnapshotCodec},
		"surface_codecs":       []string{SurfaceCodec},
		"input_codecs":         []string{InputCodec},
		"blob_codecs":          []string{BlobCodec},
	}
	raw, err := json.Marshal(hello)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (e *Endpoint) write(payload []byte) error {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := e.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return writeFrame(e.conn, payload)
}

// Welcome returns the daemon's welcome, once the handshake is done.
func (e *Endpoint) Welcome() *Welcome {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.greeting
}

// Snapshots returns the stream of session snapshots.
func (e *Endpoint) Snapshots() <-chan *Snapshot { return e.snapshot }

// Surfaces returns the stream of painted terminal surfaces.
func (e *Endpoint) Surfaces() <-chan *SurfaceFrame { return e.surface }

// Titles returns the titles the daemon gives the window.
func (e *Endpoint) Titles() <-chan string { return e.titles }

// Notifications returns what the daemon tells the client about.
func (e *Endpoint) Notifications() <-chan *Notification { return e.notifications }

// Errors returns what the daemon says went wrong.
func (e *Endpoint) Errors() <-chan string { return e.errors }

// Bells returns the rings of the terminals.
//
// The daemon forwards the bell of a pane to the client that holds the
// keyboard, the way an outer terminal would see it.
func (e *Endpoint) Bells() <-chan uint16 { return e.bells }

// BootID returns the identifier of the running session.
func (e *Endpoint) BootID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.bootID
}

// Close ends the connection.
func (e *Endpoint) Close() error {
	e.closeOnce.Do(func() { close(e.done) })
	return e.conn.Close()
}

func (e *Endpoint) readLoop() {
	defer e.closeOnce.Do(func() { close(e.done) })
	for {
		payload, err := readFrame(e.conn)
		if err != nil {
			e.fail(err)
			e.setOutage(err)
			if !e.reconnect() {
				return
			}
			continue
		}
		e.handle(payload)
	}
}

// Outage is a break in the connection to the daemon.
type Outage struct {
	// Reason says why the connection ended.
	Reason string
	// Since is when it began.
	Since time.Time
}

// Outage returns the break in the connection, when there is one.
func (e *Endpoint) Outage() *Outage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.outage
}

// OutageChanged is told when the connection breaks or comes back.
func (e *Endpoint) OutageChanged() <-chan struct{} {
	return e.outageSignal
}

// setOutage says the connection is broken.
func (e *Endpoint) setOutage(err error) {
	e.mu.Lock()
	if e.outage == nil {
		e.outage = &Outage{Reason: err.Error(), Since: time.Now()}
	}
	e.mu.Unlock()
	e.signalOutage()
}

// clearOutage says the connection is back.
func (e *Endpoint) clearOutage() {
	e.mu.Lock()
	e.outage = nil
	e.mu.Unlock()
	e.signalOutage()
}

// signalOutage tells the window that the connection changed.
func (e *Endpoint) signalOutage() {
	select {
	case e.outageSignal <- struct{}{}:
	default:
	}
}

// ReconnectNow asks the endpoint to try again at once.
func (e *Endpoint) ReconnectNow() {
	select {
	case e.reconnectNow <- struct{}{}:
	default:
	}
}

// reconnect opens the connection again, waiting a little longer each time.
//
// The daemon may be restarting, or the machine may have been asleep; the
// window stays on the endpoint and the picture of the session is kept.
func (e *Endpoint) reconnect() bool {
	backoff := 300 * time.Millisecond
	for attempt := 0; attempt < 24; attempt++ {
		timer := time.NewTimer(backoff)
		select {
		case <-e.done:
			timer.Stop()
			return false
		case <-e.reconnectNow:
			timer.Stop()
		case <-timer.C:
		}
		conn, err := e.dial()
		if err != nil {
			if backoff < 5*time.Second {
				backoff *= 2
			}
			continue
		}
		if err := e.handshake(conn); err != nil {
			conn.Close()
			continue
		}
		e.writeMu.Lock()
		e.conn = conn
		e.writeMu.Unlock()
		e.clearOutage()
		// The daemon does not know that this client holds the keyboard
		// on a connection it has not seen before.
		e.mu.Lock()
		focused := e.focused
		e.mu.Unlock()
		if focused {
			_ = e.write(encodeFocus(true))
		}
		return true
	}
	return false
}

// handshake says hello on a fresh connection.
func (e *Endpoint) handshake(conn stream) error {
	hello, err := e.helloPayload(e.opts)
	if err != nil {
		return err
	}
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	return writeFrame(conn, encodeEndpointControl("endpoint.hello.v1", hello))
}

// readError returns why the connection to the daemon ended, if it did.
func (e *Endpoint) readError() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}

func (e *Endpoint) fail(err error) {
	e.mu.Lock()
	if e.err == nil {
		e.err = err
	}
	pending := e.pending
	e.pending = map[string]*pendingRequest{}
	e.mu.Unlock()
	for _, request := range pending {
		close(request.ch)
	}
}

func (e *Endpoint) handle(payload []byte) {
	dec := newVariantReader(payload)
	variant, ok := dec.variant()
	if !ok {
		return
	}
	switch variant {
	case msgServerEndpointControl:
		control, err := decodeEndpointControl(payload)
		if err != nil {
			return
		}
		e.handleControl(control)
	case msgServerClientShellEndpointResponse:
		chunk, err := decodeResponseChunk(payload)
		if err != nil {
			return
		}
		e.handleResponse(chunk)
	case msgServerPaneSurface:
		frame, err := decodeSurfaceFrame(payload)
		if err != nil {
			return
		}
		select {
		case e.surface <- &frame:
		default:
		}
	case msgServerWindowTitle:
		title, err := decodeWindowTitle(payload)
		if err != nil {
			return
		}
		select {
		case e.titles <- title:
		default:
		}
	case msgServerSemanticNotification:
		notice, err := decodeNotification(payload)
		if err != nil {
			return
		}
		select {
		case e.notifications <- &notice:
		default:
		}
	case msgServerClientShellError:
		message, err := decodeClientError(payload)
		if err != nil {
			return
		}
		select {
		case e.errors <- message:
		default:
		}
	case msgServerTerminalBell:
		count, err := decodeTerminalBell(payload)
		if err != nil {
			return
		}
		select {
		case e.bells <- count:
		default:
		}
	default:
		// Other variants carry things this client does not read yet: the
		// welcome of the terminal protocol, the binary snapshot of an
		// earlier generation, notifications.
	}
}

func (e *Endpoint) handleControl(control endpointControl) {
	switch control.Kind {
	case "endpoint.welcome.v1":
		var welcome Welcome
		if err := json.Unmarshal([]byte(control.Data), &welcome); err != nil {
			return
		}
		e.mu.Lock()
		e.greeting = &welcome
		e.mu.Unlock()
		select {
		case e.welcome <- &welcome:
		default:
		}
	case SnapshotCodec:
		var snapshot Snapshot
		if err := json.Unmarshal([]byte(control.Data), &snapshot); err != nil {
			return
		}
		e.mu.Lock()
		e.bootID = snapshot.BootID
		e.mu.Unlock()
		select {
		case e.snapshot <- &snapshot:
		default:
		}
	}
}

func (e *Endpoint) handleResponse(chunk responseChunk) {
	e.mu.Lock()
	request, ok := e.pending[chunk.RequestID]
	if ok {
		delete(e.pending, chunk.RequestID)
	}
	e.mu.Unlock()
	if !ok {
		return
	}
	if chunk.FinalChunk {
		if len(chunk.Data) > 0 {
			select {
			case request.ch <- chunk.Data:
			default:
			}
		}
		close(request.ch)
		return
	}
	// A chunk that is not the last one is followed by more; the daemon
	// splits large answers, so keep them together.
	select {
	case request.ch <- chunk.Data:
	default:
	}
}

// Request asks the daemon to do something and waits for its answer.
//
// The daemon answers with one or more chunks; the last one closes the
// exchange. The answer keeps the shape the daemon sent.
func (e *Endpoint) Request(ctx context.Context, method string, params, out any) error {
	raw, err := e.RequestRaw(ctx, method, params)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// RequestRaw asks the daemon to do something and returns its answer.
func (e *Endpoint) RequestRaw(ctx context.Context, method string, params any) (json.RawMessage, error) {
	bootID := e.BootID()
	if bootID == "" {
		return nil, errors.New("herdr: the daemon has not sent a snapshot yet")
	}
	body, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 || string(body) == "null" {
		body = []byte("{}")
	}
	id := fmt.Sprintf("herdr-ui-%d", e.seq.Add(1))
	envelope, err := json.Marshal(map[string]any{
		"id":     id,
		"method": method,
		"params": json.RawMessage(body),
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan []byte, 8)
	e.mu.Lock()
	e.pending[id] = &pendingRequest{ch: ch}
	e.mu.Unlock()

	if err := e.write(encodeEndpointRequest(bootID, string(envelope))); err != nil {
		e.mu.Lock()
		delete(e.pending, id)
		e.mu.Unlock()
		return nil, err
	}

	var answer []byte
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				if len(answer) == 0 {
					return nil, ErrClosed
				}
				return parseAnswer(answer)
			}
			answer = append(answer, chunk...)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// parseAnswer unwraps the daemon's answer envelope.
func parseAnswer(answer []byte) (json.RawMessage, error) {
	var envelope apiEnvelope
	if err := json.Unmarshal(answer, &envelope); err != nil {
		return nil, fmt.Errorf("herdr: %w", err)
	}
	if envelope.Error != nil {
		return nil, envelope.Error
	}
	return envelope.Result, nil
}

// Focus tells the daemon whether this client has the keyboard. The daemon
// routes input to the pane of a client that has it.
//
// The answer is kept, so that a connection opened again asks for the
// keyboard again too.
func (e *Endpoint) Focus(focused bool) error {
	e.mu.Lock()
	e.focused = focused
	e.mu.Unlock()
	return e.write(encodeFocus(focused))
}

// SendText types text into a pane.
func (e *Endpoint) SendText(paneID, text string) error {
	return e.write(encodePaneInput(paneID, text))
}

// SendKey presses one key in a pane.
func (e *Endpoint) SendKey(paneID string, key PaneKey) error {
	return e.write(encodePaneKey(paneID, key))
}

// variantReader reads the variant number of a message.
type variantReader struct {
	data []byte
	pos  int
}

func newVariantReader(data []byte) *variantReader { return &variantReader{data: data} }

func (r *variantReader) variant() (uint32, bool) {
	value := uint32(0)
	shift := uint(0)
	for {
		if r.pos >= len(r.data) {
			return 0, false
		}
		b := r.data[r.pos]
		r.pos++
		value |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, true
		}
		shift += 7
		if shift > 28 {
			return 0, false
		}
	}
}

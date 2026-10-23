package endpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

type Command struct {
	CommandID     string   `json:"command_id"`
	BindingLabel  string   `json:"binding_label"`
	BindingLabels []string `json:"binding_labels"`
	Action        string   `json:"action"`
	Description   string   `json:"description"`
}
type Snapshot struct {
	BootID                string    `json:"boot_id"`
	Revision              uint64    `json:"revision"`
	FocusedPaneID         string    `json:"focused_pane_id"`
	Commands              []Command `json:"commands"`
	ServerKeybindingsTOML string    `json:"server_keybindings_toml"`
}
type Notification struct {
	ID                                                                    uint64
	Kind, Title, Body, Sound, Agent, WorkspaceID, TabID, PaneID, Position string
	ReceivedAt                                                            time.Time
}
type State struct {
	Connected     bool
	Snapshot      *Snapshot
	Notifications []Notification // Bounded last 64 events; ID allows caller deduplication/expiry.
	Err           error
}
type welcome struct {
	Generation    int      `json:"generation"`
	SnapshotCodec string   `json:"snapshot_codec"`
	SurfaceCodec  string   `json:"surface_codec"`
	InputCodec    string   `json:"input_codec"`
	BlobCodec     string   `json:"blob_codec"`
	Methods       []string `json:"methods"`
	Capabilities  []string `json:"capabilities"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error"`
}
type response struct {
	data []byte
	err  error
}
type pending struct {
	done chan response
	data []byte
}
type Client struct {
	changed         func()
	errorSequence   uint64
	socket          string
	mu              sync.Mutex
	writeMu         sync.Mutex
	imageMu         sync.Mutex
	conn            net.Conn
	cancel          context.CancelFunc
	started, closed bool
	state           State
	welcome         welcome
	pending         map[string]*pending
	sequence        uint64
	cols, rows      uint16
}

// SocketPath follows Herdr server/socket_paths.rs, including non-.sock extensions.
func SocketPath(apiSocket string) string {
	base := filepath.Base(apiSocket)
	ext := filepath.Ext(base)
	if ext != "" && ext != base {
		base = strings.TrimSuffix(base, ext)
	}
	return filepath.Join(filepath.Dir(apiSocket), base+"-client.sock")
}

// SetChanged installs a notification hook, invoked without client locks. The
// hook should schedule UI work and return promptly; it may call Latest.
func (c *Client) SetChanged(changed func()) { c.mu.Lock(); c.changed = changed; c.mu.Unlock() }
func (c *Client) notify() {
	c.mu.Lock()
	changed := c.changed
	c.mu.Unlock()
	if changed != nil {
		changed()
	}
}

func New(apiSocket string) *Client { return NewSocket(SocketPath(apiSocket)) }
func NewSocket(clientSocket string) *Client {
	return &Client{socket: clientSocket, pending: make(map[string]*pending)}
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started || c.closed {
		c.mu.Unlock()
		return errors.New("endpoint already started or closed")
	}
	c.started = true
	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.mu.Unlock()
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(runCtx, "unix", c.socket)
	if err != nil {
		c.fail(err)
		return err
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		conn.Close()
		return net.ErrClosed
	}
	c.conn = conn
	c.mu.Unlock()
	go func() { <-runCtx.Done(); c.Close() }()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	hello := map[string]any{
		"generation": 1, "cell_width_px": 0, "cell_height_px": 0,
		"surface_size": map[string]int{"cols": 80, "rows": 24},
		"pixel_mouse":  false, "direct_graphics": false, "endpoint_keybindings": false,
		"mouse_capture": false, "surface_active": false,
		"snapshot_codecs": []string{"shell.snapshot.v1"}, "surface_codecs": []string{"shell.surface.v1"},
		"input_codecs": []string{"shell.input.semantic.v1"}, "blob_codecs": []string{"shell.blob.v1"},
	}
	encoded, _ := json.Marshal(hello)
	if err = writeFrame(conn, control("endpoint.hello.v1", string(encoded))); err != nil {
		c.fail(err)
		return err
	}
	var negotiated welcome
	var b []byte
	b, err = readFrame(conn)
	if err == nil {
		d := decoder{b: b}
		if d.uint() != controlTag || d.str() != "endpoint.welcome.v1" {
			err = errors.New("server does not support endpoint generation 1")
		} else {
			data := d.str()
			err = d.finish()
			if err == nil {
				err = json.Unmarshal([]byte(data), &negotiated)
			}
		}
	}
	if err == nil {
		w := negotiated
		if w.Error != nil {
			err = errors.New(w.Error.Message)
		} else if w.Generation != 1 || w.SnapshotCodec != "shell.snapshot.v1" || w.SurfaceCodec != "shell.surface.v1" || w.InputCodec != "shell.input.semantic.v1" || w.BlobCodec != "shell.blob.v1" {
			err = errors.New("incompatible endpoint core codecs")
		}
	}
	if err != nil {
		c.fail(err)
		return err
	}
	conn.SetDeadline(time.Time{})
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return net.ErrClosed
	}
	c.welcome = negotiated
	c.state.Connected = true
	c.mu.Unlock()
	go c.readLoop(conn)
	c.notify()
	return nil
}
func (c *Client) Latest() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Notifications = slices.Clone(s.Notifications)
	if s.Snapshot != nil {
		v := *s.Snapshot
		v.Commands = slices.Clone(v.Commands)
		for i := range v.Commands {
			v.Commands[i].BindingLabels = slices.Clone(v.Commands[i].BindingLabels)
		}
		s.Snapshot = &v
	}
	return s
}
func (c *Client) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.state.Connected = false
	conn, cancel := c.conn, c.cancel
	for id, p := range c.pending {
		p.done <- response{err: net.ErrClosed}
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	var err error
	if conn != nil {
		err = conn.Close()
	}
	c.notify()
	return err
}
func (c *Client) fail(err error) {
	c.mu.Lock()
	if !c.closed {
		c.state.Err = err
	}
	c.mu.Unlock()
	c.Close()
}
func (c *Client) readLoop(conn net.Conn) {
	for {
		b, err := readFrame(conn)
		if err != nil {
			c.fail(err)
			return
		}
		if err = c.handle(b); err != nil {
			c.fail(err)
			return
		}
	}
}
func (c *Client) handle(b []byte) error {
	d := decoder{b: b}
	tag := d.uint()
	if tag == controlTag || tag == notificationTag || tag == 15 {
		defer c.notify()
	}
	switch tag {
	case controlTag:
		kind, data := d.str(), d.str()
		if err := d.finish(); err != nil {
			return err
		}
		if kind == "shell.snapshot.v1" {
			var s Snapshot
			if err := json.Unmarshal([]byte(data), &s); err != nil {
				return err
			}
			if s.BootID == "" {
				return errors.New("snapshot lacks boot identity")
			}
			c.mu.Lock()
			c.state.Snapshot = &s
			c.mu.Unlock()
		} else if strings.HasPrefix(kind, "shell.snapshot.") {
			return fmt.Errorf("unsupported snapshot codec %s", kind)
		}
	case notificationTag:
		kinds := []string{"NeedsAttention", "Finished", "UpdateInstalled", "Custom"}
		kind := d.uint()
		n := Notification{Kind: "Unknown", Title: d.str()}
		if kind < uint64(len(kinds)) {
			n.Kind = kinds[kind]
		}
		n.Body = d.optionalString()
		if d.option() {
			switch d.uint() {
			case 0:
				n.Sound = "Done"
			case 1:
				n.Sound = "Request"
			}
		}
		n.Agent = d.optionalString()
		n.WorkspaceID = d.optionalString()
		n.TabID = d.optionalString()
		n.PaneID = d.optionalString()
		if d.option() {
			positions := []string{"top-left", "top-right", "bottom-left", "bottom-right"}
			p := d.uint()
			if p < uint64(len(positions)) {
				n.Position = positions[p]
			}
		}
		if err := d.finish(); err != nil {
			return err
		}
		c.mu.Lock()
		c.sequence++
		n.ID = c.sequence
		n.ReceivedAt = time.Now()
		c.state.Notifications = append(c.state.Notifications, n)
		if len(c.state.Notifications) > 64 {
			c.state.Notifications = slices.Clone(c.state.Notifications[len(c.state.Notifications)-64:])
		}
		c.mu.Unlock()
	case responseTag:
		boot, id := d.str(), d.str()
		final := d.option()
		data := d.take(d.uint())
		if err := d.finish(); err != nil {
			return err
		}
		c.mu.Lock()
		defer c.mu.Unlock()
		p := c.pending[id]
		if p == nil {
			return nil
		}
		if c.state.Snapshot == nil || boot != c.state.Snapshot.BootID {
			return errors.New("endpoint response boot mismatch")
		}
		if len(p.data)+len(data) > MaxFrameSize {
			return errors.New("endpoint response exceeds bounds")
		}
		p.data = append(p.data, data...)
		if final {
			p.done <- response{data: p.data}
			delete(c.pending, id)
		}
	case 3:
		reason := d.optionalString()
		if err := d.finish(); err != nil {
			return err
		}
		return fmt.Errorf("Herdr shutdown: %s", reason)
	case 15:
		message := d.str()
		if err := d.finish(); err != nil {
			return err
		}
		c.mu.Lock()
		c.state.Err = errors.New(message)
		c.errorSequence++
		c.mu.Unlock()
	}
	return d.err // Unknown frames are skipped in their entirety.
}
func (c *Client) send(ctx context.Context, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	c.mu.Lock()
	conn, connected := c.conn, c.state.Connected
	c.mu.Unlock()
	if !connected || conn == nil {
		return net.ErrClosed
	}
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetWriteDeadline(deadline)
	cancelDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { conn.SetWriteDeadline(time.Now()); close(cancelDone) })
	err := writeFrame(conn, b)
	if !stop() {
		<-cancelDone
		err = ctx.Err()
	}
	conn.SetWriteDeadline(time.Time{})
	if err != nil {
		c.fail(err)
	}
	return err
}
func (c *Client) request(ctx context.Context, method string, params any) error {
	c.mu.Lock()
	if c.state.Snapshot == nil {
		c.mu.Unlock()
		return errors.New("endpoint snapshot not ready")
	}
	if !slices.Contains(c.welcome.Methods, method) {
		c.mu.Unlock()
		return fmt.Errorf("endpoint does not advertise %s", method)
	}
	boot := c.state.Snapshot.BootID
	c.sequence++
	id := fmt.Sprintf("native-%d", c.sequence)
	p := &pending{done: make(chan response, 1)}
	c.pending[id] = p
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	data, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return err
	}
	if err = c.send(ctx, appendString(appendString([]byte{15}, boot), string(data))); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r := <-p.done:
		if r.err != nil {
			return r.err
		}
		var reply struct {
			Error  *struct{ Code, Message string } `json:"error"`
			Result json.RawMessage                 `json:"result"`
		}
		if err := json.Unmarshal(r.data, &reply); err != nil {
			return err
		}
		if reply.Error != nil {
			return fmt.Errorf("%s: %s", reply.Error.Code, reply.Error.Message)
		}
		if len(reply.Result) == 0 {
			return errors.New("endpoint response lacks result")
		}
		return nil
	}
}

// SetSurfaceSize supplies the real viewport dimensions needed for a brief image
// paste surface lease. Metadata-only clients never claim geometry.
func (c *Client) SetSurfaceSize(cols, rows uint16) {
	c.mu.Lock()
	c.cols, c.rows = cols, rows
	c.mu.Unlock()
}

// SendImage activates this shell, focuses the explicit pane, sends the image,
// then restores its passive state. Activation briefly claims geometry/foreground.
// A nil error confirms transport and ordered restoration, not application acceptance.
func (c *Client) SendImage(ctx context.Context, paneID, extension string, data []byte) (err error) {
	if paneID == "" || len(paneID) > 128 {
		return errors.New("explicit pane ID is required")
	}
	if len(data) == 0 || len(data) > MaxImageSize {
		return errors.New("clipboard image must contain 1 byte to 16 MiB")
	}
	switch extension {
	case "png", "jpg", "jpeg", "gif", "webp", "bmp", "tiff":
	default:
		return errors.New("unsupported clipboard image extension")
	}
	c.imageMu.Lock()
	defer c.imageMu.Unlock()
	c.mu.Lock()
	cols, rows := c.cols, c.rows
	beforeError := c.errorSequence
	capable := slices.Contains(c.welcome.Capabilities, "surface_interest")
	c.mu.Unlock()
	if cols == 0 || rows == 0 {
		return errors.New("set actual surface dimensions before image paste")
	}
	if !capable {
		return errors.New("endpoint does not support safe surface leases")
	}
	resize := appendUint(appendUint([]byte{12, 0, 0}, uint64(cols)), uint64(rows))
	resize = append(resize, 0)
	if err = c.send(ctx, resize); err != nil {
		return err
	}
	// Restore even after an uncertain activation outcome; the server processes
	// these requests and image staging in order on this connection.
	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if e := c.request(restoreCtx, "client_shell.surface.set", map[string]bool{"active": false}); e != nil {
			c.Close()
			err = errors.Join(err, fmt.Errorf("restore passive endpoint: %w", e))
		}
		c.mu.Lock()
		if c.errorSequence > beforeError {
			err = errors.Join(err, c.state.Err)
		}
		c.mu.Unlock()
	}()
	if err = c.request(ctx, "client_shell.surface.set", map[string]bool{"active": true}); err != nil {
		return err
	}
	if err = c.request(ctx, "pane.focus", map[string]string{"pane_id": paneID}); err != nil {
		return err
	}
	b := appendString(appendString([]byte{2, 1}, paneID), extension)
	b = append(appendUint(b, uint64(len(data))), data...)
	return c.send(ctx, b)
}

// Package client talks to the herdr daemon.
//
// The daemon serves a socket per session. The client connects to it, says
// hello, and from then on the daemon pushes the state of the session and
// the surfaces of the panes, and takes the requests of the window.
package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"herdr-ui/internal/protocol"
)

// ErrClosed says that the daemon closed the connection.
var ErrClosed = errors.New("client: the daemon closed the connection")

// ErrNoWelcome says that the daemon did not answer the handshake.
var ErrNoWelcome = errors.New("client: the daemon did not answer the handshake")

// Event is something the daemon said.
type Event struct {
	// Snapshot is the state of the session, when it changed.
	Snapshot *protocol.Snapshot
	// Surface is the surface of the pane in front, when it changed.
	Surface *protocol.SurfaceFrame
	// Notification is something that happened.
	Notification *protocol.Notification
	// Error is an error the window must show.
	Error *protocol.ShellError
	// WindowTitle is the title of the window.
	WindowTitle string
}

// Client is a connection to the daemon.
type Client struct {
	conn net.Conn

	mu       sync.Mutex
	writes   sync.Mutex
	bootID   string
	welcome  *protocol.ServerWelcome
	nextID   int
	pending  map[string]chan *protocol.ResponseChunk
	closed   bool
	closeErr error

	// Events carries what the daemon says.
	Events chan Event
}

// Connect opens the socket of the daemon and says hello.
func Connect(path string, hello protocol.ClientHello) (*Client, error) {
	conn, err := net.DialTimeout("unix", path, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("client: the socket of the daemon is not there: %w", err)
	}
	c := &Client{
		conn:    conn,
		pending: map[string]chan *protocol.ResponseChunk{},
		Events:  make(chan Event, 64),
	}
	if err := c.handshake(hello); err != nil {
		conn.Close()
		return nil, err
	}
	go c.read()
	return c, nil
}

// handshake says hello and waits for the answer.
func (c *Client) handshake(hello protocol.ClientHello) error {
	data, err := hello.JSON()
	if err != nil {
		return err
	}
	if err := c.write(&protocol.EndpointControl{Kind: protocol.ControlHello, Data: data}); err != nil {
		return err
	}
	// The answer comes before anything else.
	deadline := time.Now().Add(10 * time.Second)
	for {
		payload, err := protocol.ReadFrame(c.conn)
		if err != nil {
			return fmt.Errorf("client: the daemon did not answer: %w", err)
		}
		message, err := protocol.DecodeServerMessage(payload)
		if err != nil {
			return fmt.Errorf("client: the answer of the daemon is not read: %w", err)
		}
		if message.Kind != protocol.ServerControl {
			continue
		}
		switch message.Control.Kind {
		case protocol.ControlWelcome:
			welcome, err := protocol.DecodeWelcome(message.Control.Data)
			if err != nil {
				return fmt.Errorf("client: the answer of the daemon is not read: %w", err)
			}
			if welcome.Error != nil {
				return fmt.Errorf("client: the daemon refused the client: %s", welcome.Error.Message)
			}
			c.mu.Lock()
			c.welcome = welcome
			c.mu.Unlock()
			return nil
		default:
			if time.Now().After(deadline) {
				return ErrNoWelcome
			}
		}
	}
}

// Err returns why the connection ended, when it ended.
func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeErr
}

// Welcome returns the answer of the daemon to the handshake.
func (c *Client) Welcome() *protocol.ServerWelcome {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.welcome
}

// BootID returns the boot of the session the client is attached to.
func (c *Client) BootID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bootID
}

// read reads the frames the daemon sends, until it stops.
func (c *Client) read() {
	for {
		payload, err := protocol.ReadFrame(c.conn)
		if err != nil {
			c.close(err)
			return
		}
		message, err := protocol.DecodeServerMessage(payload)
		if err != nil {
			c.close(fmt.Errorf("client: a message of the daemon is not read: %w", err))
			return
		}
		c.dispatch(message)
	}
}

// dispatch gives a message of the daemon to whoever waits for it.
func (c *Client) dispatch(message *protocol.ServerMessage) {
	switch message.Kind {
	case protocol.ServerControl:
		if message.Control.Kind == protocol.ControlSnapshot {
			snapshot, err := protocol.DecodeSnapshot(message.Control.Data)
			if err != nil {
				c.close(fmt.Errorf("client: the snapshot is not read: %w", err))
				return
			}
			c.mu.Lock()
			c.bootID = snapshot.BootID
			c.mu.Unlock()
			c.emit(Event{Snapshot: snapshot})
			return
		}
	case protocol.ServerSnapshot:
		c.mu.Lock()
		c.bootID = message.Snapshot.BootID
		c.mu.Unlock()
		c.emit(Event{Snapshot: message.Snapshot})
	case protocol.ServerSurface:
		c.emit(Event{Surface: message.Surface})
	case protocol.ServerResponse:
		c.mu.Lock()
		waiter := c.pending[message.Response.RequestID]
		delete(c.pending, message.Response.RequestID)
		c.mu.Unlock()
		if waiter != nil {
			waiter <- message.Response
		}
	case protocol.ServerNotification:
		c.emit(Event{Notification: message.Notification})
	case protocol.ServerError:
		c.emit(Event{Error: message.Error})
	case protocol.ServerWindowTitle:
		c.emit(Event{WindowTitle: message.WindowTitle})
	}
}

// emit gives an event to the reader, and does not wait when nobody reads.
func (c *Client) emit(event Event) {
	select {
	case c.Events <- event:
	default:
		// The window is behind. The event is dropped rather than
		// stopping the daemon's messages.
	}
}

// write sends a message to the daemon.
func (c *Client) write(message protocol.ClientMessage) error {
	c.writes.Lock()
	defer c.writes.Unlock()
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return ErrClosed
	}
	return protocol.WriteFrame(c.conn, protocol.Encode(message))
}

// Request asks the daemon to do one thing and waits for the answer.
//
// The answer is the JSON the daemon gives: a result, or an error.
func (c *Client) Request(method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.nextID++
	id := fmt.Sprintf("herdr-ui-%d", c.nextID)
	bootID := c.bootID
	waiter := make(chan *protocol.ResponseChunk, 1)
	c.pending[id] = waiter
	c.mu.Unlock()

	if params == nil {
		params = map[string]any{}
	}
	request, err := json.Marshal(map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	})
	if err != nil {
		return nil, err
	}
	if err := c.write(&protocol.EndpointRequest{BootID: bootID, Request: string(request)}); err != nil {
		return nil, err
	}

	select {
	case chunk := <-waiter:
		return parseResponse(chunk)
	case <-time.After(30 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("client: the daemon did not answer %s", method)
	}
}

// parseResponse reads the answer of the daemon.
func parseResponse(chunk *protocol.ResponseChunk) (json.RawMessage, error) {
	var envelope struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(chunk.Data), &envelope); err != nil {
		return nil, fmt.Errorf("client: the answer of the daemon is not JSON: %w", err)
	}
	if envelope.Error != nil {
		return nil, fmt.Errorf("client: the daemon said no: %s", envelope.Error.Message)
	}
	return envelope.Result, nil
}

// Input sends what the user did in a pane.
func (c *Client) Input(paneID string, events ...protocol.PaneInputEvent) error {
	if len(events) == 0 {
		return nil
	}
	return c.write(&protocol.PaneInput{PaneID: paneID, Events: events})
}

// Focus tells the daemon whether the window has the keyboard.
func (c *Client) Focus(focused bool) error {
	return c.write(&protocol.ShellFocus{Focused: focused})
}

// Resize tells the daemon how big the viewport of the pane is.
func (c *Client) Resize(cols, rows uint16, cellWidth, cellHeight uint32) error {
	return c.write(&protocol.ShellResize{
		CellWidth:  cellWidth,
		CellHeight: cellHeight,
		Cols:       cols,
		Rows:       rows,
	})
}

// close ends the connection.
func (c *Client) close(err error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.closeErr = err
	c.mu.Unlock()
	c.conn.Close()
	close(c.Events)
}

// Close ends the connection.
func (c *Client) Close() error {
	c.close(ErrClosed)
	return nil
}

// SocketPath returns the socket of the daemon of this machine.
//
// The daemon's own rules: the environment may say where the socket is, and
// otherwise it is in the directory of the daemon's configuration.
func SocketPath() string {
	if path := os.Getenv("HERDR_CLIENT_SOCKET_PATH"); path != "" {
		return path
	}
	if path := os.Getenv("HERDR_SOCKET_PATH"); path != "" {
		return path
	}
	return filepath.Join(ConfigDir(), "herdr-client.sock")
}

// ConfigDir returns the directory of the daemon's configuration.
func ConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, appDirName())
	}
	if runtime.GOOS == "windows" {
		if dir := os.Getenv("APPDATA"); dir != "" {
			return filepath.Join(dir, appDirName())
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), appDirName())
	}
	return filepath.Join(home, ".config", appDirName())
}

// appDirName is the name of the daemon's directory.
//
// The daemon keeps a directory of its own for the builds it makes while
// it is being worked on.
func appDirName() string {
	if os.Getenv("HERDR_DEBUG") != "" {
		return "herdr-dev"
	}
	return "herdr"
}

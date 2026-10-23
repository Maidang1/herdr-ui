package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type ProtocolError struct{ Reason string }

func (e *ProtocolError) Error() string { return "Herdr protocol: " + e.Reason }

type RemoteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *RemoteError) Error() string { return fmt.Sprintf("Herdr %s: %s", e.Code, e.Message) }

var connectionIDs atomic.Uint64

// Client owns a single serial request lane. Cancellation after a write closes
// the lane; no later request can accidentally consume the abandoned response.
type Client struct {
	target Target
	mu     sync.Mutex
	conn   net.Conn
	gate   chan struct{}
	id     uint64
	seq    uint64
	closed chan struct{}
	once   sync.Once
}

func Dial(ctx context.Context, target Target) (*Client, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", target.Socket)
	if err != nil {
		return nil, err
	}
	return &Client{target: target, conn: conn, gate: make(chan struct{}, 1), id: connectionIDs.Add(1), closed: make(chan struct{})}, nil
}

func (c *Client) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.conn != nil {
			err = c.conn.Close()
			c.conn = nil
		}
	})
	return err
}

func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.closed:
		return net.ErrClosed
	case c.gate <- struct{}{}:
	}
	defer func() { <-c.gate }()
	select {
	case <-c.closed:
		return net.ErrClosed
	default:
	}
	c.seq++
	id := fmt.Sprintf("c%d-r%d", c.id, c.seq)
	stop := context.AfterFunc(ctx, func() { c.Close() })
	defer stop()
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		var err error
		conn, err = (&net.Dialer{}).DialContext(ctx, "unix", c.target.Socket)
		if err != nil {
			c.Close()
			return err
		}
		c.mu.Lock()
		select {
		case <-c.closed:
			c.mu.Unlock()
			conn.Close()
			return net.ErrClosed
		default:
		}
		c.conn = conn
		c.mu.Unlock()
	}
	// Ordinary Herdr RPCs are one request per connection. Opening the next
	// socket is normal framing, not a retry of the previous operation.
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		conn.Close()
	}()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	if err := sendRequest(conn, id, method, params); err != nil {
		c.Close()
		return err
	}
	line, err := readLine(bufio.NewReader(conn))
	if err != nil {
		c.Close()
		return err
	}
	err = decodeResponse(line, id, out)
	var remote *RemoteError
	if err != nil && !errors.As(err, &remote) {
		c.Close()
	}
	return err
}

func sendRequest(conn net.Conn, id, method string, params any) error {
	if params == nil {
		params = struct{}{}
	}
	return json.NewEncoder(conn).Encode(struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params any    `json:"params"`
	}{id, method, params})
}

func decodeResponse(line []byte, id string, out any) error {
	var response struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(line, &response); err != nil {
		return &ProtocolError{err.Error()}
	}
	if response.ID != id {
		return &ProtocolError{"response ID mismatch"}
	}
	hasError := len(response.Error) > 0 && string(response.Error) != "null"
	hasResult := len(response.Result) > 0 && string(response.Result) != "null"
	if hasError == hasResult {
		return &ProtocolError{"response must contain exactly one result or error"}
	}
	if hasError {
		var remote RemoteError
		if err := json.Unmarshal(response.Error, &remote); err != nil || remote.Code == "" {
			return &ProtocolError{"invalid remote error"}
		}
		return &remote
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(response.Result, out); err != nil {
		return &ProtocolError{err.Error()}
	}
	return nil
}

type Subscription struct {
	Type   string `json:"type"`
	PaneID string `json:"pane_id,omitempty"`
}
type Event struct {
	Name string          `json:"event"`
	Data json.RawMessage `json:"data"`
}

// Stream has exactly one reader. Close and context cancellation interrupt Next.
type Stream struct {
	conn   net.Conn
	reader *bufio.Reader
	once   sync.Once
	stop   func() bool
}

func Subscribe(ctx context.Context, target Target, subscriptions []Subscription) (*Stream, error) {
	handshake, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(handshake, "unix", target.Socket)
	if err != nil {
		return nil, err
	}
	s := &Stream{conn: conn, reader: bufio.NewReader(conn)}
	s.stop = context.AfterFunc(ctx, func() { conn.Close() })
	stopHandshake := context.AfterFunc(handshake, func() { conn.Close() })
	defer stopHandshake()
	deadline, _ := handshake.Deadline()
	conn.SetDeadline(deadline)
	id := fmt.Sprintf("sub-%d", connectionIDs.Add(1))
	params := struct {
		Subscriptions []Subscription `json:"subscriptions"`
	}{subscriptions}
	if err := sendRequest(conn, id, "events.subscribe", params); err != nil {
		s.Close()
		return nil, err
	}
	line, err := readLine(s.reader)
	if err != nil {
		s.Close()
		return nil, err
	}
	var ack struct {
		Type string `json:"type"`
	}
	if err := decodeResponse(line, id, &ack); err != nil {
		s.Close()
		return nil, err
	}
	if ack.Type != "subscription_started" {
		s.Close()
		return nil, &ProtocolError{"missing subscription_started acknowledgement"}
	}
	// Stop the handshake cancellation before cancel() runs. The parent context
	// still owns the long-lived subscription through s.stop.
	if !stopHandshake() {
		s.Close()
		return nil, handshake.Err()
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func (s *Stream) Next() (Event, error) {
	line, err := readLine(s.reader)
	if err != nil {
		return Event{}, err
	}
	var event Event
	if err := json.Unmarshal(line, &event); err != nil {
		return Event{}, &ProtocolError{err.Error()}
	}
	if event.Name == "" {
		return Event{}, &ProtocolError{"subscription event has no name"}
	}
	return event, nil
}
func (s *Stream) Close() error {
	var err error
	s.once.Do(func() {
		if s.stop != nil {
			s.stop()
		}
		err = s.conn.Close()
	})
	return err
}

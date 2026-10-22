package ui

import (
	"sync"
	"time"

	"herdr-ui/internal/client"
	"herdr-ui/internal/protocol"
)

// Live is the state of the session, as the daemon tells it.
//
// The window draws this and nothing else. When the daemon is not there,
// the window draws the sample session instead, so that it is never empty.
type Live struct {
	mu       sync.Mutex
	events   chan client.Event
	client   *client.Client
	snapshot *protocol.Snapshot
	surface  *protocol.SurfaceFrame
	err      error
	version  uint64
}

// Connect opens the socket of the daemon and keeps what it says.
//
// It does not stop when the daemon is not there: the window then shows
// the sample session, and the daemon is looked for again.
func Connect() *Live {
	live := &Live{events: make(chan client.Event, 64)}
	live.connect()
	return live
}

// connect opens the socket and starts reading what the daemon says.
func (l *Live) connect() {
	hello := protocol.DefaultHello(120, 40, 8, 16)
	c, err := client.Connect(client.SocketPath(), hello)
	if err != nil {
		l.mu.Lock()
		l.err = err
		l.mu.Unlock()
		return
	}
	l.mu.Lock()
	l.client = c
	l.err = nil
	l.mu.Unlock()

	l.mu.Lock()
	l.events = make(chan client.Event, 64)
	l.mu.Unlock()
	go l.read(c)
}

// read keeps what the daemon says.
func (l *Live) read(c *client.Client) {
	for event := range c.Events {
		l.mu.Lock()
		switch {
		case event.Snapshot != nil:
			l.snapshot = event.Snapshot
			l.version++
		case event.Surface != nil:
			l.surface = event.Surface
			l.version++
		case event.Error != nil:
			l.err = event.Error
		}
		events := l.events
		l.mu.Unlock()
		// The event is also given to whoever watches the session.
		select {
		case events <- event:
		default:
		}
	}
	l.mu.Lock()
	l.client = nil
	l.version++
	l.mu.Unlock()
}

// Events carries what the daemon says.
func (l *Live) Events() <-chan client.Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.events
}

// Close ends the connection to the daemon.
func (l *Live) Close() error {
	c := l.Client()
	if c == nil {
		return nil
	}
	return c.Close()
}

// Client returns the connection to the daemon, when there is one.
func (l *Live) Client() *client.Client {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.client
}

// Snapshot returns the state of the session.
func (l *Live) Snapshot() *protocol.Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snapshot
}

// Surface returns the surface of the pane in front.
func (l *Live) Surface() *protocol.SurfaceFrame {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.surface
}

// Err returns why the daemon is not there, when it is not there.
func (l *Live) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

// Connected tells whether the daemon is there.
func (l *Live) Connected() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.client != nil
}

// FocusedPaneID returns the pane that has the keyboard.
func (l *Live) FocusedPaneID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.snapshot == nil {
		return ""
	}
	return l.snapshot.FocusedPaneID
}

// Request asks the daemon to do one thing.
//
// A window that is not connected to the daemon does nothing, and says so.
func (l *Live) Request(method string, params any) error {
	c := l.Client()
	if c == nil {
		return ErrNotConnected
	}
	_, err := c.Request(method, params)
	return err
}

// ErrNotConnected says that the window is not attached to the daemon.
var ErrNotConnected = errNotConnected{}

type errNotConnected struct{}

func (errNotConnected) Error() string { return "the window is not attached to the daemon" }

// Tick is how often the window looks at the state of the session.
//
// The daemon pushes what changes; the window reads it when it is drawn
// again. A third of a second is fast enough to feel alive and slow enough
// to cost nothing.
const Tick = 33 * time.Millisecond

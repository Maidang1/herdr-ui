package client

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"herdr-ui/internal/protocol"
)

// fakeDaemon is a daemon that says what the test tells it to say.
type fakeDaemon struct {
	path string
	// requests are the requests the client sent, in order.
	requests chan string
}

// startFakeDaemon opens a socket and serves it in the background.
func startFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	// The socket of a unix socket has a short path: the one of the
	// temporary directory of the test is too long for macOS.
	path := filepath.Join(os.TempDir(), fmt.Sprintf("herdr-ui-test-%d.sock", os.Getpid()))
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("the socket of the fake daemon is not there: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	daemon := &fakeDaemon{path: path, requests: make(chan string, 8)}
	go daemon.serve(t, listener)
	return daemon
}

// serve is the daemon of the test.
func (d *fakeDaemon) serve(t *testing.T, listener net.Listener) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	// The handshake of the client.
	payload, err := protocol.ReadFrame(conn)
	if err != nil {
		return
	}
	hello := decodeControl(t, payload)
	if hello.Kind != protocol.ControlHello {
		t.Errorf("the client says %q first, the handshake was wanted", hello.Kind)
	}

	// The answer of the daemon.
	welcome, _ := json.Marshal(protocol.ServerWelcome{
		Generation:    1,
		ServerVersion: "0.9.5",
		SnapshotCodec: protocol.CodecSnapshot,
		SurfaceCodec:  protocol.CodecSurface,
		InputCodec:    protocol.CodecInput,
		BlobCodec:     protocol.CodecBlob,
		Methods:       []string{"pane.focus"},
	})
	writeControl(t, conn, protocol.ControlWelcome, string(welcome))

	// The snapshot of the session.
	snapshot, _ := json.Marshal(protocol.Snapshot{
		BootID:            "boot-v1",
		Revision:          3,
		FocusedWorkspaceID: "w1",
		Workspaces: []protocol.Workspace{
			{WorkspaceID: "w1", Label: "deepseek-harness", Branch: "master", Focused: true},
		},
	})
	writeControl(t, conn, protocol.ControlSnapshot, string(snapshot))

	// The surface of the pane in front.
	writeFrame(t, conn, surfaceBytes())

	// The answer of a request.
	for {
		payload, err := protocol.ReadFrame(conn)
		if err != nil {
			return
		}
		if !isRequest(payload) {
			// The input of a pane is not a request.
			continue
		}
		request := decodeRequest(t, payload)
		d.requests <- request.Request
		answer, _ := json.Marshal(map[string]any{
			"id":     requestID(request.Request),
			"result": map[string]any{"ok": true},
		})
		w := &protocol.Writer{}
		w.Variant(18)
		w.Str("boot-v1")
		w.Str(requestID(request.Request))
		w.Bool(true)
		w.Bytes(answer)
		writeFrame(t, conn, w.Payload())
	}
}

// decodeControl reads a named control message.
func decodeControl(t *testing.T, payload []byte) *protocol.Control {
	t.Helper()
	r := protocol.NewReader(payload)
	variant, err := r.Variant()
	if err != nil {
		t.Fatalf("the message of the client is not read: %v", err)
	}
	if variant != 20 {
		t.Fatalf("the message of the client is the case %d, a control message was wanted", variant)
	}
	kind, err := r.Str()
	if err != nil {
		t.Fatalf("the control message is not read: %v", err)
	}
	data, err := r.Str()
	if err != nil {
		t.Fatalf("the control message is not read: %v", err)
	}
	return &protocol.Control{Kind: kind, Data: data}
}

// isRequest tells whether the bytes of the client are a request.
func isRequest(payload []byte) bool {
	r := protocol.NewReader(payload)
	variant, err := r.Variant()
	return err == nil && variant == 15
}

// decodeRequest reads a request of the client.
func decodeRequest(t *testing.T, payload []byte) *protocol.EndpointRequest {
	t.Helper()
	r := protocol.NewReader(payload)
	variant, err := r.Variant()
	if err != nil {
		t.Fatalf("the message of the client is not read: %v", err)
	}
	if variant != 15 {
		t.Fatalf("the message of the client is the case %d, a request was wanted", variant)
	}
	bootID, err := r.Str()
	if err != nil {
		t.Fatalf("the request is not read: %v", err)
	}
	request, err := r.Str()
	if err != nil {
		t.Fatalf("the request is not read: %v", err)
	}
	return &protocol.EndpointRequest{BootID: bootID, Request: request}
}

// requestID returns the number of a request.
func requestID(request string) string {
	var envelope struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(request), &envelope)
	return envelope.ID
}

// writeControl sends a named control message.
func writeControl(t *testing.T, conn net.Conn, kind, data string) {
	t.Helper()
	w := &protocol.Writer{}
	w.Variant(20)
	w.Str(kind)
	w.Str(data)
	writeFrame(t, conn, w.Payload())
}

// writeFrame sends a frame.
func writeFrame(t *testing.T, conn net.Conn, payload []byte) {
	t.Helper()
	if err := protocol.WriteFrame(conn, payload); err != nil {
		t.Errorf("the frame is not written: %v", err)
	}
}

// surfaceBytes returns the bytes of a surface of two cells.
func surfaceBytes() []byte {
	w := &protocol.Writer{}
	// The case of a surface.
	w.Variant(13)
	w.Str("boot-v1")
	w.U64(1)
	w.U64(1)
	w.Count(2)
	for _, symbol := range []string{"a", "b"} {
		w.Str(symbol)
		w.U32(0x02D79921)
		w.U32(0)
		w.U16(0)
		w.Bool(false)
		w.U8(0)
	}
	w.U16(2)
	w.U16(1)
	w.U8(1)
	w.U16(1)
	w.U16(0)
	w.Bool(true)
	w.U8(2)
	w.Count(0)
	w.Bytes(nil)
	w.Count(0)
	w.Count(0)
	w.U8(0)
	w.Count(0)
	w.Count(0)
	w.Count(0)
	return w.Payload()
}

// TestTheClientTalksToTheDaemon checks that the client says hello, takes
// the snapshot and the surface, and asks the daemon to do things.
func TestTheClientTalksToTheDaemon(t *testing.T) {
	daemon := startFakeDaemon(t)

	c, err := Connect(daemon.path, protocol.DefaultHello(120, 40, 8, 16))
	if err != nil {
		t.Fatalf("the client does not connect: %v", err)
	}
	defer c.Close()

	welcome := c.Welcome()
	if welcome == nil {
		t.Fatal("the daemon did not answer the handshake")
	}
	if welcome.ServerVersion != "0.9.5" {
		t.Errorf("the version of the daemon is %q, the fake one says %q", welcome.ServerVersion, "0.9.5")
	}

	var snapshot *protocol.Snapshot
	var surface *protocol.SurfaceFrame
	deadline := time.After(5 * time.Second)
	for snapshot == nil || surface == nil {
		select {
		case event := <-c.Events:
			if event.Snapshot != nil {
				snapshot = event.Snapshot
			}
			if event.Surface != nil {
				surface = event.Surface
			}
		case <-deadline:
			t.Fatal("the daemon said nothing in time")
		}
	}
	if snapshot.BootID != "boot-v1" {
		t.Errorf("the boot is %q, the fake daemon says %q", snapshot.BootID, "boot-v1")
	}
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].Label != "deepseek-harness" {
		t.Errorf("the workspaces are %v, the fake daemon sends one", snapshot.Workspaces)
	}
	if surface.Width != 2 || len(surface.Cells) != 2 {
		t.Errorf("the surface is %dx%d with %d cells, the fake daemon sends 2x1 with 2 cells",
			surface.Width, surface.Height, len(surface.Cells))
	}
	if surface.Text(0) != "ab" {
		t.Errorf("the surface says %q, the fake daemon sends %q", surface.Text(0), "ab")
	}

	// A request must come back with the answer of the daemon.
	result, err := c.Request("pane.focus", map[string]any{"pane_id": "w1:p1"})
	if err != nil {
		t.Fatalf("the daemon does not answer the request: %v", err)
	}
	if !strings.Contains(string(result), "\"ok\"") {
		t.Errorf("the answer is %s, the fake daemon says that it went well", result)
	}
	select {
	case request := <-daemon.requests:
		if !strings.Contains(request, "pane.focus") {
			t.Errorf("the request is %s, the fake daemon was asked to focus a pane", request)
		}
	case <-time.After(2 * time.Second):
		t.Error("the request never reached the daemon")
	}
}

// TestTheInputGoesToTheDaemon checks that what the user does in a pane
// reaches the daemon.
func TestTheInputGoesToTheDaemon(t *testing.T) {
	daemon := startFakeDaemon(t)

	c, err := Connect(daemon.path, protocol.DefaultHello(120, 40, 8, 16))
	if err != nil {
		t.Fatalf("the client does not connect: %v", err)
	}
	defer c.Close()

	// The daemon must know the boot before the input is sent.
	deadline := time.After(5 * time.Second)
	for c.BootID() == "" {
		select {
		case <-c.Events:
		case <-deadline:
			t.Fatal("the daemon says nothing in time")
		}
	}
	if err := c.Input("w1:p1", &protocol.TextCommit{Text: "hello"}); err != nil {
		t.Fatalf("the input is not sent: %v", err)
	}
	if err := c.Focus(true); err != nil {
		t.Fatalf("the focus is not sent: %v", err)
	}
}

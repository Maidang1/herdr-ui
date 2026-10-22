package herdr

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeDaemon is a daemon that speaks just enough of the protocol for the
// client to be exercised without a real one.
type fakeDaemon struct {
	socket string
	frames chan []byte
}

func startFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	// A unix socket path is short on macOS, so the test keeps it short too.
	socket := filepath.Join("/tmp", "herdr-ui-fake-"+strings.ReplaceAll(t.Name(), "/", "-")+".sock")
	_ = os.Remove(socket)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		listener.Close()
		_ = os.Remove(socket)
	})

	daemon := &fakeDaemon{socket: socket, frames: make(chan []byte, 16)}
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			payload, err := readFrame(conn)
			if err != nil {
				return
			}
			daemon.frames <- payload
			variant, _ := newVariantReader(payload).variant()
			if variant == msgClientShellEndpointRequest {
				answerRequest(conn, payload)
				continue
			}
			control, err := decodeEndpointControl(payload)
			if err != nil {
				continue
			}
			if control.Kind != "endpoint.hello.v1" {
				continue
			}
			welcome, _ := json.Marshal(map[string]any{
				"generation":     1,
				"server_version": "0.0.0",
				"methods":        []string{"pane.focus"},
				"capabilities":   []string{},
			})
			if err := writeFrame(conn, encodeEndpointControl("endpoint.welcome.v1", string(welcome))); err != nil {
				return
			}
			snapshot, _ := json.Marshal(map[string]any{
				"boot_id":         "boot-1",
				"revision":        1,
				"workspaces":      []map[string]any{},
				"tabs":            []map[string]any{},
				"panes":           []map[string]any{},
				"agents":          []map[string]any{},
				"focused_pane_id": "w1:p1",
			})
			if err := writeFrame(conn, encodeEndpointControl(SnapshotCodec, string(snapshot))); err != nil {
				return
			}
		}
	}()
	return daemon
}

func TestTheClientSaysHelloAndReadsTheWelcome(t *testing.T) {
	daemon := startFakeDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{
		SocketPath:  daemon.socket,
		CellWidthPx: 8, CellHeightPx: 17, Cols: 80, Rows: 24,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()

	welcome := endpoint.Welcome()
	if welcome == nil {
		t.Fatal("the daemon sent no welcome")
	}
	if welcome.Generation != 1 {
		t.Fatalf("the generation came back as %d", welcome.Generation)
	}
	if !welcome.HasMethod("pane.focus") {
		t.Fatalf("the methods came back as %v", welcome.Methods)
	}

	select {
	case snapshot := <-endpoint.Snapshots():
		if snapshot.BootID != "boot-1" {
			t.Fatalf("the boot id came back as %q", snapshot.BootID)
		}
	case <-ctx.Done():
		t.Fatal("no snapshot arrived")
	}
}

// answerRequest answers one API request the way the daemon does: with a
// chunk that carries the answer to the request the client named.
func answerRequest(conn net.Conn, payload []byte) {
	decoder := newPayloadDecoder(payload)
	bootID, err := decoder.String()
	if err != nil {
		return
	}
	request, err := decoder.String()
	if err != nil {
		return
	}
	var envelope struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(request), &envelope); err != nil {
		return
	}
	answer, _ := json.Marshal(map[string]any{
		"id":     envelope.ID,
		"result": map[string]any{"type": "pane_focus"},
	})
	encoder := bincodeEncoder()
	encoder.Variant(msgServerClientShellEndpointResponse)
	encoder.String(bootID)
	encoder.String(envelope.ID)
	encoder.Bool(true)
	encoder.BytesOf(answer)
	_ = writeFrame(conn, encoder.Bytes())
}

func TestTheClientSendsARequestAndReadsTheAnswer(t *testing.T) {
	daemon := startFakeDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{SocketPath: daemon.socket, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()

	// Wait for the snapshot, which carries the boot the request needs.
	select {
	case <-endpoint.Snapshots():
	case <-ctx.Done():
		t.Fatal("no snapshot arrived")
	}

	result, err := endpoint.RequestRaw(ctx, "pane.focus", map[string]any{"pane_id": "w1:p1"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), "pane_focus") {
		t.Fatalf("the answer came back as %s", result)
	}
}

func TestTheClientTypesIntoAPane(t *testing.T) {
	daemon := startFakeDaemon(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{SocketPath: daemon.socket, Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()

	if err := endpoint.Focus(true); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.SendText("w1:p1", "hello"); err != nil {
		t.Fatal(err)
	}

	// The hello comes first, then the focus, then the input.
	deadline := time.After(5 * time.Second)
	var seen []string
	for len(seen) < 3 {
		select {
		case payload := <-daemon.frames:
			seen = append(seen, describeFrame(payload))
		case <-deadline:
			t.Fatalf("the frames came as %v", seen)
		}
	}
	if seen[0] != "endpoint control" {
		t.Fatalf("the first frame came as %q", seen[0])
	}
	if seen[1] != "focus" {
		t.Fatalf("the second frame came as %q", seen[1])
	}
	if seen[2] != "pane input" {
		t.Fatalf("the third frame came as %q", seen[2])
	}
}

// describeFrame names a frame for the tests.
func describeFrame(payload []byte) string {
	decoder := newVariantReader(payload)
	variant, ok := decoder.variant()
	if !ok {
		return "unknown"
	}
	switch variant {
	case msgEndpointControl:
		if _, err := decodeEndpointControl(payload); err != nil {
			return "unknown"
		}
		return "endpoint control"
	case msgClientShellFocus:
		return "focus"
	case msgClientShellPaneInput:
		return "pane input"
	default:
		return "other"
	}
}

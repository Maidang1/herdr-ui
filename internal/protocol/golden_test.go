package protocol

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// golden reads the bytes the Rust client writes for each message.
//
// The file is the check of this package: the two clients must write the
// same bytes for the same message, or they do not understand each other.
func golden(t *testing.T) map[string][]byte {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", "golden.txt"))
	if err != nil {
		t.Fatalf("the golden bytes are not there: %v", err)
	}
	defer file.Close()

	bytes := map[string][]byte{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		// The bytes are written one by one, the way they are read.
		raw, err := hex.DecodeString(strings.Join(fields[1:], ""))
		if err != nil {
			t.Fatalf("the golden bytes of %s are not bytes: %v", fields[0], err)
		}
		bytes[fields[0]] = raw
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("the golden bytes are not readable: %v", err)
	}
	return bytes
}

// TestTheBytesMatchTheRustClient checks that every message this package
// writes is written the way the Rust client writes it.
func TestTheBytesMatchTheRustClient(t *testing.T) {
	want := golden(t)

	cases := []struct {
		name    string
		message ClientMessage
	}{
		{
			"key_char",
			&PaneInput{
				PaneID: "w1:p1",
				Events: []PaneInputEvent{
					&KeyEvent{Code: CharKey('a')},
				},
			},
		},
		{
			"key_enter_shift",
			&PaneInput{
				PaneID: "w1:p1",
				Events: []PaneInputEvent{
					&KeyEvent{Code: KeyEnter, Modifiers: ModShift},
				},
			},
		},
		{
			"text_commit",
			&PaneInput{
				PaneID: "w1:p1",
				Events: []PaneInputEvent{&TextCommit{Text: "hi"}},
			},
		},
		{
			"paste",
			&PaneInput{
				PaneID: "w1:p1",
				Events: []PaneInputEvent{&Paste{Text: "x"}},
			},
		},
		{
			"focus",
			&ShellFocus{Focused: true},
		},
		{
			"shell_resize",
			&ShellResize{
				CellWidth:  8,
				CellHeight: 16,
				Cols:       90,
				Rows:       30,
			},
		},
	}
	for _, c := range cases {
		got, ok := want[c.name]
		if !ok {
			t.Errorf("there are no golden bytes for %s", c.name)
			continue
		}
		encoded := Encode(c.message)
		if string(encoded) != string(got) {
			t.Errorf("%s:\n the bytes of this client are %x\n the bytes of the Rust client are %x", c.name, encoded, got)
		}
	}
}

// TestTheHandshakeMatchesTheRustClient checks the handshake, which carries
// JSON: the bytes must be the same length, and the JSON must say the same
// thing.
func TestTheHandshakeMatchesTheRustClient(t *testing.T) {
	want, ok := golden(t)["hello"]
	if !ok {
		t.Fatal("there are no golden bytes for the handshake")
	}
	hello := DefaultHello(120, 40, 8, 16)
	data, err := hello.JSON()
	if err != nil {
		t.Fatalf("the handshake is not JSON: %v", err)
	}
	// The JSON of the Rust client is written with its keys in order, so
	// the bytes are compared by what they say.
	var gotJSON, wantJSON map[string]any
	if err := json.Unmarshal([]byte(data), &gotJSON); err != nil {
		t.Fatalf("the handshake of this client is not JSON: %v", err)
	}
	start := bytesIndex(want, []byte(`{"`))
	if start < 0 {
		t.Fatal("the golden handshake has no JSON in it")
	}
	if err := json.Unmarshal(want[start:], &wantJSON); err != nil {
		t.Fatalf("the JSON of the Rust client is not JSON: %v", err)
	}
	if len(gotJSON) != len(wantJSON) {
		t.Errorf("the handshake says %d things, the Rust client says %d", len(gotJSON), len(wantJSON))
	}
	for key, value := range wantJSON {
		got, ok := gotJSON[key]
		if !ok {
			t.Errorf("the handshake of this client does not say %q", key)
			continue
		}
		if key == "surface_size" {
			continue
		}
		if !sameJSON(got, value) {
			t.Errorf("the handshake says %q is %v, the Rust client says %v", key, got, value)
		}
	}
}

// TestTheSurfaceOfTheRustClient checks that the surface the Rust client
// wrote is read the way it was written.
func TestTheSurfaceOfTheRustClient(t *testing.T) {
	want, ok := golden(t)["surface"]
	if !ok {
		t.Fatal("there are no golden bytes for the surface")
	}
	message, err := DecodeServerMessage(want)
	if err != nil {
		t.Fatalf("the surface of the Rust client is not read: %v", err)
	}
	if message.Kind != ServerSurface {
		t.Fatalf("the message is a %v, a surface was wanted", message.Kind)
	}
	frame := message.Surface
	if frame.BootID != "boot-v1" {
		t.Errorf("the boot is %q, the Rust client wrote %q", frame.BootID, "boot-v1")
	}
	if frame.ProjectionRevision != 7 || frame.SurfaceRevision != 3 {
		t.Errorf("the revisions are %d and %d, the Rust client wrote 7 and 3", frame.ProjectionRevision, frame.SurfaceRevision)
	}
	if frame.Width != 2 || frame.Height != 1 {
		t.Errorf("the surface is %dx%d, the Rust client wrote 2x1", frame.Width, frame.Height)
	}
	if len(frame.Cells) != 2 {
		t.Fatalf("the surface has %d cells, the Rust client wrote 2", len(frame.Cells))
	}
	if frame.Cells[0].Symbol != "a" {
		t.Errorf("the first cell says %q, the Rust client wrote %q", frame.Cells[0].Symbol, "a")
	}
	if frame.Cells[0].Fg != 0x02FF0000 {
		t.Errorf("the first cell is painted %x, the Rust client wrote %x", frame.Cells[0].Fg, 0x02FF0000)
	}
	if frame.Cells[1].Symbol != "→" {
		t.Errorf("the second cell says %q, the Rust client wrote %q", frame.Cells[1].Symbol, "→")
	}
	if frame.Cells[1].Modifier != 3 {
		t.Errorf("the second cell has the modifiers %d, the Rust client wrote 3", frame.Cells[1].Modifier)
	}
	if frame.Cells[1].Hyperlink == nil || *frame.Cells[1].Hyperlink != 0 {
		t.Error("the second cell has no link")
	}
	if frame.Cursor == nil || frame.Cursor.X != 1 || frame.Cursor.Y != 0 || !frame.Cursor.Visible {
		t.Errorf("the cursor is %v, the Rust client wrote it at 1,0 and shown", frame.Cursor)
	}
	if len(frame.Hyperlinks) != 1 || frame.Hyperlinks[0] != "https://example.com" {
		t.Errorf("the links are %v, the Rust client wrote one link", frame.Hyperlinks)
	}
	if len(frame.Panes) != 1 || frame.Panes[0].PaneID != "w1:p1" {
		t.Errorf("the panes are %v, the Rust client wrote one pane", frame.Panes)
	}
	if !frame.Panes[0].Focused {
		t.Error("the pane of the Rust client is not focused")
	}
}

// TestTheAnswerOfTheRustClient checks the answer of the daemon to the
// handshake.
func TestTheAnswerOfTheRustClient(t *testing.T) {
	want, ok := golden(t)["welcome"]
	if !ok {
		t.Fatal("there are no golden bytes for the answer")
	}
	message, err := DecodeServerMessage(want)
	if err != nil {
		t.Fatalf("the answer of the Rust client is not read: %v", err)
	}
	if message.Kind != ServerControl {
		t.Fatalf("the message is a %v, a control message was wanted", message.Kind)
	}
	if message.Control.Kind != ControlWelcome {
		t.Errorf("the control message is %q, the answer was wanted", message.Control.Kind)
	}
	welcome, err := DecodeWelcome(message.Control.Data)
	if err != nil {
		t.Fatalf("the answer is not JSON: %v", err)
	}
	if welcome.ServerVersion != "0.9.5" {
		t.Errorf("the version of the daemon is %q, the Rust client wrote %q", welcome.ServerVersion, "0.9.5")
	}
	if welcome.SnapshotCodec != CodecSnapshot {
		t.Errorf("the codec of the snapshot is %q, %q was wanted", welcome.SnapshotCodec, CodecSnapshot)
	}
	if !welcome.HasMethod("pane.focus") {
		t.Error("the daemon does not let the client focus a pane")
	}
}

// TestTheRequestMatchesTheRustClient checks the request of a method.
func TestTheRequestMatchesTheRustClient(t *testing.T) {
	want, ok := golden(t)["request"]
	if !ok {
		t.Fatal("there are no golden bytes for the request")
	}
	encoded := Encode(&EndpointRequest{
		BootID:  "boot-v1",
		Request: `{"id":"gpui-1","method":"pane.focus","params":{"pane_id":"w1:p1"}}`,
	})
	if string(encoded) != string(want) {
		t.Errorf("the request:\n the bytes of this client are %x\n the bytes of the Rust client are %x", encoded, want)
	}
}

// bytesIndex returns where the bytes are in the other bytes.
func bytesIndex(haystack, needle []byte) int {
	return strings.Index(string(haystack), string(needle))
}

// sameJSON says whether two things of JSON are the same.
func sameJSON(a, b any) bool {
	left, err := json.Marshal(a)
	if err != nil {
		return false
	}
	right, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

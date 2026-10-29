package endpoint

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/localipc"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func await(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func TestFrozenHerdrFrames(t *testing.T) {
	c := New("/tmp/herdr.sock")
	if err := c.handle(fixture(t, "endpoint-snapshot-v1.bin")); err != nil {
		t.Fatal(err)
	}
	if err := c.handle(fixture(t, "notification-v1.bin")); err != nil {
		t.Fatal(err)
	}
	s := c.Latest()
	if s.Snapshot.BootID != "boot-v1" || s.Snapshot.Revision != 7 || len(s.Snapshot.Commands) != 1 {
		t.Fatal(s.Snapshot)
	}
	n := s.Notifications[0]
	if n.Kind != "NeedsAttention" || n.Title != "codex needs attention" || n.Body != "repo · 1" || n.Sound != "Request" || n.PaneID != "w1:p1" || n.Position != "top-right" {
		t.Fatal(n)
	}
	// Caller mutations cannot poison the catalog shared with the reader.
	s.Snapshot.Commands[0].CommandID = "changed"
	if c.Latest().Snapshot.Commands[0].CommandID == "changed" {
		t.Fatal("mutable catalog escaped")
	}
	for i := 0; i < 70; i++ {
		if err := c.handle(fixture(t, "notification-v1.bin")); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.Latest().Notifications) != 64 {
		t.Fatal("unbounded notifications")
	}
	if err := c.handle([]byte{250, 1, 2, 3}); err != nil {
		t.Fatal("unknown frame not skipped", err)
	}
}
func TestBoundedCodec(t *testing.T) {
	for _, n := range []uint64{0, 250, 251, 65535, 65536, 1 << 32, 1 << 63} {
		d := decoder{b: appendUint(nil, n)}
		if d.uint() != n || d.finish() != nil {
			t.Fatal(n, d.err)
		}
	}
	if !bytes.Equal(control("", ""), []byte{20, 0, 0}) {
		t.Fatal("frozen control tag changed")
	}
	for _, payload := range [][]byte{{251}, {252, 0}, {254}, {255}} {
		d := decoder{b: payload}
		d.uint()
		if d.err == nil {
			t.Fatal("malformed varint accepted", payload)
		}
	}
	for _, n := range []uint32{0, MaxFrameSize + 1, 0xffffffff} {
		if _, e := readFrame(bytes.NewReader(binary.LittleEndian.AppendUint32(nil, n))); e == nil {
			t.Fatal("frame bound ignored", n)
		}
	}
	d := decoder{b: []byte{1, 255}}
	d.str()
	if d.err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}
func TestSocketDerivation(t *testing.T) {
	for a, w := range map[string]string{"/tmp/herdr.sock": "/tmp/herdr-client.sock", "/tmp/custom": "/tmp/custom-client.sock", "/tmp/foo.bar": "/tmp/foo-client.sock"} {
		if g := SocketPath(a); g != filepath.FromSlash(w) {
			t.Fatal(a, g, w)
		}
	}
}
func listener(t *testing.T) net.Listener {
	t.Helper()
	dir, e := os.MkdirTemp("", "hep-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	l, e := localipc.Listen(filepath.Join(dir, "c.sock"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { l.Close() })
	return l
}
func TestHandshakeAndCancellation(t *testing.T) {
	l := listener(t)
	serverErr := make(chan error, 1)
	go func() {
		conn, e := l.Accept()
		if e != nil {
			serverErr <- e
			return
		}
		defer conn.Close()
		b, e := readFrame(conn)
		if e != nil {
			serverErr <- e
			return
		}
		d := decoder{b: b}
		tag, kind, data := d.uint(), d.str(), d.str()
		var h struct {
			Generation int  `json:"generation"`
			Active     bool `json:"surface_active"`
		}
		if e = json.Unmarshal([]byte(data), &h); e != nil {
			serverErr <- e
			return
		}
		if tag != 20 || kind != "endpoint.hello.v1" || h.Generation != 1 || h.Active {
			serverErr <- fmt.Errorf("wrong hello %s", data)
			return
		}
		for _, name := range []string{"endpoint-welcome-v1.bin", "endpoint-snapshot-v1.bin", "notification-v1.bin"} {
			b, e := os.ReadFile(filepath.Join("testdata", name))
			if e != nil {
				serverErr <- e
				return
			}
			if e = writeFrame(conn, b); e != nil {
				serverErr <- e
				return
			}
		}
		var one [1]byte
		_, e = conn.Read(one[:])
		if e == io.EOF {
			e = nil
		}
		serverErr <- e
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := NewSocket(l.Addr().String())
	changes := make(chan State, 16)
	c.SetChanged(func() {
		select {
		case changes <- c.Latest():
		default:
		}
	})
	if e := c.Start(ctx); e != nil {
		t.Fatal(e)
	}
	await(t, func() bool { return len(c.Latest().Notifications) == 1 })
	select {
	case <-changes:
	default:
		t.Fatal("missing change callback")
	}
	cancel()
	await(t, func() bool { return !c.Latest().Connected })
	if e := <-serverErr; e != nil {
		t.Fatal(e)
	}
}
func TestImageLeaseRestored(t *testing.T) {
	l := listener(t)
	serverErr := make(chan error, 1)
	go func() {
		conn, e := l.Accept()
		if e != nil {
			serverErr <- e
			return
		}
		defer conn.Close()
		if _, e = readFrame(conn); e != nil {
			serverErr <- e
			return
		}
		w := fixture(t, "endpoint-welcome-v1.json")
		var m map[string]any
		json.Unmarshal(w, &m)
		m["methods"] = []string{"client_shell.surface.set", "pane.focus"}
		m["capabilities"] = []string{"surface_interest"}
		w, _ = json.Marshal(m)
		writeFrame(conn, control("endpoint.welcome.v1", string(w)))
		writeFrame(conn, fixture(t, "endpoint-snapshot-v1.bin"))
		for step := 0; step < 5; step++ {
			b, e := readFrame(conn)
			if e != nil {
				serverErr <- e
				return
			}
			d := decoder{b: b}
			tag := d.uint()
			switch step {
			case 0:
				if tag != 12 || d.uint() != 0 || d.uint() != 0 || d.uint() != 120 || d.uint() != 40 || d.option() {
					serverErr <- fmt.Errorf("resize %v", b)
					return
				}
			case 1, 2, 4:
				boot, request := d.str(), d.str()
				if tag != 15 || boot != "boot-v1" {
					serverErr <- fmt.Errorf("request %v", b)
					return
				}
				var r struct {
					ID, Method string
					Params     map[string]any
				}
				if e = json.Unmarshal([]byte(request), &r); e != nil {
					serverErr <- e
					return
				}
				if step == 2 {
					if r.Method != "pane.focus" || r.Params["pane_id"] != "w1:p1" {
						serverErr <- fmt.Errorf("focus %s", request)
						return
					}
				} else if r.Method != "client_shell.surface.set" || r.Params["active"] != (step == 1) {
					serverErr <- fmt.Errorf("surface %s", request)
					return
				}
				reply := appendString(appendString([]byte{18}, boot), r.ID)
				reply = append(reply, 1)
				data := []byte(`{"id":"` + r.ID + `","result":{}}`)
				reply = append(appendUint(reply, uint64(len(data))), data...)
				if e = writeFrame(conn, reply); e != nil {
					serverErr <- e
					return
				}
			case 3:
				if tag != 2 || d.uint() != 1 || d.str() != "w1:p1" || d.str() != "png" || !bytes.Equal(d.take(d.uint()), []byte{1, 2, 3}) {
					serverErr <- fmt.Errorf("image %v", b)
					return
				}
			}
			if e = d.finish(); e != nil {
				serverErr <- e
				return
			}
		}
		serverErr <- nil
		io.Copy(io.Discard, conn)
	}()
	c := NewSocket(l.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if e := c.Start(ctx); e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	await(t, func() bool { return c.Latest().Snapshot != nil })
	if e := c.SendImage(ctx, "w1:p1", "png", []byte{1}); e == nil {
		t.Fatal("missing geometry accepted")
	}
	c.SetSurfaceSize(120, 40)
	if e := c.SendImage(ctx, "w1:p1", "png", []byte{1, 2, 3}); e != nil {
		t.Fatal(e)
	}
	if e := <-serverErr; e != nil {
		t.Fatal(e)
	}
}

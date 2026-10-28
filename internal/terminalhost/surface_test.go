package terminalhost

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/localipc"
	"github.com/Maidang1/herdr-ui/internal/surfaceview"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// Fake endpoint speaks the public framed binary protocol, not Host internals.
// Surface fixtures were emitted by the authoritative Rust herdr-protocol codec.
const hostSurfaceGolden = "0d04626f6f7407fbfb000103e7958cfc332211fffb0001fb010100010001010100000106011368747470733a2f2f6578616d706c652e636f6d00010470616e65fb2c010000010100000101000102fbf40101010100000c180000000000"
const hostPatchGolden = "1304626f6f7407fbfb00fbfc00010000010161fc332211fffb0001fb0101000100010470616e65fb2c010000010100000101000102fbf40101010100000c1800"

type fakeSurfaceEndpoint struct {
	t         *testing.T
	l         net.Listener
	conn      net.Conn
	mu        sync.Mutex
	messages  chan []byte
	ready     chan struct{}
	done      chan struct{}
	socket    string
	holdFocus chan struct{}
	holdTab   string
	focusSeen chan struct{}
}

func hostUint(b []byte, n uint64) []byte {
	switch {
	case n < 251:
		return append(b, byte(n))
	case n <= 65535:
		return binary.LittleEndian.AppendUint16(append(b, 251), uint16(n))
	case n <= 0xffffffff:
		return binary.LittleEndian.AppendUint32(append(b, 252), uint32(n))
	default:
		return binary.LittleEndian.AppendUint64(append(b, 253), n)
	}
}
func hostString(b []byte, s string) []byte { return append(hostUint(b, uint64(len(s))), s...) }
func hostRead(r io.Reader) ([]byte, error) {
	var h [4]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return nil, e
	}
	n := binary.LittleEndian.Uint32(h[:])
	if n == 0 || n > endpoint.MaxFrameSize {
		return nil, fmt.Errorf("bad frame %d", n)
	}
	b := make([]byte, n)
	_, e := io.ReadFull(r, b)
	return b, e
}
func hostReadUint(b *[]byte) uint64 {
	if len(*b) == 0 {
		return 0
	}
	x := (*b)[0]
	*b = (*b)[1:]
	switch x {
	case 251:
		if len(*b) < 2 {
			return 0
		}
		n := binary.LittleEndian.Uint16(*b)
		*b = (*b)[2:]
		return uint64(n)
	case 252:
		if len(*b) < 4 {
			return 0
		}
		n := binary.LittleEndian.Uint32(*b)
		*b = (*b)[4:]
		return uint64(n)
	default:
		return uint64(x)
	}
}
func hostReadString(b *[]byte) string {
	n := hostReadUint(b)
	if n > uint64(len(*b)) {
		return ""
	}
	s := string((*b)[:n])
	*b = (*b)[n:]
	return s
}
func (f *fakeSurfaceEndpoint) write(b []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return net.ErrClosed
	}
	out := binary.LittleEndian.AppendUint32(nil, uint32(len(b)))
	out = append(out, b...)
	_, e := f.conn.Write(out)
	return e
}
func (f *fakeSurfaceEndpoint) control(kind string, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return f.write(hostString(hostString([]byte{20}, kind), string(b)))
}
func newFakeSurfaceEndpoint(t *testing.T, heldTab ...string) *fakeSurfaceEndpoint {
	t.Helper()
	dir, e := os.MkdirTemp("", "hs-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	api := filepath.Join(dir, "api.sock")
	l, e := localipc.Listen(endpoint.SocketPath(api))
	if e != nil {
		t.Fatal(e)
	}
	f := &fakeSurfaceEndpoint{t: t, l: l, socket: api, messages: make(chan []byte, 128), ready: make(chan struct{}), done: make(chan struct{})}
	t.Cleanup(func() {
		l.Close()
		f.mu.Lock()
		if f.conn != nil {
			f.conn.Close()
		}
		f.mu.Unlock()
	})
	if len(heldTab) > 0 {
		f.holdTab = heldTab[0]
		f.holdFocus = make(chan struct{})
		f.focusSeen = make(chan struct{}, 1)
	}
	go f.run()
	return f
}
func (f *fakeSurfaceEndpoint) run() {
	defer close(f.done)
	for {
		c, e := f.l.Accept()
		if e != nil {
			return
		}
		f.serve(c)
	}
}
func (f *fakeSurfaceEndpoint) serve(c net.Conn) {
	f.mu.Lock()
	f.conn = c
	f.mu.Unlock()
	defer c.Close()
	b, e := hostRead(c)
	if e != nil {
		return
	}
	f.messages <- b
	helloPayload := b[1:]
	if hostReadString(&helloPayload) != "endpoint.hello.v1" {
		return
	}
	var hello struct {
		Active bool `json:"surface_active"`
	}
	if json.Unmarshal([]byte(hostReadString(&helloPayload)), &hello) != nil {
		return
	}
	active := hello.Active
	f.control("endpoint.welcome.v1", map[string]any{"generation": 1, "snapshot_codec": "shell.snapshot.v1", "surface_codec": "shell.surface.v1", "input_codec": "shell.input.semantic.v1", "blob_codec": "shell.blob.v1", "methods": []string{"client_shell.surface.set", "tab.focus", "pane.focus"}, "capabilities": []string{"surface_interest"}})
	f.control("shell.snapshot.v1", map[string]any{"boot_id": "boot", "revision": 7, "focused_tab_id": "tab", "focused_pane_id": "pane"})
	select {
	case <-f.ready:
	default:
		close(f.ready)
	}
	for {
		b, e = hostRead(c)
		if e != nil {
			return
		}
		f.messages <- b
		if len(b) > 0 && b[0] == 15 {
			rest := b[1:]
			boot := hostReadString(&rest)
			raw := hostReadString(&rest)
			var req struct {
				ID, Method string
				Params     map[string]any
			}
			if json.Unmarshal([]byte(raw), &req) != nil {
				return
			}
			if req.Method == "client_shell.surface.set" {
				active, _ = req.Params["active"].(bool)
			}
			// Authoritative server/headless/endpoint_requests.rs:72 rejects every
			// ordinary endpoint API while passive; only surface.set bypasses it.
			inactive := req.Method != "client_shell.surface.set" && !active
			if req.Method == "tab.focus" && !inactive {
				if f.holdFocus != nil && req.Params["tab_id"] == f.holdTab {
					select {
					case f.focusSeen <- struct{}{}:
					default:
					}
					select {
					case <-f.holdFocus:
					case <-time.After(3 * time.Second):
						return
					}
				}
				f.control("shell.snapshot.v1", map[string]any{"boot_id": "boot", "revision": 7, "focused_tab_id": req.Params["tab_id"], "focused_pane_id": "pane"})
			}
			response := map[string]any{"id": req.ID, "result": map[string]any{"ok": true}}
			if inactive {
				response = map[string]any{"id": req.ID, "error": map[string]any{"code": "surface_inactive", "message": "this method requires an active client shell surface"}}
			}
			data, _ := json.Marshal(response)
			reply := hostString(hostString([]byte{18}, boot), req.ID)
			reply = append(reply, 1)
			reply = append(hostUint(reply, uint64(len(data))), data...)
			if f.write(reply) != nil {
				return
			}
		}
	}
}
func (f *fakeSurfaceEndpoint) next(t *testing.T, tag byte) []byte {
	t.Helper()
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	for {
		select {
		case b := <-f.messages:
			if len(b) > 0 && b[0] == tag {
				return b
			}
		case <-timer.C:
			t.Fatalf("waiting for endpoint tag %d", tag)
			return nil
		}
	}
}
func (f *fakeSurfaceEndpoint) fixture(t *testing.T, h string) {
	t.Helper()
	b, e := hex.DecodeString(h)
	if e != nil {
		t.Fatal(e)
	}
	if e = f.write(b); e != nil {
		t.Fatal(e)
	}
}
func surfaceWait(t *testing.T, posts chan func(), condition func() bool) {
	t.Helper()
	until := time.NewTimer(3 * time.Second)
	defer until.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		if condition() {
			return
		}
		select {
		case fn := <-posts:
			fn()
		case <-tick.C:
		case <-until.C:
			t.Fatal("native surface condition timed out")
		}
	}
}
func surfaceHost(t *testing.T, f *fakeSurfaceEndpoint) (*Host, Binding, chan func(), *atomic.Int32) {
	t.Helper()
	posts := make(chan func(), 128)
	calls := &atomic.Int32{}
	h := New(Config{NativeSurface: true, Target: herdr.Target{Socket: f.socket}, Post: func(fn func()) { posts <- fn }})
	h.create = func(terminal.Options) (instance, error) {
		calls.Add(1)
		return nil, fmt.Errorf("attach factory must not run")
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := h.Close(ctx); err != nil {
			t.Errorf("bounded close: %v", err)
		}
	})
	bs, e := h.Reconcile([]Spec{{TargetID: 1, TerminalID: "terminal", PaneID: "pane"}})
	if e != nil {
		t.Fatal(e)
	}
	h.ConfigureSurface("tab", "pane", 500, 200, true)
	surfaceWait(t, posts, func() bool { h.mu.Lock(); defer h.mu.Unlock(); e := h.lookup(bs[0]); return e != nil && e.installed })
	return h, bs[0], posts, calls
}
func hostRequest(t *testing.T, b []byte) (string, map[string]any) {
	t.Helper()
	rest := b[1:]
	if boot := hostReadString(&rest); boot != "boot" {
		t.Fatalf("request boot %q", boot)
	}
	raw := hostReadString(&rest)
	var r struct {
		Method string
		Params map[string]any
	}
	if e := json.Unmarshal([]byte(raw), &r); e != nil {
		t.Fatal(e)
	}
	return r.Method, r.Params
}
func TestNativeSurfaceEndpointRenderAndSemanticInput(t *testing.T) {
	f := newFakeSurfaceEndpoint(t)
	h, b, posts, calls := surfaceHost(t, f)
	hello := f.next(t, 20)
	rest := hello[1:]
	if hostReadString(&rest) != "endpoint.hello.v1" {
		t.Fatal("wrong handshake")
	}
	var hv map[string]any
	if e := json.Unmarshal([]byte(hostReadString(&rest)), &hv); e != nil {
		t.Fatal(e)
	}
	if hv["surface_active"] != false || hv["surface_reuse"] == true || hv["surface_delta"] == true {
		t.Fatalf("unsafe native negotiation %+v", hv)
	}
	resize := f.next(t, 12)
	want := h.surface.selection()
	expected := hostUint(hostUint([]byte{12, 0, 0}, uint64(want.cols)), uint64(want.rows))
	expected = append(expected, 0)
	if !bytes.Equal(resize, expected) {
		t.Fatalf("resize got %x want %x", resize, expected)
	}
	method, p := hostRequest(t, f.next(t, 15))
	if method != "client_shell.surface.set" || p["active"] != true {
		t.Fatalf("activation %s %+v", method, p)
	}
	method, p = hostRequest(t, f.next(t, 15))
	if method != "tab.focus" || p["tab_id"] != "tab" {
		t.Fatalf("selection %s %+v", method, p)
	}
	method, p = hostRequest(t, f.next(t, 15))
	if method != "pane.focus" || p["pane_id"] != "pane" {
		t.Fatalf("pane focus %s %+v", method, p)
	}
	if got := f.next(t, 18); !bytes.Equal(got, []byte{18, 1}) {
		t.Fatalf("host focus %x", got)
	}
	f.fixture(t, hostSurfaceGolden)
	surfaceWait(t, posts, func() bool { return h.surface.snapshot().Surface != nil })
	tt := ui.NewTester(func(c *ui.Context) { h.View(c, b).Fill().AutoFocus() }, 180, 70)
	h.mu.Lock()
	inst, ok := h.lookup(b).owned.(*surfaceInstance)
	h.mu.Unlock()
	if !ok || inst.frame == nil {
		t.Fatal("native frame did not reach view")
	}
	searches := make(chan terminal.SearchStatus, 8)
	h.cfg.OnSearch = func(_ Binding, status terminal.SearchStatus) { searches <- status }
	inst.view.Search("界")
	if found := receive(t, searches); found.Total != 1 {
		t.Fatalf("surface text did not reach search: %+v", found)
	}
	if inst.frame.Frame.Cells[0].Symbol != "界" {
		t.Fatal("rendered frame differs")
	}
	f.fixture(t, hostPatchGolden)
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot().Surface; return s != nil && s.SurfaceRevision == 252 })
	tt.Frame()
	inst.view.Search("a")
	var found terminal.SearchStatus
	for len(searches) > 0 {
		found = <-searches
	}
	if found.Text != "a" || found.Total != 1 {
		t.Fatalf("patched text did not reach search: %+v", found)
	}
	if inst.frame.SurfaceRevision != 252 || inst.frame.Frame.Cells[0].Symbol != "a" {
		t.Fatal("patch did not reach renderer")
	}
	tt.Type("你好")
	wantInput, _ := hex.DecodeString("0d0470616e65010106e4bda0e5a5bd")
	if got := f.next(t, 13); !bytes.Equal(got, wantInput) {
		t.Fatalf("typed event got %x want %x", got, wantInput)
	}
	inst.eventText("a\nb", true)
	wantInput, _ = hex.DecodeString("0d0470616e65010303610a62")
	if got := f.next(t, 13); !bytes.Equal(got, wantInput) {
		t.Fatalf("paste got %x want %x", got, wantInput)
	}
	tt.Key(0, ui.KeyEnter)
	wantInput, _ = hex.DecodeString("0d0470616e650100010000010000000000")
	if got := f.next(t, 13); !bytes.Equal(got, wantInput) {
		t.Fatalf("Enter got %x want %x", got, wantInput)
	}
	// A pane absent from the complete canvas must become blank and read-only.
	absent, _ := hex.DecodeString(hostSurfaceGolden)
	absent[8] = 253
	absent = bytes.Replace(absent, []byte("pane"), []byte("gone"), 1)
	if err := f.write(absent); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot().Surface; return s != nil && s.SurfaceRevision == 253 })
	tt.Frame()
	if inst.frame != nil {
		t.Fatal("missing pane retained a writable rendered frame")
	}
	inst.eventText("absent-pane", false)
	restored, _ := hex.DecodeString(hostSurfaceGolden)
	restored[8] = 254
	if err := f.write(restored); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot().Surface; return s != nil && s.SurfaceRevision == 254 })
	tt.Frame()
	inst.eventText("restored", false)
	expected = hostString([]byte{13}, "pane")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "restored")
	if got := f.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("absent pane accepted input: %x", got)
	}
	if calls.Load() != 0 {
		t.Fatal("native path spawned attach factory")
	}
}

func TestNativeSurfaceReconnectDoesNotReplayOldInput(t *testing.T) {
	f := newFakeSurfaceEndpoint(t)
	h, b, posts, _ := surfaceHost(t, f)
	f.next(t, 18)
	f.fixture(t, hostSurfaceGolden)
	surfaceWait(t, posts, func() bool { return h.surface.snapshot().Surface != nil })
	tt := ui.NewTester(func(c *ui.Context) { h.View(c, b).Fill().AutoFocus() }, 180, 70)
	h.mu.Lock()
	p := h.lookup(b).owned.(*surfaceInstance)
	h.mu.Unlock()
	f.mu.Lock()
	f.conn.Close()
	f.mu.Unlock()
	surfaceWait(t, posts, func() bool { h.surface.mu.Lock(); defer h.surface.mu.Unlock(); return h.surface.client == nil })
	// The UI may still display its last frame while the endpoint reconnects.
	// An event committed during that interval must never reach the new socket,
	// even when the process boot ID is unchanged.
	p.eventText("stale", false)
	f.next(t, 20) // the next connection's handshake
	f.next(t, 18) // all synchronous selection/geometry restoration has completed
	f.fixture(t, hostSurfaceGolden)
	surfaceWait(t, posts, func() bool { return h.surface.snapshot().Surface != nil })
	tt.Frame()
	p.eventText("fresh", false)
	expected := hostString([]byte{13}, "pane")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "fresh")
	if got := f.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("reconnect replayed stale input: got %x want %x", got, expected)
	}
}

func TestNativeSurfaceSelectionAndWindowsRemainIndependent(t *testing.T) {
	a, b := newFakeSurfaceEndpoint(t), newFakeSurfaceEndpoint(t)
	first, firstBinding, firstPosts, calls := surfaceHost(t, a)
	second, _, secondPosts, _ := surfaceHost(t, b)
	a.next(t, 18)
	b.next(t, 18)
	a.fixture(t, hostSurfaceGolden)
	surfaceWait(t, firstPosts, func() bool { return first.surface.snapshot().Surface != nil })
	tt := ui.NewTester(func(c *ui.Context) { first.View(c, firstBinding).Fill().AutoFocus() }, 180, 70)
	first.mu.Lock()
	p := first.lookup(firstBinding).owned.(*surfaceInstance)
	first.mu.Unlock()
	// A selection update precedes queued input and cannot send to the pane
	// belonging to the canvas being retired.
	first.ConfigureSurface("other-tab", "other-pane", 300, 100, true)
	p.eventText("stale-selection", false)
	method, params := hostRequest(t, a.next(t, 15))
	if method != "tab.focus" || params["tab_id"] != "other-tab" {
		t.Fatalf("changed selection %s %+v", method, params)
	}
	resized := a.next(t, 12)
	if len(resized) == 0 {
		t.Fatal("missing changed geometry")
	}
	surfaceWait(t, firstPosts, func() bool {
		s := first.surface.snapshot()
		return s.Snapshot != nil && s.Snapshot.FocusedTabID == "other-tab"
	})
	if got := second.surface.selection(); got.tab != "tab" || got.pane != "pane" {
		t.Fatalf("second window changed %+v", got)
	}
	// Return to the first canvas, then a current event provides a FIFO barrier:
	// any wrongly retained prior event would arrive before it.
	first.ConfigureSurface("tab", "pane", 500, 200, true)
	a.next(t, 15)
	a.next(t, 12)
	surfaceWait(t, firstPosts, func() bool {
		s := first.surface.snapshot()
		return s.Snapshot != nil && s.Snapshot.FocusedTabID == "tab"
	})
	tt.Frame()
	p.eventText("current", false)
	expected := hostString([]byte{13}, "pane")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "current")
	if got := a.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("selection replayed stale input: %x", got)
	}
	if calls.Load() != 0 {
		t.Fatal("native window used attach")
	}
	surfaceWait(t, secondPosts, func() bool { return second.surface.snapshot().Snapshot != nil })
}

func TestNativeSurfaceShutdownInterruptsIncompleteHandshake(t *testing.T) {
	dir, err := os.MkdirTemp("", "hs-close-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	api := filepath.Join(dir, "api.sock")
	listener, err := localipc.Listen(endpoint.SocketPath(api))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started := make(chan struct{})
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err = hostRead(conn); err != nil {
			return
		}
		close(started)
		var b [1]byte
		conn.Read(b[:])
	}()
	h := New(Config{NativeSurface: true, Target: herdr.Target{Socket: api}, Post: func(func()) {}})
	h.ConfigureSurface("tab", "pane", 500, 200, true)
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("endpoint never connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = h.Close(ctx); err != nil {
		t.Fatalf("shutdown waited for handshake timeout: %v", err)
	}
	select {
	case <-ended:
	case <-ctx.Done():
		t.Fatal("endpoint socket remained open after shutdown")
	}
}

func TestNativeSurfaceQueuedInputKeepsSelectionGeneration(t *testing.T) {
	f := newFakeSurfaceEndpoint(t, "away")
	h, b, posts, _ := surfaceHost(t, f)
	f.next(t, 18)
	f.fixture(t, hostSurfaceGolden)
	surfaceWait(t, posts, func() bool { return h.surface.snapshot().Surface != nil })
	tt := ui.NewTester(func(c *ui.Context) { h.View(c, b).Fill().AutoFocus() }, 180, 70)
	h.mu.Lock()
	p := h.lookup(b).owned.(*surfaceInstance)
	h.mu.Unlock()
	ev, err := endpoint.TextEvent("old-generation", false)
	if err != nil {
		t.Fatal(err)
	}
	stale := surfaceInput{selection: p.frameSelection, connection: p.frameConnection, binding: b, boot: p.frame.BootID, target: "pane", event: ev}
	h.ConfigureSurface("away", "pane", 500, 200, true)
	select {
	case <-f.focusSeen:
	case <-time.After(3 * time.Second):
		t.Fatal("focus request did not enter barrier")
	}
	// The worker is blocked awaiting the server's away-focus response. Return
	// to exactly the same tab, pane and geometry while the old event is queued.
	h.ConfigureSurface("tab", "pane", 500, 200, true)
	h.surface.inputs <- stale
	close(f.holdFocus)
	surfaceWait(t, posts, func() bool {
		s := h.surface.snapshot()
		return s.Snapshot != nil && s.Snapshot.FocusedTabID == "tab" && h.surface.selection().generation > stale.selection.generation
	})
	// Wait for both server-side navigation operations, not merely local desire.
	hostRequest(t, f.next(t, 15))
	method, params := hostRequest(t, f.next(t, 15))
	if method != "tab.focus" || params["tab_id"] != "tab" {
		t.Fatalf("return focus %s %+v", method, params)
	}
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot(); return s.Snapshot != nil && s.Snapshot.FocusedTabID == "tab" })
	tt.Frame()
	p.eventText("new-generation", false)
	expected := hostString([]byte{13}, "pane")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "new-generation")
	if got := f.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("selection generation replayed: %x", got)
	}
}

const hostPopupGolden = "0d04626f6f7407fbfd000103e7958cfc332211fffb0001fb010100010001010100000106011368747470733a2f2f6578616d706c652e636f6d00010470616e65fb2c010000010100000101000102fbf40101010100000c18000105706f70757005506f70757000000103e7958cfc332211fffb0001fb010100010001010100000106011368747470733a2f2f6578616d706c652e636f6d0000000c18000000"

func TestNativeSurfacePopupInputRequiresCurrentSnapshotFocus(t *testing.T) {
	f := newFakeSurfaceEndpoint(t)
	h, b, posts, _ := surfaceHost(t, f)
	f.next(t, 18)
	f.fixture(t, hostPopupGolden)
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot().Surface; return s != nil && s.Popup != nil })
	tt := ui.NewTester(func(c *ui.Context) { h.View(c, b).Fill().AutoFocus() }, 180, 70)
	h.mu.Lock()
	p := h.lookup(b).owned.(*surfaceInstance)
	h.mu.Unlock()
	if p.frame == nil || p.frame.Popup == nil {
		t.Fatal("popup not rendered")
	}
	if err := f.control("shell.snapshot.v1", map[string]any{"boot_id": "boot", "revision": 8, "focused_tab_id": "tab", "focused_pane_id": "other"}); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool {
		s := h.surface.snapshot()
		return s.Snapshot != nil && s.Snapshot.FocusedPaneID == "other"
	})
	// No View call: the user can commit text while the screen still carries
	// the old popup. Dispatch must validate the newer snapshot's pane focus.
	p.eventText("stale-popup", false)
	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case got := <-f.messages:
		timer.Stop()
		t.Fatalf("old popup accepted after focus changed: %x", got)
	case <-timer.C:
	}
	if err := f.control("shell.snapshot.v1", map[string]any{"boot_id": "boot", "revision": 9, "focused_tab_id": "tab", "focused_pane_id": "pane"}); err != nil {
		t.Fatal(err)
	}
	restored, _ := hex.DecodeString(hostPopupGolden)
	restored[6] = 9
	restored[8] = 254
	if err := f.write(restored); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool {
		s := h.surface.snapshot()
		return s.Surface != nil && s.Surface.SurfaceRevision == 254 && s.Snapshot.FocusedPaneID == "pane"
	})
	tt.Frame()
	p.eventText("current-popup", false)
	expected := hostString([]byte{14}, "popup")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "current-popup")
	if got := f.next(t, 14); !bytes.Equal(got, expected) {
		t.Fatalf("current popup payload %x", got)
	}
	// A popup present elsewhere in the canvas must not intercept input from
	// a rendered pane whose metadata says that it is not focused.
	unfocused := append([]byte(nil), restored...)
	unfocused[8] = 255
	unfocused[82] = 0
	decoded, err := endpoint.DecodeSurfaceFrame(unfocused)
	if err != nil || len(decoded.Panes) != 1 || decoded.Panes[0].Focused {
		t.Fatalf("unfocused fixture invalid: %v", err)
	}
	if err = f.write(unfocused); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot(); return s.Surface != nil && s.Surface.SurfaceRevision == 255 })
	tt.Frame()
	p.eventText("pane-only", false)
	expected = hostString([]byte{13}, "pane")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "pane-only")
	select {
	case got := <-f.messages:
		if !bytes.Equal(got, expected) {
			t.Fatalf("unfocused pane input routed to popup: %x", got)
		}
	case <-time.After(100 * time.Millisecond):
		// The dispatcher may reject all pane input while a popup is active.
	}
}

func renderedSurfaceInputHost(t *testing.T) (*fakeSurfaceEndpoint, *Host, *surfaceInstance, *ui.Tester, chan func()) {
	t.Helper()
	f := newFakeSurfaceEndpoint(t)
	h, b, posts, _ := surfaceHost(t, f)
	f.next(t, 18)
	f.fixture(t, hostSurfaceGolden)
	surfaceWait(t, posts, func() bool { return h.surface.snapshot().Surface != nil })
	tt := ui.NewTester(func(c *ui.Context) { h.View(c, b).Fill().AutoFocus() }, 180, 70)
	h.mu.Lock()
	p := h.lookup(b).owned.(*surfaceInstance)
	h.mu.Unlock()
	if p.frame == nil {
		t.Fatal("surface input has no rendered source frame")
	}
	return f, h, p, tt, posts
}
func paneTextPayload(text string) []byte {
	b := hostString([]byte{13}, "pane")
	b = append(b, 1, 1)
	return hostString(b, text)
}
func TestNativeWindowsAltGrCommitsTextWithoutControlChord(t *testing.T) {
	f, _, p, _, _ := renderedSurfaceInputHost(t)
	p.windowsTextKeys = true
	p.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyQ, Mods: ui.Ctrl | ui.Alt})
	p.input(ui.InputEvent{Kind: ui.InputText, Text: "@"})
	p.input(ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyQ, Mods: ui.Ctrl | ui.Alt})
	p.eventText("after", false)
	for _, text := range []string{"@", "after"} {
		if got := f.next(t, 13); !bytes.Equal(got, paneTextPayload(text)) {
			t.Fatalf("AltGr emitted chord or duplicate: got %x want text %q", got, text)
		}
	}
}
func TestNativeWindowsCtrlAltChordWithoutTextSendsOnRelease(t *testing.T) {
	f, _, p, _, _ := renderedSurfaceInputHost(t)
	p.windowsTextKeys = true
	p.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyQ, Mods: ui.Ctrl | ui.Alt})
	timer := time.NewTimer(20 * time.Millisecond)
	select {
	case got := <-f.messages:
		timer.Stop()
		t.Fatalf("ambiguous CtrlAlt key sent before text resolution: %x", got)
	case <-timer.C:
	}
	p.input(ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyQ, Mods: ui.Ctrl | ui.Alt})
	// Rust Key{Char('q'),modifiers:6,kind:Press,repeat_count:1, all
	// optional fields None,tracks_release:false} inside one pane-input event.
	expected, _ := hex.DecodeString("0d0470616e6501000f710600010000000000")
	if got := f.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("CtrlAlt release got %x want %x", got, expected)
	}
	p.eventText("after", false)
	if got := f.next(t, 13); !bytes.Equal(got, paneTextPayload("after")) {
		t.Fatalf("CtrlAlt key duplicated after release: %x", got)
	}
}
func TestNativeConsumedPrefixLetterDoesNotLeakCommittedText(t *testing.T) {
	f, h, p, _, _ := renderedSurfaceInputHost(t)
	h.cfg.OnKey = func(_ Binding, _ ui.Modifiers, key ui.Key, _ InputState) KeyResult {
		return KeyResult{Consumed: key == ui.KeyH}
	}
	if !p.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyH}) {
		t.Fatal("prefix letter not consumed")
	}
	p.input(ui.InputEvent{Kind: ui.InputText, Text: "h"})
	p.input(ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyH})
	p.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyA})
	p.input(ui.InputEvent{Kind: ui.InputText, Text: "a"})
	p.input(ui.InputEvent{Kind: ui.InputKeyUp, Key: ui.KeyA})
	if got := f.next(t, 13); !bytes.Equal(got, paneTextPayload("a")) {
		t.Fatalf("consumed prefix text leaked or following text lost: %x", got)
	}
	p.eventText("after", false)
	if got := f.next(t, 13); !bytes.Equal(got, paneTextPayload("after")) {
		t.Fatalf("consumed prefix caused extra input: %x", got)
	}
}
func TestNativePaneInputQueuedBeforePopupAppearsIsDropped(t *testing.T) {
	f, h, p, tt, posts := renderedSurfaceInputHost(t)
	f.fixture(t, hostPopupGolden)
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot(); return s.Surface != nil && s.Surface.Popup != nil })
	// The latest server state has a popup, but the last rendered frame did
	// not. Its input must not slip underneath the new popup into the pane.
	p.eventText("stale-pane", false)
	timer := time.NewTimer(100 * time.Millisecond)
	select {
	case got := <-f.messages:
		timer.Stop()
		t.Fatalf("old pane accepted input beneath new popup: %x", got)
	case <-timer.C:
	}
	tt.Frame()
	p.eventText("current-popup", false)
	expected := hostString([]byte{14}, "popup")
	expected = append(expected, 1, 1)
	expected = hostString(expected, "current-popup")
	if got := f.next(t, 14); !bytes.Equal(got, expected) {
		t.Fatalf("current popup input missing: %x", got)
	}
}

func TestNativeSurfaceShiftTabEncodesBackTab(t *testing.T) {
	f, _, p, _, _ := renderedSurfaceInputHost(t)
	p.input(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab, Mods: ui.Shift})
	// ClientPaneInputEvent::Key, ClientKeyCode::BackTab=11, Shift=1,
	// Press=0, repeat_count=1, with no generated/physical/Windows fields.
	expected, _ := hex.DecodeString("0d0470616e6501000b0100010000000000")
	if got := f.next(t, 13); !bytes.Equal(got, expected) {
		t.Fatalf("ShiftTab got %x want BackTab %x", got, expected)
	}
}
func TestNativeSurfacePopupMouseReportingOverridesUnderlyingPane(t *testing.T) {
	f, h, p, tt, posts := renderedSurfaceInputHost(t)
	popup, _ := hex.DecodeString(hostPopupGolden)
	popup[83] = 0
	popup[152] = 1
	decoded, err := endpoint.DecodeSurfaceFrame(popup)
	if err != nil || decoded.Panes[0].MouseReporting || !decoded.Popup.MouseReporting {
		t.Fatalf("popup reporting fixture: %v", err)
	}
	if err = f.write(popup); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, posts, func() bool { s := h.surface.snapshot(); return s.Surface != nil && s.Surface.Popup != nil })
	tt.Frame()
	fallback := 0
	h.cfg.OnScroll = func(Binding, int) { fallback++ }
	_, height := surfaceview.CellMetrics(h.cfg.Font)
	events := []struct {
		input ui.InputEvent
		wire  []byte
	}{
		{ui.InputEvent{Kind: ui.InputPointerDown, Button: 0, X: 5, Y: 3}, []byte{2, 0, 0, 0, 0, 0, 0, 0, 0}},
		{ui.InputEvent{Kind: ui.InputPointerMove, X: 5, Y: 3, Mods: ui.Shift}, []byte{2, 2, 0, 0, 0, 0, 0, 1, 0}},
		{ui.InputEvent{Kind: ui.InputPointerUp, Button: 0, X: 5, Y: 3}, []byte{2, 1, 0, 0, 0, 0, 0, 0, 0}},
		{ui.InputEvent{Kind: ui.InputScroll, DY: height * 2, Precise: true}, []byte{2, 5, 0, 0, 0, 0, 0, 2}},
	}
	for _, tc := range events {
		if !p.input(tc.input) {
			t.Fatalf("popup did not consume mouse kind %v", tc.input.Kind)
		}
		expected := hostString([]byte{14}, "popup")
		expected = append(expected, 1)
		expected = append(expected, tc.wire...)
		if got := f.next(t, 14); !bytes.Equal(got, expected) {
			t.Fatalf("popup mouse kind %v: got %x want %x", tc.input.Kind, got, expected)
		}
	}
	if fallback != 0 {
		t.Fatal("popup wheel fell through to underlying pane scrollback")
	}
}
func TestNativeSurfaceScrollNormalization(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		event          ui.InputEvent
		height         float32
		want           int
	}{
		{"windows-default", "windows", ui.InputEvent{DY: 99}, 20, 3},
		{"windows-user-setting", "windows", ui.InputEvent{DY: 165}, 20, 5},
		{"windows-negative", "windows", ui.InputEvent{DY: -99}, 20, -3},
		{"unix-notch", "darwin", ui.InputEvent{DY: 40}, 20, 3},
		{"unix-negative", "linux", ui.InputEvent{DY: -40}, 20, -3},
		{"horizontal", "linux", ui.InputEvent{DX: 40}, 20, 3},
		{"precise-cells", "windows", ui.InputEvent{DY: 40, Precise: true}, 20, 2},
		{"cap-positive", "windows", ui.InputEvent{DY: 3300000}, 20, 1000},
		{"cap-negative", "windows", ui.InputEvent{DY: -3300000}, 20, -1000},
		{"zero", "windows", ui.InputEvent{}, 20, 0},
		{"nan", "windows", ui.InputEvent{DY: float32(math.NaN())}, 20, 0},
		{"infinite", "windows", ui.InputEvent{DY: float32(math.Inf(1))}, 20, 0},
		{"invalid-height", "windows", ui.InputEvent{DY: 99}, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var remainder float32
			if got := surfaceScrollRowsFor(tc.platform, tc.event, tc.height, &remainder); got != tc.want {
				t.Fatalf("rows got %d want %d, remainder %g", got, tc.want, remainder)
			}
		})
	}
}
func TestNativeSurfacePreciseScrollAccumulatesWithoutAmplification(t *testing.T) {
	var remainder float32
	for i, want := range []int{0, 0, 0, 1} {
		if got := surfaceScrollRowsFor("windows", ui.InputEvent{DY: 5, Precise: true}, 20, &remainder); got != want {
			t.Fatalf("sub-row event %d: got %d want %d", i, got, want)
		}
	}
	if remainder != 0 {
		t.Fatalf("whole row left remainder %g", remainder)
	}
	for i, tc := range []struct {
		dy   float32
		want int
	}{{15, 0}, {-5, 0}, {-10, 0}, {-20, -1}} {
		if got := surfaceScrollRowsFor("darwin", ui.InputEvent{DY: tc.dy, Precise: true}, 20, &remainder); got != tc.want {
			t.Fatalf("sign reversal %d: got %d want %d", i, got, tc.want)
		}
	}
	if remainder != 0 {
		t.Fatalf("opposite motion left remainder %g", remainder)
	}
}

func TestFakeSurfaceEndpointRejectsFocusWhileInactive(t *testing.T) {
	f := newFakeSurfaceEndpoint(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := endpoint.NewSurface(f.socket)
	defer client.Close()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	surfaceWait(t, nil, func() bool { return client.Latest().Snapshot != nil })
	for _, focus := range []struct {
		name string
		call func() error
	}{
		{"tab", func() error { return client.FocusTab(ctx, "tab") }},
		{"pane", func() error { return client.FocusPane(ctx, "pane") }},
	} {
		if err := focus.call(); err == nil || !strings.Contains(err.Error(), "surface_inactive") {
			t.Fatalf("passive %s focus should fail surface_inactive: %v", focus.name, err)
		}
	}
	// Resize is a direct protocol message and remains permitted before the
	// activation barrier, so the server receives actual geometry first.
	if err := client.ResizeSurface(ctx, 90, 30); err != nil {
		t.Fatal(err)
	}
	if err := client.SetSurfaceActive(ctx, true); err != nil {
		t.Fatal(err)
	}
	if err := client.FocusTab(ctx, "tab"); err != nil {
		t.Fatalf("active tab focus: %v", err)
	}
	if err := client.FocusPane(ctx, "pane"); err != nil {
		t.Fatalf("active pane focus: %v", err)
	}
	if err := client.SetSurfaceActive(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := client.FocusTab(ctx, "tab"); err == nil || !strings.Contains(err.Error(), "surface_inactive") {
		t.Fatalf("deactivated focus should fail surface_inactive: %v", err)
	}
}

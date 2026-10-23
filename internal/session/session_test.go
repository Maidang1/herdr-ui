package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

const emptySnapshot = `{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[],"tabs":[],"panes":[],"layouts":[],"agents":[]}}`

type fakeServer struct {
	mu              sync.Mutex
	calls           int
	lifecycle       net.Conn
	agentSubs       []net.Conn
	batches         [][]string
	result          string
	rejectAgentOnce bool
	block           chan struct{}
	snapshots       chan int
}

func (f *fakeServer) target(t *testing.T) herdr.Target {
	dir, err := os.MkdirTemp("/tmp", "herdr-session-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "s")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		l.Close()
		f.mu.Lock()
		if f.lifecycle != nil {
			f.lifecycle.Close()
		}
		for _, c := range f.agentSubs {
			c.Close()
		}
		f.mu.Unlock()
	})
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go f.handle(c)
		}
	}()
	return herdr.Target{Socket: path}
}
func (f *fakeServer) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return
		}
		var req struct {
			ID, Method string
			Params     json.RawMessage
		}
		if json.Unmarshal(line, &req) != nil {
			return
		}
		switch req.Method {
		case "ping":
			fmt.Fprintf(c, "{\"id\":%q,\"result\":{\"type\":\"pong\"}}\n", req.ID)
		case "events.subscribe":
			var params struct {
				Subscriptions []herdr.Subscription `json:"subscriptions"`
			}
			json.Unmarshal(req.Params, &params)
			var ids []string
			for _, sub := range params.Subscriptions {
				if sub.Type == "pane.agent_status_changed" {
					ids = append(ids, sub.PaneID)
				}
			}
			f.mu.Lock()
			if len(ids) == 0 {
				f.lifecycle = c
			} else {
				f.batches = append(f.batches, ids)
				f.agentSubs = append(f.agentSubs, c)
			}
			reject := len(ids) > 0 && f.rejectAgentOnce
			if reject {
				f.rejectAgentOnce = false
			}
			f.mu.Unlock()
			if reject {
				fmt.Fprintf(c, "{\"id\":%q,\"error\":{\"code\":\"pane_not_found\",\"message\":\"closed during handshake\"}}\n", req.ID)
				return
			}
			fmt.Fprintf(c, "{\"id\":%q,\"result\":{\"type\":\"subscription_started\"}}\n", req.ID)
			r.ReadByte()
			return
		case "session.snapshot":
			f.mu.Lock()
			f.calls++
			n := f.calls
			block := f.block
			result := f.result
			if result == "" {
				result = emptySnapshot
			}
			f.mu.Unlock()
			if f.snapshots != nil {
				select {
				case f.snapshots <- n:
				default:
				}
			}
			if n == 3 && block != nil {
				<-block
			}
			fmt.Fprintf(c, "{\"id\":%q,\"result\":%s}\n", req.ID, result)
		}
	}
}
func (f *fakeServer) event() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lifecycle != nil {
		fmt.Fprintln(f.lifecycle, `{"event":"workspace_created","data":{}}`)
	}
}

func TestBootstrapDoesNotPollAndCloseIsIdempotent(t *testing.T) {
	f := &fakeServer{}
	s := New(f.target(t))
	s.Start(context.Background(), func() {})
	await(t, func() bool { return s.Latest().State == Ready })
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	f.mu.Lock()
	after := f.calls
	f.mu.Unlock()
	if calls != 2 || after != calls {
		t.Fatalf("bootstrap=%d after idle=%d", calls, after)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Latest().State != Closed {
		t.Fatal(s.Latest())
	}
}

func TestEventDuringSnapshotIsNotLost(t *testing.T) {
	f := &fakeServer{block: make(chan struct{}), snapshots: make(chan int, 8)}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	f.event()
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.calls == 3 })
	f.event()
	close(f.block)
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.calls >= 4 })
}

func TestSubscriptionEOFReconnectsWithoutClearingSnapshot(t *testing.T) {
	f := &fakeServer{}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	first := s.Latest()
	f.mu.Lock()
	f.lifecycle.Close()
	f.mu.Unlock()
	await(t, func() bool { return s.Latest().State == Reconnecting })
	if p := s.Latest(); p.Snapshot == nil || !p.Stale {
		t.Fatal("disconnect discarded last snapshot")
	}
	await(t, func() bool { return s.Latest().State == Ready && s.Latest().Epoch > first.Epoch })
}

const populatedSnapshot = `{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","agent_status":"working"}],"layouts":[],"agents":[]}}`

func TestAgentSubscriptionsReplaceWithoutSpuriousReconnect(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	epoch := s.Latest().Epoch
	f.mu.Lock()
	if len(f.batches) != 1 || len(f.batches[0]) != 1 || f.batches[0][0] != "p" {
		t.Fatal("agent subscription missing pane ID")
	}
	f.result = emptySnapshot
	f.mu.Unlock()
	f.event()
	await(t, func() bool { p := s.Latest(); return p.State == Ready && len(p.Snapshot.PaneIDs()) == 0 })
	time.Sleep(30 * time.Millisecond)
	if s.Latest().Epoch != epoch {
		t.Fatal("expected close of retired agent subscription caused reconnect")
	}
}

func TestPaneDisappearingDuringSubscribeRefreshesRatherThanIncompatible(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, rejectAgentOnce: true}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.batches) != 2 {
		t.Fatalf("batches=%v", f.batches)
	}
	if s.Latest().Epoch != 1 {
		t.Fatal("transient pane race changed epoch")
	}
}

func TestMalformedSnapshotStopsUntilExplicitRetry(t *testing.T) {
	f := &fakeServer{result: `{"type":"session_snapshot","snapshot":{"version":"test"}}`}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Incompatible })
	f.mu.Lock()
	calls := f.calls
	f.mu.Unlock()
	time.Sleep(300 * time.Millisecond)
	f.mu.Lock()
	after := f.calls
	f.result = emptySnapshot
	f.mu.Unlock()
	if after != calls {
		t.Fatal("malformed protocol retried indefinitely")
	}
	s.Refresh()
	await(t, func() bool { return s.Latest().State == Ready })
}

func await(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out")
}

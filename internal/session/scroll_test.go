package session

import (
	"context"
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSnapshotScrollMetrics(t *testing.T) {
	s, err := DecodeSnapshot(modelFixture(t, func(raw map[string]any) {
		raw["panes"].([]any)[0].(map[string]any)["scroll"] = map[string]any{"offset_from_bottom": 3, "max_offset_from_bottom": 100, "viewport_rows": 24}
	}))
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Pane("p")
	if !p.Scroll.Known || p.Scroll.Offset != 3 || p.Scroll.MaxOffset != 100 || p.Scroll.Rows != 24 {
		t.Fatal(p.Scroll)
	}
}

func TestScrollQueueSupersedesPendingAndRejectsStale(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, scrollBlock: make(chan struct{})}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	// A wrong terminal mapping is rejected before sending any operation.
	bad := s.Execute(context.Background(), Operation{Kind: Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: "p", TerminalID: "old", Offset: 1})
	if got := <-bad; got.Status != StaleTarget {
		t.Fatal(got)
	}
	operation := Operation{Kind: Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: "p", TerminalID: "term", Offset: 1}
	first := s.Execute(context.Background(), operation)
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.scrollCalls) == 1 })
	operation.Offset = 2
	second := s.Execute(context.Background(), operation)
	operation.Offset = 3
	third := s.Execute(context.Background(), operation)
	if got := <-second; got.Status != Superseded {
		t.Fatal(got)
	}
	close(f.scrollBlock)
	if got := <-first; got.Status != Succeeded {
		t.Fatal(got)
	}
	if got := <-third; got.Status != Succeeded {
		t.Fatal(got)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.scrollCalls) != 2 || f.scrollCalls[0] != 1 || f.scrollCalls[1] != 3 {
		t.Fatal(f.scrollCalls)
	}
}

func TestVisibleScrollSubscriptionUpdatesWithoutPolling(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	s.ObserveScroll("p", "term")
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.scrollSubs) == 1 })
	await(t, func() bool { return s.Latest().State == Ready })
	revision := s.Latest().Revision
	f.mu.Lock()
	fmt.Fprintln(f.scrollSubs[0], `{"event":"pane.scroll_changed","data":{"pane_id":"p"}}`)
	f.mu.Unlock()
	await(t, func() bool { return s.Latest().Revision > revision && s.Latest().State == Ready })
	epoch := s.Latest().Epoch
	s.ObserveScroll("", "")
	await(t, func() bool { return s.Latest().Revision > revision+1 && s.Latest().State == Ready })
	if s.Latest().Epoch != epoch {
		t.Fatal("closing scroll subscription caused reconnect")
	}
}

func TestScrollBeforeStartAndAfterClose(t *testing.T) {
	s := New(herdr.Target{})
	select {
	case result := <-s.Execute(context.Background(), Operation{Kind: Scroll}):
		if result.Status != StaleTarget {
			t.Fatal(result)
		}
	case <-time.After(time.Second):
		t.Fatal("Execute blocked")
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := <-s.Execute(context.Background(), Operation{Kind: Scroll}); got.Status != OperationClosed {
		t.Fatal(got)
	}
}

func TestParentCancellationCompletesConcurrentOperations(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot}
	s := New(f.target(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx, nil)
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	channels := make([]<-chan OperationResult, 128)
	gate := make(chan struct{})
	var calls sync.WaitGroup
	for i := range channels {
		calls.Go(func() {
			<-gate
			channels[i] = s.Execute(context.Background(), Operation{Kind: Scroll, TargetID: p.TargetID, Epoch: p.Epoch, PaneID: "p", TerminalID: "term", Offset: uint64(i % 100)})
		})
	}
	close(gate)
	cancel()
	calls.Wait()
	for _, ch := range channels {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("operation admitted after final drain was orphaned")
		}
	}
	if err := s.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestVisibleScrollSetSubscribesEveryValidPaneOnce(t *testing.T) {
	data := strings.Replace(populatedSnapshot, `"panes":[`, `"panes":[{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"t"},`, 1)
	f := &fakeServer{result: data}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	targets := []ScrollTarget{{"p2", "term2"}, {"p", "term"}, {"p", "term"}, {"missing", "no"}}
	s.ObserveScrolls(targets)
	targets[0].PaneID = "mutated"
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.scrollBatches) == 1 })
	f.mu.Lock()
	got := append([]string(nil), f.scrollBatches[0]...)
	f.mu.Unlock()
	if strings.Join(got, ",") != "p,p2" {
		t.Fatal("visible subscription batch", got)
	}
	await(t, func() bool { return s.Latest().State == Ready })
	epoch := s.Latest().Epoch
	s.ObserveScrolls([]ScrollTarget{{"p2", "term2"}})
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.scrollBatches) == 2 })
	await(t, func() bool { return s.Latest().State == Ready })
	if s.Latest().Epoch != epoch {
		t.Fatal("visible set replacement reconnected session")
	}
}

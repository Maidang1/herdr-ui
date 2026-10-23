package session

import (
	"context"
	"strings"
	"testing"
)

func TestLayoutOperationsValidateScopeAndDuplicateSplit(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, layoutBlock: make(chan struct{})}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	op := Operation{Kind: Split, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term", Direction: "right"}
	first := s.Execute(context.Background(), op)
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.layoutCalls > 0 })
	if result := <-s.Execute(context.Background(), op); result.Status != Busy {
		t.Fatal("duplicate split accepted", result)
	}
	close(f.layoutBlock)
	if result := <-first; result.Status != Succeeded || result.PaneID != "new" {
		t.Fatal(result)
	}
	op.TabID = "wrong"
	if result := <-s.Execute(context.Background(), op); result.Status != StaleTarget {
		t.Fatal(result)
	}
}

func TestRatioValidatesFreshExportFingerprint(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	op := Operation{Kind: SetRatio, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term", Fingerprint: "old tree", Ratio: .6}
	if result := <-s.Execute(context.Background(), op); result.Status != StaleTarget {
		t.Fatal(result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ratioCalls != 0 {
		t.Fatal("stale ratio mutation sent")
	}
}

// A slow ordinary request leaves only the latest ratio in the pending slot.
// The paths owned by queued operations must not alias callers' mutable slices.
func TestRatioQueueCoalescesAndOwnsPath(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, layoutBlock: make(chan struct{})}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	op := Operation{Kind: Split, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term", Direction: "right"}
	split := s.Execute(context.Background(), op)
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return f.layoutCalls == 1 })
	op.Kind = SetRatio
	op.Fingerprint = "tree"
	op.Ratio = .6
	op.Path = []bool{false}
	old := s.Execute(context.Background(), op)
	op.Ratio = .7
	newest := s.Execute(context.Background(), op)
	op.Path[0] = true
	if result := <-old; result.Status != Superseded {
		t.Fatal(result)
	}
	s.operationMu.Lock()
	if len(s.pendingOperations) != 1 {
		t.Fatalf("pending=%d", len(s.pendingOperations))
	}
	for _, q := range s.pendingOperations {
		if q.operation.Path[0] || q.operation.Ratio != .7 {
			t.Fatalf("pending operation mutated: %+v", q.operation)
		}
	}
	s.operationMu.Unlock()
	close(f.layoutBlock)
	<-split
	if result := <-newest; result.Status != StaleTarget {
		t.Fatal(result)
	}
}

func TestRatioExportFailureIsDefinitelyNotSent(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, exportDrop: true}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: SetRatio, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term", Fingerprint: "tree", Ratio: .6})
	if result.Status != NotSent {
		t.Fatal(result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ratioCalls != 0 {
		t.Fatal("ratio mutation sent after failed export")
	}
}

func TestReadOnlyLayoutExportDoesNotGenerateSnapshotLoop(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: ExportLayout, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != Succeeded {
		t.Fatal(result)
	}
	// Serial read-only requests are barriers; none should force a fresh snapshot.
	for i := 0; i < 3; i++ {
		result = <-s.Execute(context.Background(), result.Operation)
		if result.Status != Succeeded {
			t.Fatal(result)
		}
	}
	if s.Latest().Revision != p.Revision {
		t.Fatal("read-only exports generated new snapshots")
	}
}

func TestSplitLostResponseIsNeverReplayedAfterReconnect(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, dropSplit: true}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: Split, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term", Direction: "right"})
	if result.Status != OutcomeUnknown {
		t.Fatal(result)
	}
	await(t, func() bool { latest := s.Latest(); return latest.State == Ready && latest.Epoch > p.Epoch })
	latest := s.Latest()
	result = <-s.Execute(context.Background(), Operation{Kind: ExportLayout, TargetID: latest.TargetID, Epoch: latest.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != Succeeded {
		t.Fatal(result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.layoutCalls != 1 {
		t.Fatalf("split replayed %d times", f.layoutCalls)
	}
}

func TestClosePaneUsesExplicitTargetAndRejectsDuplicate(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, closeBlock: make(chan struct{})}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	op := Operation{Kind: ClosePane, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"}
	first := s.Execute(context.Background(), op)
	await(t, func() bool { f.mu.Lock(); defer f.mu.Unlock(); return len(f.closeCalls) == 1 })
	duplicate := <-s.Execute(context.Background(), op)
	close(f.closeBlock)
	if duplicate.Status != Busy {
		t.Fatal("duplicate close accepted", duplicate)
	}
	if result := <-first; result.Status != Succeeded {
		t.Fatal(result)
	}
	op.TerminalID = "replacement"
	if result := <-s.Execute(context.Background(), op); result.Status != StaleTarget {
		t.Fatal("mismatched terminal accepted", result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.closeCalls) != 1 || f.closeCalls[0] != "p" || f.closeExtra {
		t.Fatal("close was not explicit pane-only", f.closeCalls, f.closeExtra)
	}
}

func TestClosePaneConfirmationRequiredIsNotOverridden(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, closeReject: true}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: ClosePane, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != Rejected || !strings.Contains(result.Error, "confirmation_required") {
		t.Fatal(result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.closeCalls) != 1 {
		t.Fatal("close was repeated", f.closeCalls)
	}
}

func TestClosePaneLostResponseIsNotReplayedAfterReconnect(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, closeDrop: true}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: ClosePane, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != OutcomeUnknown {
		t.Fatal(result)
	}
	await(t, func() bool { latest := s.Latest(); return latest.State == Ready && latest.Epoch > p.Epoch })
	latest := s.Latest()
	result = <-s.Execute(context.Background(), Operation{Kind: ExportLayout, TargetID: latest.TargetID, Epoch: latest.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != Succeeded {
		t.Fatal(result)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.closeCalls) != 1 {
		t.Fatal("close replayed", f.closeCalls)
	}
}

func TestClosePaneRequiresSuccessfulResponseType(t *testing.T) {
	f := &fakeServer{result: populatedSnapshot, closeResultType: "pane_info"}
	s := New(f.target(t))
	s.Start(context.Background(), nil)
	defer s.Close(context.Background())
	await(t, func() bool { return s.Latest().State == Ready })
	p := s.Latest()
	result := <-s.Execute(context.Background(), Operation{Kind: ClosePane, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"})
	if result.Status != OutcomeUnknown {
		t.Fatal("malformed close acknowledgement accepted", result)
	}
}

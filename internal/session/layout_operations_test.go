package session

import (
	"context"
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

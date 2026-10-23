package session

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/layout"
)

type OperationKind uint8

const (
	Scroll OperationKind = iota + 1
	Neighbor
	Split
	Zoom
	ExportLayout
	SetRatio
	ClosePane
)

type ResultStatus string

const (
	Succeeded       ResultStatus = "Succeeded"
	Rejected        ResultStatus = "Rejected"
	OutcomeUnknown  ResultStatus = "OutcomeUnknown"
	StaleTarget     ResultStatus = "StaleTarget"
	Busy            ResultStatus = "Busy"
	Superseded      ResultStatus = "Superseded"
	OperationClosed ResultStatus = "Closed"
	NotSent         ResultStatus = "NotSent"
)

type Operation struct {
	Kind                                       OperationKind
	TargetID, Epoch                            uint64
	PaneID, TerminalID                         string
	Offset                                     uint64
	WorkspaceID, TabID, Direction, Fingerprint string
	Path                                       []bool
	Ratio                                      float64
}
type OperationResult struct {
	Status      ResultStatus
	Error       string
	OperationID uint64
	Operation   Operation
	Scroll      ScrollInfo
	PaneID      string
	Root        *layout.Node
	Fingerprint string
}
type queuedOperation struct {
	operation   Operation
	id          uint64
	ctx         context.Context
	result      chan OperationResult
	reservation string
}
type operationKey struct {
	epoch                  uint64
	pane, terminal         string
	kind                   OperationKind
	tab, fingerprint, path string
	nonce                  uint64
}

// Execute accepts only closed operation types. Scroll slots hold one latest
// pending request per identity; the coordinator owns the one in-flight request.
func (s *Session) Execute(ctx context.Context, op Operation) <-chan OperationResult {
	op.Path = append([]bool{}, op.Path...)
	q := &queuedOperation{operation: op, ctx: ctx, result: make(chan OperationResult, 1)}
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	s.nextOperation++
	q.id = s.nextOperation
	p := s.Latest()
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed || p.State == Closed {
		q.complete(OperationClosed, "session closed", ScrollInfo{})
		return q.result
	}
	if ctx.Err() != nil {
		q.complete(NotSent, ctx.Err().Error(), ScrollInfo{})
		return q.result
	}
	if op.Kind < Scroll || op.Kind > ClosePane {
		q.complete(Rejected, "unsupported operation", ScrollInfo{})
		return q.result
	}
	if !validOperation(p, op) {
		q.complete(StaleTarget, "scroll target is not authoritative", ScrollInfo{})
		return q.result
	}
	if (op.Kind == Split && (op.Direction != "right" && op.Direction != "down")) || (op.Kind == Neighbor && (op.Direction != "left" && op.Direction != "right" && op.Direction != "up" && op.Direction != "down")) || (op.Kind == SetRatio && (op.Fingerprint == "" || math.IsNaN(op.Ratio) || math.IsInf(op.Ratio, 0) || op.Ratio < .1 || op.Ratio > .9)) {
		q.complete(Rejected, "invalid operation parameters", ScrollInfo{})
		return q.result
	}
	key := operationKey{epoch: op.Epoch, pane: op.PaneID, terminal: op.TerminalID, kind: op.Kind}
	if op.Kind == SetRatio {
		key.pane, key.terminal = "", ""
		key.tab, key.fingerprint, key.path = op.TabID, op.Fingerprint, pathKey(op.Path)
	} else if op.Kind != Scroll {
		key.nonce = q.id
	}
	if op.Kind == Split || op.Kind == Zoom || op.Kind == ClosePane {
		if s.reservedOperations == nil {
			s.reservedOperations = make(map[string]bool)
		}
		q.reservation = fmt.Sprintf("%d:%d:%s:%s:%s", op.Kind, op.Epoch, op.WorkspaceID, op.TabID, op.PaneID)
		if s.reservedOperations[q.reservation] {
			q.complete(Busy, "operation already pending for this pane", ScrollInfo{})
			return q.result
		}
	}
	if old := s.pendingOperations[key]; old != nil {
		old.complete(Superseded, "replaced before sending", ScrollInfo{})
	} else {
		if len(s.pendingOperations) >= 32 {
			q.complete(Busy, "operation queue is full", ScrollInfo{})
			return q.result
		}
		s.operationOrder = append(s.operationOrder, key)
	}
	s.pendingOperations[key] = q
	if q.reservation != "" {
		s.reservedOperations[q.reservation] = true
	}
	signal(s.operationWake)
	return q.result
}

func validOperation(p PublishedSnapshot, op Operation) bool {
	if p.State != Ready || p.Stale || p.Snapshot == nil || p.TargetID != op.TargetID || p.Epoch != op.Epoch {
		return false
	}
	pane, ok := p.Snapshot.Pane(op.PaneID)
	if !ok || pane.TerminalID != op.TerminalID {
		return false
	}
	return op.Kind == Scroll || (op.WorkspaceID != "" && op.TabID != "" && pane.WorkspaceID == op.WorkspaceID && pane.TabID == op.TabID)
}
func (q *queuedOperation) complete(status ResultStatus, message string, scroll ScrollInfo) {
	q.finish(OperationResult{Status: status, Error: message, Scroll: scroll})
}
func (q *queuedOperation) finish(result OperationResult) {
	result.OperationID = q.id
	result.Operation = q.operation
	q.result <- result
	close(q.result)
}
func (s *Session) takeOperation() *queuedOperation {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	if len(s.operationOrder) == 0 {
		return nil
	}
	key := s.operationOrder[0]
	s.operationOrder = s.operationOrder[1:]
	q := s.pendingOperations[key]
	delete(s.pendingOperations, key)
	return q
}
func (s *Session) finishPending(status ResultStatus, message string) {
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	for _, q := range s.pendingOperations {
		q.complete(status, message, ScrollInfo{})
	}
	clear(s.pendingOperations)
	clear(s.reservedOperations)
	s.operationOrder = nil
}

func (s *Session) execute(ctx context.Context, client *herdr.Client, q *queuedOperation) error {
	defer func() {
		if q.reservation != "" {
			s.operationMu.Lock()
			delete(s.reservedOperations, q.reservation)
			s.operationMu.Unlock()
		}
	}()
	if q.ctx.Err() != nil {
		q.complete(NotSent, q.ctx.Err().Error(), ScrollInfo{})
		return nil
	}
	if !validOperation(s.Latest(), q.operation) {
		q.complete(StaleTarget, "scroll target changed before send", ScrollInfo{})
		return nil
	}
	call, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(q.ctx, cancel)
	defer stop()
	if q.operation.Kind != Scroll {
		return s.executeLayout(call, client, q)
	}
	var response struct {
		Type string `json:"type"`
		Pane struct {
			ID       string `json:"pane_id"`
			Terminal string `json:"terminal_id"`
			Scroll   *struct {
				Offset uint64 `json:"offset_from_bottom"`
				Max    uint64 `json:"max_offset_from_bottom"`
				Rows   uint64 `json:"viewport_rows"`
			} `json:"scroll"`
		} `json:"pane"`
	}
	err := client.Call(call, "pane.scroll", map[string]any{"pane_id": q.operation.PaneID, "offset_from_bottom": q.operation.Offset}, &response)
	if err != nil {
		var remote *herdr.RemoteError
		if errors.As(err, &remote) {
			q.complete(Rejected, err.Error(), ScrollInfo{})
			return nil
		}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	if response.Pane.ID != q.operation.PaneID || response.Pane.Terminal != q.operation.TerminalID || response.Pane.Scroll == nil {
		err := &herdr.ProtocolError{Reason: "pane.scroll response has a different or missing target"}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	v := response.Pane.Scroll
	if v.Offset > v.Max {
		err := &herdr.ProtocolError{Reason: "invalid scroll response metrics"}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	q.complete(Succeeded, "", ScrollInfo{v.Offset, v.Max, v.Rows, true})
	return nil
}

func pathKey(path []bool) string {
	var b strings.Builder
	for _, v := range path {
		if v {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

// ScrollTarget identifies a visible pane's server history subscription.
type ScrollTarget struct{ PaneID, TerminalID string }

// ObserveScroll preserves the single-pane caller contract.
func (s *Session) ObserveScroll(paneID, terminalID string) {
	if paneID == "" {
		s.ObserveScrolls(nil)
		return
	}
	s.ObserveScrolls([]ScrollTarget{{paneID, terminalID}})
}

// ObserveScrolls replaces the visible set, independent of keyboard focus.
func (s *Session) ObserveScrolls(targets []ScrollTarget) {
	targets = append([]ScrollTarget(nil), targets...)
	slices.SortFunc(targets, func(a, b ScrollTarget) int { return strings.Compare(a.PaneID, b.PaneID) })
	targets = slices.Compact(targets)
	s.visibleMu.Lock()
	changed := !slices.Equal(s.visiblePanes, targets)
	s.visiblePanes = targets
	s.visibleMu.Unlock()
	if changed {
		s.Refresh()
	}
}
func (s *Session) visibleScrolls() []ScrollTarget {
	s.visibleMu.Lock()
	defer s.visibleMu.Unlock()
	return append([]ScrollTarget(nil), s.visiblePanes...)
}

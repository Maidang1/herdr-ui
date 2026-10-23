package session

import (
	"context"
	"errors"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

type OperationKind uint8

const Scroll OperationKind = 1

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
	Kind               OperationKind
	TargetID, Epoch    uint64
	PaneID, TerminalID string
	Offset             uint64
}
type OperationResult struct {
	Status      ResultStatus
	Error       string
	OperationID uint64
	Operation   Operation
	Scroll      ScrollInfo
}
type queuedOperation struct {
	operation Operation
	id        uint64
	ctx       context.Context
	result    chan OperationResult
}
type operationKey struct {
	epoch          uint64
	pane, terminal string
}

// Execute accepts only closed operation types. Scroll slots hold one latest
// pending request per identity; the coordinator owns the one in-flight request.
func (s *Session) Execute(ctx context.Context, op Operation) <-chan OperationResult {
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
	if op.Kind != Scroll {
		q.complete(Rejected, "unsupported operation", ScrollInfo{})
		return q.result
	}
	if !validOperation(p, op) {
		q.complete(StaleTarget, "scroll target is not authoritative", ScrollInfo{})
		return q.result
	}
	key := operationKey{op.Epoch, op.PaneID, op.TerminalID}
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
	signal(s.operationWake)
	return q.result
}

func validOperation(p PublishedSnapshot, op Operation) bool {
	if p.State != Ready || p.Stale || p.Snapshot == nil || p.TargetID != op.TargetID || p.Epoch != op.Epoch {
		return false
	}
	pane, ok := p.Snapshot.Pane(op.PaneID)
	return ok && pane.TerminalID == op.TerminalID
}
func (q *queuedOperation) complete(status ResultStatus, message string, scroll ScrollInfo) {
	q.result <- OperationResult{status, message, q.id, q.operation, scroll}
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
	s.operationOrder = nil
}

func (s *Session) execute(ctx context.Context, client *herdr.Client, q *queuedOperation) error {
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

// ObserveScroll scopes the third subscription to the pane this window shows.
// Its pane/terminal pair is validated against each authoritative snapshot.
func (s *Session) ObserveScroll(paneID, terminalID string) {
	s.visibleMu.Lock()
	changed := s.visiblePane != paneID || s.visibleTerminal != terminalID
	s.visiblePane, s.visibleTerminal = paneID, terminalID
	s.visibleMu.Unlock()
	if changed {
		s.Refresh()
	}
}
func (s *Session) visibleScroll() (string, string) {
	s.visibleMu.Lock()
	defer s.visibleMu.Unlock()
	return s.visiblePane, s.visibleTerminal
}

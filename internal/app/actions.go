package app

import (
	"fmt"
	"github.com/bytedance/herdr-ui/internal/session"
)

type Intent struct{ ID uint64 }
type pendingIntent struct {
	operation                       session.Operation
	viewRevision, completedRevision uint64
	complete                        bool
	result                          session.OperationResult
}

func (a *App) ViewRevision() uint64 { return a.viewRevision }
func (a *App) Pending(kind session.OperationKind, paneID string) bool {
	for _, p := range a.intents {
		if p.operation.Kind == kind && p.operation.PaneID == paneID {
			return true
		}
	}
	return false
}
func (a *App) BeginOperation(op session.Operation) (Intent, bool) {
	if a.publication.State != session.Ready || a.publication.Stale || a.publication.TargetID != op.TargetID || a.publication.Epoch != op.Epoch || a.snapshot == nil {
		return Intent{}, false
	}
	pane, ok := a.snapshot.Pane(op.PaneID)
	if !ok || pane.TerminalID != op.TerminalID || pane.WorkspaceID != op.WorkspaceID || pane.TabID != op.TabID || a.Pending(op.Kind, op.PaneID) {
		return Intent{}, false
	}
	if a.intents == nil {
		a.intents = map[uint64]*pendingIntent{}
	}
	a.nextIntent++
	a.intents[a.nextIntent] = &pendingIntent{operation: op, viewRevision: a.viewRevision}
	a.actionMessage = ""
	a.version++
	return Intent{a.nextIntent}, true
}
func (a *App) CompleteOperation(intent Intent, result session.OperationResult) {
	p := a.intents[intent.ID]
	if p == nil {
		return
	}
	p.complete = true
	p.result = result
	p.completedRevision = a.publication.Revision
	a.version++
	switch result.Status {
	case session.Succeeded:
		if result.PaneID == "" && p.operation.Kind != session.ClosePane {
			delete(a.intents, intent.ID)
		}
	case session.OutcomeUnknown:
		a.actionMessage = "The previous action may have completed. Check the refreshed layout before repeating it. The action was not retried."
	default:
		a.actionMessage = fmt.Sprintf("%s: %s", result.Status, result.Error)
		delete(a.intents, intent.ID)
	}
	a.applyPending()
}
func (a *App) applyPending() {
	for id, p := range a.intents {
		if p.operation.TargetID != a.publication.TargetID || p.operation.Epoch != a.publication.Epoch {
			delete(a.intents, id)
			continue
		}
		if !p.complete {
			continue
		}
		if p.result.Status == session.OutcomeUnknown {
			if a.publication.Revision > p.completedRevision {
				delete(a.intents, id)
			}
			continue
		}
		if p.result.Status != session.Succeeded {
			continue
		}
		if p.operation.Kind == session.ClosePane {
			// Acknowledgement precedes the authoritative topology update. Keep
			// duplicate closes blocked until the captured terminal is gone.
			if a.snapshot != nil {
				pane, ok := a.snapshot.Pane(p.operation.PaneID)
				if !ok || pane.TerminalID != p.operation.TerminalID {
					delete(a.intents, id)
				}
			}
			continue
		}
		if p.viewRevision != a.viewRevision || a.workspaceID != p.operation.WorkspaceID || a.tabs[a.workspaceID] != p.operation.TabID {
			delete(a.intents, id)
			continue
		}
		if a.snapshot != nil {
			pane, ok := a.snapshot.Pane(p.result.PaneID)
			if ok {
				delete(a.intents, id)
				if pane.WorkspaceID == p.operation.WorkspaceID && pane.TabID == p.operation.TabID {
					a.SelectPane(pane.ID)
				} else {
					a.actionMessage = "The returned pane is outside the selected tab."
				}
			} else if a.publication.Revision > p.completedRevision {
				delete(a.intents, id)
				a.actionMessage = "The returned pane is no longer available."
			}
		}
	}
}
func (a *App) CycleTab(delta int) {
	if a.snapshot == nil {
		return
	}
	tabs := a.snapshot.Tabs(a.workspaceID)
	if len(tabs) < 2 {
		return
	}
	current := 0
	for i, tab := range tabs {
		if tab.ID == a.tabs[a.workspaceID] {
			current = i
			break
		}
	}
	next := ((current+delta)%len(tabs) + len(tabs)) % len(tabs)
	a.SelectTab(tabs[next].ID)
}

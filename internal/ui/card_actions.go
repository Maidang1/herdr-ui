package ui

import (
	"fmt"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func cardOperation(action string, pane app.PaneRow, frame app.View, p session.PublishedSnapshot) (session.Operation, bool) {
	kind := actionKind(action)
	if kind != session.Split && kind != session.Zoom && kind != session.ClosePane {
		return session.Operation{}, false
	}
	if p.State != session.Ready || p.Stale || p.Snapshot == nil || p.TargetID != frame.TargetID || p.Epoch != frame.Epoch {
		return session.Operation{}, false
	}
	actual, ok := p.Snapshot.Pane(pane.ID)
	if !ok || actual.TerminalID != pane.TerminalID || actual.WorkspaceID != frame.WorkspaceID || actual.TabID != frame.TabID {
		return session.Operation{}, false
	}
	direction := ""
	if action == "split_vertical" {
		direction = "right"
	} else if action == "split_horizontal" {
		direction = "down"
	}
	return session.Operation{Kind: kind, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: actual.WorkspaceID, TabID: actual.TabID, PaneID: actual.ID, TerminalID: actual.TerminalID, Direction: direction}, true
}
func (w *Window) cardActionOperation(action string, pane app.PaneRow, frame app.View) (session.Operation, bool) {
	if w.terminal == nil || w.terminal.session == nil || w.controller == nil {
		return session.Operation{}, false
	}
	current := w.currentView()
	if current.TargetID != frame.TargetID || current.Epoch != frame.Epoch || current.WorkspaceID != frame.WorkspaceID || current.TabID != frame.TabID {
		return session.Operation{}, false
	}
	op, ok := cardOperation(action, pane, frame, w.terminal.session.Latest())
	return op, ok && !w.app.Pending(op.Kind, pane.ID) && !w.app.Pending(session.ClosePane, pane.ID)
}
func (w *Window) dispatchCardAction(action string, pane app.PaneRow, frame app.View) {
	op, ok := w.cardActionOperation(action, pane, frame)
	if !ok {
		return
	}
	intent, ok := w.app.BeginOperation(op)
	if !ok {
		return
	}
	w.BlurInput()
	w.execute(op, func(r session.OperationResult) { w.app.CompleteOperation(intent, r); w.terminal.session.Refresh() })
}

type cardControl struct {
	action, label string
	icon          *native.SVG
}

// A new terminal or epoch must not inherit a press or menu selection from the old card.
func cardControlKey(action string, pane app.PaneRow, frame app.View) string {
	return fmt.Sprintf("card-action:%d:%d:%s:%s:%s", frame.TargetID, frame.Epoch, pane.ID, pane.TerminalID, action)
}

func cardControlItems(zoomed bool) []cardControl {
	zoom, icon := "Zoom pane", icons.Maximize2
	if zoomed {
		zoom, icon = "Restore pane", icons.Minimize2
	}
	return []cardControl{{"split_vertical", "Split right", icons.Columns2}, {"split_horizontal", "Split down", icons.Rows2}, {"zoom", zoom, icon}, {"close_pane", "Close pane", icons.X}}
}
func (w *Window) cardControls(c *native.Context, pane app.PaneRow, frame app.View, width float64, p design.Palette) {
	items := cardControlItems(frame.Layout.Zoomed)
	if width < 180 {
		more := native.ButtonBase(c.Key(cardControlKey("menu", pane, frame))).Grow(1).MinWidth(0).Height(24).Center().Radius(12).Label("Pane actions").KeepFocus()
		w.controlState(more, p)
		more.Children(func() { native.Icon(c, icons.Ellipsis).Size(14, 14).TextColor(p.Secondary) })
		more.Menu(func(m *native.Menu) {
			for _, item := range items {
				_, enabled := w.cardActionOperation(item.action, pane, frame)
				if m.Item(item.label).Disabled(!enabled).Chosen() {
					w.dispatchCardAction(item.action, pane, frame)
				}
			}
		})
		return
	}
	for _, item := range items {
		_, enabled := w.cardActionOperation(item.action, pane, frame)
		button := native.ButtonBase(c.Key(cardControlKey(item.action, pane, frame))).Size(24, 24).Shrink(0).Center().Radius(12).Label(item.label).KeepFocus().Disabled(!enabled)
		w.controlState(button, p)
		color := p.Secondary
		if !enabled {
			color = p.Disabled
		}
		button.Children(func() { native.Icon(c, item.icon).Size(14, 14).TextColor(color) })
		if button.Clicked() {
			w.dispatchCardAction(item.action, pane, frame)
		}
	}
}

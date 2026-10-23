package ui

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	"github.com/bytedance/herdr-ui/internal/layout"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/bytedance/herdr-ui/internal/terminalhost"
	native "github.com/egoist/mygo/ui"
)

type terminalPanel struct {
	panes       map[string]*terminalPanel
	focused     string
	host        *terminalhost.Host
	session     *session.Session
	ctx         context.Context
	post        func(func())
	binding     terminalhost.Binding
	spec        terminalhost.Spec
	tracker     app.ScrollTracker
	notice      string
	statusKnown bool
	lastStatus  terminalhost.State
}

// ConnectTerminal is called by main during window assembly, on the UI thread.
func (w *Window) ConnectTerminal(host *terminalhost.Host, s *session.Session, ctx context.Context, post func(func())) {
	w.terminal = &terminalPanel{host: host, session: s, ctx: ctx, post: post, panes: map[string]*terminalPanel{}}
}

func (w *Window) DetachTerminal() {
	if t := w.terminal; t != nil {
		t.host.Reconcile(nil)
		t.session.ObserveScroll("", "")
		t.panes = map[string]*terminalPanel{}
		t.focused = ""
		t.binding = terminalhost.Binding{}
		t.spec = terminalhost.Spec{}
		t.tracker = app.ScrollTracker{}
		t.notice = ""
		t.statusKnown = false
	}
}

func (t *terminalPanel) install(binding terminalhost.Binding, spec terminalhost.Spec) {
	if t.binding != binding {
		t.tracker = app.ScrollTracker{}
		t.notice = ""
		t.statusKnown = false
	}
	t.binding, t.spec = binding, spec
}

// Scroll runs only from the plugin's UI-thread hook. Mouse/alternate-scroll
// programs bypass that hook and retain the plugin's native terminal encoding.
func (w *Window) Scroll(binding terminalhost.Binding, rows int) {
	t := w.paneForBinding(binding)
	if t == nil {
		return
	}
	published := t.session.Latest()
	if published.Snapshot == nil {
		return
	}
	pane, ok := published.Snapshot.Pane(t.spec.PaneID)
	if !ok || pane.TerminalID != binding.TerminalID {
		return
	}
	op, ticket, ok := t.tracker.Wheel(published, pane, rows)
	if !ok {
		if published.Stale {
			t.notice = "Scrolling is temporarily unavailable while reconnecting."
		}
		return
	}
	result := t.session.Execute(t.ctx, op)
	go func() {
		completion := <-result
		if completion.Status == session.Superseded {
			return
		}
		t.post(func() {
			if t.binding != binding {
				return
			}
			latest := t.session.Latest()
			if latest.Snapshot == nil {
				return
			}
			pane, ok := latest.Snapshot.Pane(t.spec.PaneID)
			if !ok {
				return
			}
			t.tracker.Complete(ticket, completion, latest, pane)
			if completion.Status != session.Succeeded && completion.Status != session.Superseded {
				t.notice = completion.Error
			} else if completion.Status == session.Succeeded {
				t.notice = ""
			}
		})
	}()
}

func (w *Window) paneForBinding(binding terminalhost.Binding) *terminalPanel {
	if w.terminal == nil {
		return nil
	}
	for _, t := range w.terminal.panes {
		if t.binding == binding {
			return t
		}
	}
	return nil
}

func (w *Window) terminalContent(c *native.Context, v app.View, p design.Palette) {
	t := w.terminal
	area := native.Box(c.Key("terminal-area")).Grow(1).FillWidth().MinHeight(0).Clip()
	area.Children(func() {
		bounds := area.Bounds()
		if bounds.W <= 0 || bounds.H <= 0 {
			c.After(time.Millisecond)
			return
		}
		if !v.HasLayout {
			w.DetachTerminal()
			native.Text(c, "Synchronizing layout…").TextColor(p.Secondary)
			return
		}
		geometry := v.Layout
		geometry.Splits = gridDividers(geometry.Splits)
		w.updateDrag(v)
		var previewDrag *ratioDrag
		if w.controller != nil && w.controller.drag != nil && w.controller.drag.ready {
			previewDrag = w.controller.drag
			geometry.Panes, geometry.Splits = preview(previewDrag.root, geometry.Area)
		}
		projection, err := layout.Project(geometry.Area, geometry.Panes, geometry.Splits, layout.Rect{W: float64(bounds.W), H: float64(bounds.H)})
		if err != nil {
			w.DetachTerminal()
			native.Text(c, "Layout unavailable: "+err.Error()).TextColor(p.Danger)
			return
		}
		multi := len(projection.Panes) > 1
		cards := make([]layout.PaneRect, len(projection.Panes))
		viewport := layout.Rect{W: float64(bounds.W), H: float64(bounds.H)}
		for i, pane := range projection.Panes {
			cards[i] = pane
			if multi {
				cards[i].Rect = cardBounds(pane.Rect, viewport)
			}
		}
		latest := t.session.Latest()
		valid := latest.State == session.Ready && !latest.Stale && latest.TargetID == v.TargetID && latest.Snapshot != nil
		specs := []terminalhost.Spec{}
		rows := map[string]app.PaneRow{}
		for _, pane := range v.Panes {
			rows[pane.ID] = pane
		}
		for _, rect := range cards {
			pane, ok := rows[rect.PaneID]
			if !ok || !hasCardContent(rect.Rect, multi) {
				continue
			}
			if valid {
				actual, ok := latest.Snapshot.Pane(pane.ID)
				if !ok || actual.TerminalID != pane.TerminalID {
					valid = false
				}
			}
			specs = append(specs, terminalhost.Spec{TargetID: v.TargetID, PaneID: pane.ID, TerminalID: pane.TerminalID, CWD: pane.CWD})
		}
		if valid {
			bindings, err := t.host.Reconcile(specs)
			if err != nil {
				native.Text(c, err.Error()).TextColor(p.Danger)
				return
			}
			next := map[string]*terminalPanel{}
			for i, spec := range specs {
				child := t.panes[spec.PaneID]
				if child == nil {
					child = &terminalPanel{host: t.host, session: t.session, ctx: t.ctx, post: t.post}
				}
				child.install(bindings[i], spec)
				next[spec.PaneID] = child
			}
			t.panes = next
		} else {
			for id, child := range t.panes {
				row, ok := rows[id]
				if !ok || child.spec.TargetID != v.TargetID || child.spec.TerminalID != row.TerminalID {
					w.DetachTerminal()
					break
				}
			}
		}
		requestFocus := t.focused != v.PaneID
		for _, rect := range cards {
			pane, ok := rows[rect.PaneID]
			child := t.panes[rect.PaneID]
			if !ok || child == nil || !hasCardContent(rect.Rect, multi) {
				continue
			}
			r := rect.Rect
			w.paneCard(c, r, pane, pane.ID == v.PaneID, multi, p, func() {
				w.drawTerminal(c, v, p, child, pane, valid, requestFocus)
			})
		}
		if valid && !v.Layout.Zoomed {
			w.dividers(c, projection, v.Layout.Splits, previewDrag, p)
		}
		targets := make([]session.ScrollTarget, 0, len(t.panes))
		for _, spec := range specs {
			if child := t.panes[spec.PaneID]; child != nil {
				targets = append(targets, session.ScrollTarget{PaneID: child.spec.PaneID, TerminalID: child.spec.TerminalID})
			}
		}
		t.session.ObserveScrolls(targets)
	})
}
func (w *Window) drawTerminal(c *native.Context, v app.View, p design.Palette, t *terminalPanel, pane app.PaneRow, valid, requestFocus bool) {
	status := t.host.Status(t.binding)
	if !t.statusKnown || t.lastStatus != status.State {
		t.statusKnown = true
		t.lastStatus = status.State
		log.Printf("attach target=%d terminal=%s generation=%d state=%v exit=%d", t.binding.TargetID, t.binding.TerminalID, t.binding.Generation, status.State, status.ExitCode)
	}
	message := status.Message
	if t.notice != "" {
		if message != "" {
			message += " · " + t.notice
		} else {
			message = t.notice
		}
	}
	if message != "" {
		native.Row(c).Padding(8, 16).Gap(8).AlignItems(native.Center).Children(func() {
			color := p.Secondary
			if status.State == terminalhost.Failed {
				color = p.Danger
			}
			native.Icon(c, icons.CircleAlert).Size(14, 14).TextColor(color)
			native.Text(c, message).Grow(1).TextColor(color)
			if status.State == terminalhost.Failed {
				if native.Button(c, "Retry").Label("Retry terminal").Disabled(!valid).Clicked() {
					t.host.Retry(t.binding)
				}
			}
		})
	}
	switch status.State {
	case terminalhost.Starting:
		native.Column(c).Grow(1).FillWidth().Center().Children(func() { native.Text(c, "Connecting terminal…").TextColor(p.Secondary) })
	case terminalhost.Attached, terminalhost.Failed:
		// The failed terminal's final screen contains actual CLI diagnostics such
		// as owner contention; preserve it below Retry without logging its text.
		if w.controller != nil && pane.ID == v.PaneID {
			state := t.host.InputState(t.binding)
			w.controller.guarded = state.Guarded
			w.controller.guardKnown = true
			if state.Composing {
				w.controller.machine.Reset()
			}
		}
		el := t.host.View(c.Key(fmt.Sprintf("terminal:%d:%s:%d", t.binding.TargetID, t.binding.TerminalID, t.binding.Generation)), t.binding).Fill()
		if requestFocus && pane.ID == v.PaneID {
			el.Focus()
			w.terminal.focused = pane.ID
		} else if !requestFocus && el.Focused() && pane.ID != v.PaneID {
			w.app.SelectPane(pane.ID)
			w.BlurInput()
		}
		if !requestFocus && pane.ID == v.PaneID && !el.Focused() && w.controller != nil {
			w.controller.machine.Reset()
		}
	}
}

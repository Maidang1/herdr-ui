package ui

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/bytedance/herdr-ui/internal/terminalhost"
	native "github.com/egoist/mygo/ui"
)

type terminalPanel struct {
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
	w.terminal = &terminalPanel{host: host, session: s, ctx: ctx, post: post}
}

func (w *Window) DetachTerminal() {
	if t := w.terminal; t != nil {
		t.host.Reconcile(nil)
		t.session.ObserveScroll("", "")
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
	t := w.terminal
	if t == nil || t.binding != binding {
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

func (w *Window) terminalContent(c *native.Context, v app.View, p design.Palette) {
	t := w.terminal
	area := native.Column(c.Key("terminal-area")).Grow(1).FillWidth().MinHeight(0).AlignItems(native.Stretch)
	area.Children(func() {
		var pane app.PaneRow
		for _, candidate := range v.Panes {
			if candidate.ID == v.PaneID {
				pane = candidate
				break
			}
		}
		wanted := terminalhost.Spec{TargetID: v.TargetID, TerminalID: pane.TerminalID, PaneID: pane.ID, CWD: pane.CWD}
		same := t.spec.TargetID == wanted.TargetID && t.spec.TerminalID == wanted.TerminalID && t.spec.PaneID == wanted.PaneID && t.binding.Generation != 0
		bounds := area.Bounds()
		if bounds.W <= 0 || bounds.H <= 0 {
			c.After(time.Millisecond)
			return
		}
		latest := t.session.Latest()
		valid := latest.State == session.Ready && !latest.Stale && latest.TargetID == v.TargetID && pane.ID != ""
		if valid {
			current, ok := latest.Snapshot.Pane(pane.ID)
			valid = ok && current.TerminalID == pane.TerminalID
		}
		if valid {
			bindings, err := t.host.Reconcile([]terminalhost.Spec{wanted})
			if err != nil {
				t.notice = err.Error()
			} else {
				t.install(bindings[0], wanted)
				t.session.ObserveScroll(pane.ID, pane.TerminalID)
			}
		} else if !same {
			w.DetachTerminal()
		}
		if t.binding.Generation == 0 {
			title := "Synchronizing terminal…"
			if pane.ID == "" && v.State == session.Ready {
				title = "No terminal in this tab"
			}
			native.Column(c).Grow(1).FillWidth().Center().Padding(24).Children(func() { native.Text(c, title).TextColor(p.Secondary) })
			return
		}
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
			t.host.View(c.Key(fmt.Sprintf("terminal:%d:%s:%d", t.binding.TargetID, t.binding.TerminalID, t.binding.Generation)), t.binding).Fill().AutoFocus()
		}
	})
}

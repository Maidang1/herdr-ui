package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

type detailsUI struct {
	open, busy bool
	generation uint64
	op         session.Operation
	data       herdr.PaneDetails
	error      string
}
type outputWatchUI struct {
	cancel     func()
	op         session.Operation
	generation uint64
	status     string
	regex      bool
	minutes    string
}

// SyncFeatureTargets ends reminders when their captured terminal or epoch ends.
func (w *Window) SyncFeatureTargets() {
	s := &w.outputWatch
	if s.cancel != nil && !w.featureCurrent(s.op) {
		s.cancel()
		s.cancel = nil
		s.generation++
		s.status = "Output reminder ended because the terminal changed."
	}
}
func (w *Window) featureCurrent(op session.Operation) bool {
	p := w.app.Publication()
	if p.State != session.Ready || p.Stale || p.TargetID != op.TargetID || p.Epoch != op.Epoch || p.Snapshot == nil {
		return false
	}
	pane, ok := p.Snapshot.Pane(op.PaneID)
	return ok && pane.TerminalID == op.TerminalID && pane.TabID == op.TabID && pane.WorkspaceID == op.WorkspaceID
}
func (w *Window) openDetails() {
	op, ok := w.operation(session.ExportLayout, "")
	if !ok {
		return
	}
	w.CloseCommandSurface()
	w.BlurInput()
	w.details.open = true
	w.details.op = op
	w.details.data = herdr.PaneDetails{}
	w.details.busy = false
	w.details.generation++
	w.loadDetails()
}
func (w *Window) loadDetails() {
	d := &w.details
	if d.busy || w.hooks.PaneDetails == nil {
		return
	}
	d.busy = true
	d.error = ""
	d.generation++
	gen := d.generation
	op := d.op
	w.hooks.PaneDetails(op, func(data herdr.PaneDetails, err error) {
		if gen != d.generation {
			return
		}
		d.busy = false
		if !w.featureCurrent(op) {
			d.error = "This terminal is no longer available. Close details and select a terminal again."
			return
		}
		if err != nil {
			d.error = err.Error()
			return
		}
		if data.Agent.TerminalID != "" && data.Agent.TerminalID != op.TerminalID {
			d.error = "The terminal was replaced. Reopen details."
			return
		}
		d.data = data
	})
}
func (w *Window) detailsView(c *native.Context, p design.Palette) {
	d := &w.details
	native.Scroll(c.Key("pane-details-scroll")).Grow(1).FillWidth().Children(func() {
		native.Column(c.Key("pane-details")).FillWidth().MaxWidth(760).Padding(24).Gap(20).Children(func() {
			native.Row(c).FillWidth().Gap(12).AlignItems(native.Center).Children(func() {
				native.Text(c, "Agent and process details").FontSize(20).Bold().Grow(1)
				if native.Button(c, "Refresh").Disabled(d.busy || !w.featureCurrent(d.op)).Clicked() {
					w.loadDetails()
				}
				if native.Button(c, "Close").Clicked() {
					d.open = false
					d.generation++
					d.busy = false
				}
			})
			if d.busy {
				native.Text(c, "Reading details…").TextColor(p.Secondary)
			}
			if d.error != "" {
				native.Text(c, d.error).TextColor(p.Danger)
			}
			a := d.data.Agent
			if d.data.AgentError != "" {
				native.Text(c, "Agent details: "+d.data.AgentError).TextColor(p.Secondary)
			}
			if d.data.ExplainError != "" {
				native.Text(c, "Diagnostics: "+d.data.ExplainError).TextColor(p.Secondary)
			}
			w.settingsSection(c, "details-agent", "Agent", "Agent names belong to the agent; pane labels are managed separately.", p, func(g *settingsGroup) {
				g.row("Name", fallback(a.Name, "Automatic"), func() {
					if native.Button(c, "Rename…").Disabled(d.busy || !w.featureCurrent(d.op) || w.hooks.RenameAgent == nil || a.Agent == "").Clicked() {
						op := d.op
						w.ask(shortcutPrompt{title: "Name agent", value: a.Name, placeholder: "Agent name", message: "Leave empty to return to the automatic name.", action: "Save"}, func(value string) {
							var name *string
							if strings.TrimSpace(value) != "" {
								name = &value
							}
							d.busy = true
							gen := d.generation
							w.hooks.RenameAgent(op, name, func(err error) {
								if gen != d.generation {
									return
								}
								d.busy = false
								if !w.featureCurrent(op) {
									return
								}
								if err != nil {
									d.error = "The name may have changed. Refresh before repeating. " + err.Error()
									return
								}
								w.loadDetails()
							})
						})
					}
				})
				g.row("Agent", fallback(a.Agent, "No agent detected"), func() {})
				g.row("Status", fallback(a.Status, "Unknown"), func() {})
				g.row("Terminal", d.op.TerminalID, func() {})
				g.row("Working directory", fallback(a.ForegroundCWD, a.CWD), func() {})
				state := "No session reference reported"
				if a.Session != nil {
					state = fmt.Sprintf("%s · %s · %s", a.Session.Agent, a.Session.Kind, a.Session.Value)
				}
				g.row("Session reference", state, func() {})
				g.row("Readiness", fmt.Sprintf("Interactive: %t · Launch pending: %t · Detection skipped: %t", a.InteractiveReady, a.LaunchPending, a.DetectionSkipped), func() {})
			})
			pr := d.data.Process
			w.settingsSection(c, "details-process", "Processes", "Values are reported by Herdr. Missing values are unavailable on this platform.", p, func(g *settingsGroup) {
				g.row("Shell PID", pidText(pr.ShellPID), func() {})
				g.row("TTY", fallback(pr.TTY, "Unavailable"), func() {})
				g.row("Foreground process group", pidText(pr.ForegroundGroup), func() {})
				for _, process := range pr.Processes {
					g.row(fmt.Sprintf("%s · PID %d", process.Name, process.PID), strings.TrimSpace(process.Command+"\n"+process.CWD), func() {})
				}
			})
			w.settingsSection(c, "details-diagnostics", "Detection diagnostics", "Read-only observations from Herdr; refreshing does not scroll terminal history.", p, func(g *settingsGroup) {
				g.row("Detection", fallback(a.Status, "No state reported"), func() {
					if native.Button(c, "Copy diagnostics").Disabled(w.hooks.WriteClipboard == nil || len(d.data.Explain) == 0).Clicked() {
						var pretty bytes.Buffer
						if json.Indent(&pretty, d.data.Explain, "", "  ") == nil {
							w.shortcutError(w.hooks.WriteClipboard(pretty.String()))
						}
					}
				})
				for _, item := range diagnosticSummary(d.data.Explain) {
					g.row(item[0], item[1], func() {})
				}
			})
			w.outputWatchControls(c, p)
		})
	})
}
func diagnosticSummary(data json.RawMessage) [][2]string {
	var values map[string]json.RawMessage
	if json.Unmarshal(data, &values) != nil {
		return nil
	}
	var rows [][2]string
	for _, field := range [][2]string{{"final_state", "Final state"}, {"status", "Status"}, {"source", "Source"}, {"manifest_source", "Manifest source"}, {"manifest_version", "Manifest version"}, {"matched_rule", "Matched rule"}, {"reason", "Reason"}} {
		var value string
		if raw, ok := values[field[0]]; ok && json.Unmarshal(raw, &value) == nil && value != "" {
			rows = append(rows, [2]string{field[1], value})
		}
	}
	return rows
}
func pidText(pid *uint32) string {
	if pid == nil {
		return "Unavailable"
	}
	return strconv.FormatUint(uint64(*pid), 10)
}
func fallback(value, otherwise string) string {
	if value != "" {
		return value
	}
	return otherwise
}
func (w *Window) promptOutputWatch() {
	if w.outputWatch.cancel != nil {
		w.outputWatch.cancel()
		w.outputWatch.cancel = nil
		w.outputWatch.generation++
		w.outputWatch.status = "Output reminder cancelled."
		return
	}
	op, ok := w.operation(session.ExportLayout, "")
	if !ok {
		return
	}
	w.promptOutputWatchFor(op)
}
func (w *Window) promptOutputWatchFor(op session.Operation) {
	if !w.featureCurrent(op) || w.hooks.WatchOutput == nil {
		return
	}
	w.ask(shortcutPrompt{title: "Notify when output matches", placeholder: "Text to match", message: "Watches the latest 80 lines. Existing matching output triggers immediately. Matching mode and timeout can be set in Details. Cancel from the Pane menu or Details.", action: "Watch", required: true}, func(text string) {
		if !w.featureCurrent(op) {
			return
		}
		s := &w.outputWatch
		s.generation++
		gen := s.generation
		s.op = op
		s.status = "Waiting for output: " + text
		duration := 30 * time.Minute
		if minutes, err := strconv.Atoi(s.minutes); err == nil && minutes > 0 && minutes <= 1440 {
			duration = time.Duration(minutes) * time.Minute
		}
		s.cancel = w.hooks.WatchOutput(op, herdr.OutputWatch{Text: text, Regex: s.regex, Timeout: duration}, func(match herdr.OutputMatch, err error) {
			if gen != s.generation {
				return
			}
			s.cancel = nil
			if !w.featureCurrent(op) {
				s.status = "Output reminder ended because the terminal changed."
				return
			}
			if err != nil {
				s.status = "Output reminder ended: " + err.Error()
				return
			}
			s.status = "Matched: " + text
			w.deliverOutputNotice(app.Notice{Kind: "OutputMatched", Title: "Output matched", Body: fallback(match.MatchedLine, text), WorkspaceID: op.WorkspaceID, TabID: op.TabID, PaneID: op.PaneID, Received: time.Now()})
		})
	})
}
func (w *Window) outputWatchControls(c *native.Context, p design.Palette) {
	w.settingsSection(c, "details-output", "Output reminder", "Matches the latest 80 lines without scrolling. Existing matches trigger immediately. One reminder runs per window.", p, func(g *settingsGroup) {
		if w.outputWatch.minutes == "" {
			w.outputWatch.minutes = "30"
		}
		if w.outputWatch.cancel == nil {
			g.toggle("output-regex", "Regular expression", "Uses Herdr's Rust regex syntax.", &w.outputWatch.regex)
			g.row("Timeout in minutes", "From 1 to 1440 minutes; invalid values use 30 minutes.", func() {
				native.TextInput(c.Key("output-minutes"), &w.outputWatch.minutes).Label("Timeout in minutes").Width(80)
			})
		}
		g.row("Reminder", w.outputWatch.status, func() {
			label := "Watch output…"
			if w.outputWatch.cancel != nil {
				label = "Cancel reminder"
			}
			if native.Button(c, label).Disabled(w.hooks.WatchOutput == nil || (w.outputWatch.cancel == nil && !w.featureCurrent(w.details.op))).Clicked() {
				if w.outputWatch.cancel != nil {
					w.promptOutputWatch()
				} else {
					w.promptOutputWatchFor(w.details.op)
				}
			}
		})
	})
}

func (w *Window) deliverOutputNotice(n app.Notice) { w.toasts.Receive(n); w.PumpNotifications() }

func (w *Window) ask(prompt shortcutPrompt, submit func(string)) {
	if w.controller == nil {
		return
	}
	prompt.focus = true
	prompt.submit = submit
	w.controller.shortcut.prompt = &prompt
	w.BlurInput()
}

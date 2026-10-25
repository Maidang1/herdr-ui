package ui

import (
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/session"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/input"
	native "github.com/egoist/mygo/ui"
)

func (w *Window) shortcutPanel(c *native.Context, p design.Palette) {
	if w.controller == nil {
		return
	}
	a := &w.controller.shortcut
	w.applyShortcutFocus(w.app.Publication(), time.Now())

	if a.navigate {
		native.Column(c.Key("navigator")).FillWidth().Padding(12).Gap(4).Background(p.Elevated).Children(func() {
			native.Text(c, "Navigate · ↑ ↓ workspace · h j k l pane · 1–9 workspace · Enter to open · Esc to cancel").FontSize(12)
			for i, row := range w.currentView().Rows {
				button := native.Button(c.Key("navigate:"+row.ID), fmt.Sprintf("%d  %s", i+1, row.Name)).Label("Navigate to " + row.Name)
				if row.ID == a.navigateWorkspace {
					button.Background(p.Inset)
				}
				if button.Clicked() {
					w.app.SelectWorkspace(row.ID)
					a.navigate = false
					w.BlurInput()
				}
			}
		})
	}
	w.navigatorPanel(c, p)
	if cp := a.copy; cp != nil {
		panel := native.Column(c.Key("copy-mode")).FillWidth().Padding(12).Gap(4).Background(p.Elevated).Label("Copy mode")
		panel.HandleInput(func(ev native.InputEvent) bool {
			if ev.Kind != native.InputKeyDown {
				return false
			}
			key, ok := input.NativeKey(ev.Mods, ev.Key)
			if !ok {
				return false
			}
			if key == w.controller.machine.Bindings.Prefix || w.controller.machine.Awaiting {
				result := w.controller.machine.Handle(key, false)
				if len(result.Send) > 0 {
					w.sendCopyLiteral(result.Send)
				}
				if result.Action != "" {
					w.DispatchIndex(result.Action, result.Index)
				}
				return true
			}
			if key == "/" || key == "?" {
				reverse := key == "?"
				prompt := searchPrompt(cp.query, reverse)
				prompt.focus, prompt.submit = true, func(query string) { cp.query = query; cp.reverse = reverse; cp.search(reverse); cp.focus = true }
				a.prompt = &prompt
				return true
			}
			exit, copy := cp.key(key)
			if copy && w.hooks.WriteClipboard != nil {
				w.shortcutError(w.hooks.WriteClipboard(cp.selectedText()))
			}
			if exit {
				a.copy = nil
				w.BlurInput()
			}
			return true
		})
		if cp.focus && a.prompt == nil {
			panel.Focus()
			cp.focus = false
		}
		panel.Children(func() {
			native.Row(c).Gap(8).Children(func() {
				native.Text(c, fmt.Sprintf("Copy mode · line %d/%d · v select · y copy · / search · q close", cp.row+1, len(cp.lines))).Grow(1).FontSize(12)
				if native.Button(c, "Close copy mode").Clicked() {
					a.copy = nil
					w.BlurInput()
				}
			})
			start := max(0, cp.row-7)
			end := min(len(cp.lines), start+15)
			for i := start; i < end; i++ {
				row := native.Row(c.Key(fmt.Sprintf("copy-line:%d", i))).FillWidth()
				if i == cp.row {
					row.Background(p.Inset)
				}
				row.Children(func() {
					native.Text(c, fmt.Sprintf("%5d", i+1)).Width(48).TextColor(p.Secondary)
					runes := []rune(cp.lines[i])
					lo, hi := cp.selectionBounds(i)
					if len(runes) == 0 {
						runes = []rune{' '}
					}
					for col, r := range runes {
						cell := native.Text(c, string(r)).Font("monospace")
						if col >= lo && col < hi && lo >= 0 {
							cell.Background(p.Hover)
						}
						if i == cp.row && col == cp.col {
							cell.Background(p.Text).TextColor(p.Content)
						}
					}
				})
			}
			if cp.query != "" {
				native.Text(c, "Search: "+cp.query).FontSize(12).TextColor(p.Secondary)
			}
		})
	}
}

// shortcutDialog asks before an action as a Mac alert does: what it acts
// on as the title, what follows under it, then the field when it takes
// text, and the buttons at the trailing edge with the default, named by its
// verb, last. Return chooses the default and Escape cancels.
func (w *Window) shortcutDialog(c *native.Context, p design.Palette) {
	if w.controller == nil {
		return
	}
	a := &w.controller.shortcut
	prompt := a.prompt
	if prompt == nil {
		return
	}
	open := true
	width, _ := c.Size()
	action := prompt.action
	if action == "" {
		action = "OK"
	}
	closePrompt := func() {
		a.prompt = nil
		if a.copy != nil {
			a.copy.focus = true
		}
		w.BlurInput()
	}
	native.DialogBase(c.Key("shortcut-prompt"), &open, func(backdrop, panel native.Element) {
		backdrop.Background(native.RGBA(0, 0, 0, .32))
		panel.Label("Action confirmation dialog").Width(min(420, max(0, width-48))).Padding(20).Radius(w.shape.Dialog).Shadow(0, 16, 48, 0, p.ContentShadow).AlignItems(native.Stretch)
		w.floating(c, panel, p, glassDense)
		native.Column(c).FillWidth().Gap(6).Children(func() {
			native.Text(c, prompt.title).FontSize(15).FontWeight(600).FillWidth()
			if prompt.message != "" {
				native.Text(c, prompt.message).FontSize(13).TextColor(p.Secondary).FillWidth()
			}
		})
		submitted := false
		if !prompt.confirm {
			field := native.TextInput(c.Key("shortcut-value"), &prompt.value).Label(prompt.title).Placeholder(prompt.placeholder).FillWidth().Margin(14, 0, 0, 0)
			submitted = field.Submitted()
			if prompt.focus {
				field.Focus()
				prompt.focus = false
			}
		}
		ready := !prompt.required || strings.TrimSpace(prompt.value) != ""
		native.Row(c).FillWidth().Margin(20, 0, 0, 0).Gap(8).AlignItems(native.Center).Children(func() {
			native.Box(c).Grow(1)
			cancel := w.dialogButton(c, "prompt-cancel", "Cancel", false, false, true, p).Clicked()
			accept := w.dialogButton(c, "prompt-accept", action, true, prompt.destructive, ready, p)
			if prompt.confirm && prompt.focus {
				accept.Focus()
				prompt.focus = false
			}
			submit := ready && (submitted || accept.Clicked() || (panel.FocusWithin() && panel.Shortcut(0, native.KeyEnter)))
			cancel = cancel || (panel.FocusWithin() && panel.Shortcut(0, native.KeyEscape))
			if cancel {
				closePrompt()
			} else if submit {
				a.prompt = nil
				prompt.submit(prompt.value)
				w.BlurInput()
			}
		})
	})
	if !open && a.prompt == prompt {
		closePrompt()
	}
}

// dialogButton is a push button of a dialog: the default one filled with
// the accent, or red when it destroys, the others quiet.
func (w *Window) dialogButton(c *native.Context, key, label string, primary, destructive, enabled bool, p design.Palette) native.Element {
	button := native.ButtonBase(c.Key(key)).Label(label).Height(30).MinWidth(84).Padding(0, 14).Radius(max(6, w.shape.Dialog-20)).Center().Disabled(!enabled)
	fill, text := p.Hover, p.Text
	if primary {
		// The accent and red are light in dark windows and deep in light
		// ones: the label takes the opposite end.
		fill, text = p.Accent, native.RGB(255, 255, 255)
		if c.Theme().Dark {
			text = native.RGB(28, 26, 32)
		}
		if destructive {
			fill = p.Danger
		}
	}
	switch {
	case !enabled:
		fill, text = fill.Alpha(.45), text.Alpha(.6)
	case button.Pressed():
		fill = fill.Mix(p.Text, .18)
	case button.Hovered():
		fill = fill.Mix(p.Text, .08)
	}
	button.Background(fill)
	if !primary {
		button.Border(1, p.Border)
	}
	if button.FocusVisible() {
		button.Border(2, p.Focus)
	}
	button.Children(func() {
		native.Text(c, label).FontSize(13).FontWeight(500).TextColor(text).SingleLine()
	})
	return button
}

func (w *Window) modalOpen() bool {
	return w.referenceOpen() || w.controller != nil && (w.controller.shortcut.prompt != nil || w.controller.shortcut.copy != nil || w.controller.shortcut.navigator || w.commandSurfaceState != nil)
}

// Navigation highlights a workspace without changing selection until Enter.
func (w *Window) navigateWorkspace(delta int) {
	a := &w.controller.shortcut
	v := w.currentView()
	index := 0
	for i, row := range v.Rows {
		if row.ID == a.navigateWorkspace {
			index = i
			break
		}
	}
	index = wrap(index+delta, len(v.Rows))
	if index >= 0 {
		a.navigateWorkspace = v.Rows[index].ID
	}
}

func (w *Window) navigationAction(key string) string {
	action := w.controller.machine.Bindings.Navigation[key]
	if key == "left" {
		return "navigate_pane_left"
	}
	if key == "right" {
		return "navigate_pane_right"
	}
	return strings.TrimSpace(action)
}

// applyShortcutFocus only consumes publications already applied to App. A
// successful focus acknowledgement may arrive before its zoom geometry.
func (w *Window) applyShortcutFocus(p session.PublishedSnapshot, now time.Time) {
	if w.controller == nil {
		return
	}
	a := &w.controller.shortcut
	if follow := a.followCommand; follow != nil {
		if p.TargetID != follow.target || p.Epoch != follow.epoch || w.app.ViewRevision() != follow.viewRevision || !now.Before(follow.expires) {
			a.followCommand = nil
		} else if p.State == session.Ready && !p.Stale && p.Snapshot != nil && p.Revision > follow.publicationRevision {
			id := p.Snapshot.Metadata().FocusedPaneID
			if id != "" && id != follow.sourcePane && paneVisibleForSelection(p.Snapshot, id) {
				a.followCommand = nil
				w.app.SelectPane(id)
				w.BlurInput()
			}
		}
	}
	if a.pendingPane != "" {
		if p.TargetID != a.pendingTarget || p.Epoch != a.pendingEpoch || w.app.ViewRevision() != a.pendingRevision || !now.Before(a.pendingUntil) {
			a.pendingPane = ""
		} else if p.State == session.Ready && !p.Stale && p.Snapshot != nil && p.Revision > a.pendingPublication && paneVisibleForSelection(p.Snapshot, a.pendingPane) {
			id := a.pendingPane
			a.pendingPane = ""
			w.app.SelectPane(id)
			w.BlurInput()
		}
	}
}
func paneVisibleForSelection(snapshot *session.Snapshot, id string) bool {
	pane, ok := snapshot.Pane(id)
	if !ok {
		return false
	}
	layout, exists := snapshot.Layout(pane.TabID)
	return !exists || !layout.Zoomed || layout.FocusedPaneID == id
}
func (w *Window) sendCopyLiteral(data []byte) {
	if w.terminal == nil || w.terminal.session == nil || w.terminal.host == nil {
		return
	}
	a := &w.controller.shortcut
	p := w.terminal.session.Latest()
	v := w.currentView()
	if p.State != session.Ready || p.Stale || a.copy == nil || a.copyPane == "" || v.PaneID != a.copyPane || p.TargetID != a.copyTarget || p.Epoch != a.copyEpoch {
		return
	}
	child := w.terminal.panes[a.copyPane]
	if child == nil || child.spec.TerminalID != a.copyTerminal || !bindingMatchesPublication(child.binding, child.spec, p) {
		return
	}
	w.terminal.host.Send(child.binding, data)
}

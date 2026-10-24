package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/session"
)

// ActionHooks are platform integrations. All callbacks run on the UI thread.
type ActionHooks struct {
	Detach             func()
	Settings           func() error
	ReadImage          func() []byte
	PasteImage         func(string, []byte) error
	WriteClipboard     func(string) error
	EditText           func(string) error
	InvokeCommand      func(int, session.Operation, func(error))
	NotificationTarget func() string
	NotificationText   func() (string, time.Duration)
	// FontSize applies a new terminal font size to every attached
	// terminal and persists it.
	FontSize func(size float32)
}

func (w *Window) SetActionHooks(h ActionHooks) { w.hooks = h }

type shortcutPrompt struct {
	title, value string
	confirm      bool
	focus        bool
	submit       func(string)
}
type shortcutState struct {
	navigator                   bool
	navigatorQuery              string
	navigatorIndex              int
	navigatorFocus              bool
	followCommand               *commandFocusIntent
	navigate                    bool
	navigateWorkspace           string
	prompt                      *shortcutPrompt
	busy                        bool
	pendingPane                 string
	pendingRevision             uint64
	pendingPublication          uint64
	pendingUntil                time.Time
	copyPane, copyTerminal      string
	copyTarget, copyEpoch       uint64
	pendingTarget, pendingEpoch uint64
	copy                        *copyState
}

func extendedAction(action string) bool {
	switch action {
	case "help", "toggle_sidebar", "previous_tab", "next_tab", "split_vertical", "split_horizontal", "focus_pane_left", "focus_pane_down", "focus_pane_up", "focus_pane_right", "close_pane", "zoom", "resize_mode", "resize_left", "resize_right", "resize_up", "resize_down", "zoom_in", "zoom_out", "zoom_reset", "attention", "zen", "find", "find_next", "find_previous":
		return false
	}
	for _, a := range Actions {
		if a.ID == action {
			return true
		}
	}
	return strings.HasPrefix(action, "command[")
}

func (w *Window) canShortcut(action string) (bool, bool) {
	if !extendedAction(action) {
		return false, false
	}
	if w.controller == nil {
		return true, false
	}
	switch action {
	case "settings", "detach":
		return true, true
	}
	v := w.currentView()
	if v.State != session.Ready || v.Stale {
		return true, false
	}
	if action == "new_workspace" || action == "reload_config" {
		return true, !w.controller.shortcut.busy
	}
	return true, v.PaneID != "" && !w.controller.shortcut.busy
}

func (w *Window) dispatchShortcut(action string, index int) bool {
	if !extendedAction(action) {
		return false
	}
	if !w.Can(action) {
		return true
	}
	a := &w.controller.shortcut
	w.controller.message = ""
	w.controller.machine.Reset()
	if action != "workspace_picker" && action != "goto" {
		a.navigate = false
	}
	v := w.currentView()
	switch action {
	case "settings":
		if w.hooks.Settings != nil {
			w.shortcutError(w.hooks.Settings())
		}
		return true
	case "detach":
		if w.hooks.Detach != nil {
			w.hooks.Detach()
		}
		return true
	case "goto":
		a.navigator = true
		a.navigatorQuery = ""
		a.navigatorIndex = 0
		a.navigatorFocus = true
		w.navigatorSessions++
		return true
	case "workspace_picker":
		a.navigate = true
		a.navigateWorkspace = v.WorkspaceID
		return true
	case "previous_workspace", "next_workspace", "switch_workspace":
		next := index
		if action != "switch_workspace" {
			for i, row := range v.Rows {
				if row.ID == v.WorkspaceID {
					next = i
					break
				}
			}
			if action == "previous_workspace" {
				next--
			} else {
				next++
			}
			next = wrap(next, len(v.Rows))
		}
		if next >= 0 && next < len(v.Rows) {
			w.app.SelectWorkspace(v.Rows[next].ID)
			w.BlurInput()
		}
		return true
	case "switch_tab":
		if index >= 0 && index < len(v.Tabs) {
			w.app.SelectTab(v.Tabs[index].ID)
			w.BlurInput()
		}
		return true
	case "cycle_pane_next", "cycle_pane_previous":
		for i, pane := range v.Panes {
			if pane.ID == v.PaneID {
				delta := 1
				if action == "cycle_pane_previous" {
					delta = -1
				}
				w.selectShortcutPane(v.Panes[wrap(i+delta, len(v.Panes))].ID)
				break
			}
		}
		return true
	case "last_pane":
		w.selectShortcutPane(w.app.LastPaneID())
		return true
	case "previous_agent", "next_agent", "focus_agent":
		next := index
		if action != "focus_agent" {
			next = -1
			for i, row := range v.Agents {
				if row.PaneID == v.PaneID {
					next = i
					break
				}
			}
			if action == "previous_agent" {
				if next < 0 {
					next = 0
				}
				next--
			} else {
				next++
			}
			next = wrap(next, len(v.Agents))
		}
		if next >= 0 && next < len(v.Agents) {
			w.selectShortcutPane(v.Agents[next].PaneID)
		}
		return true
	case "open_notification_target":
		if w.hooks.NotificationTarget != nil {
			if id := w.hooks.NotificationTarget(); id != "" {
				w.selectShortcutPane(id)
			}
		}
		return true
	case "remote_image_paste":
		if w.hooks.ReadImage != nil && w.hooks.PasteImage != nil {
			if data := w.hooks.ReadImage(); len(data) > 0 {
				w.shortcutError(w.hooks.PasteImage(v.PaneID, data))
			}
		}
		return true
	case "rename_workspace", "rename_tab", "rename_pane", "new_worktree", "open_worktree":
		value := ""
		switch action {
		case "rename_workspace":
			value = v.WorkspaceName
		case "rename_tab":
			for _, tab := range v.Tabs {
				if tab.ID == v.TabID {
					value = tab.Name
				}
			}
		case "rename_pane":
			for _, pane := range v.Panes {
				if pane.ID == v.PaneID {
					value = pane.Label
				}
			}
		}
		op, ok := w.shortcutOperation(action)
		if !ok {
			return true
		}
		a.prompt = &shortcutPrompt{title: w.ActionLabel(action), value: value, focus: true, submit: func(text string) { op.Text = text; w.runShortcut(op, nil) }}
		return true
	case "close_tab", "close_workspace", "remove_worktree":
		op, ok := w.shortcutOperation(action)
		if !ok {
			return true
		}
		title := w.ActionLabel(action) + "?"
		if action == "close_workspace" {
			title = "Close workspace " + v.WorkspaceName + " and its linked workspaces?"
		}
		if action == "remove_worktree" {
			title = "Remove worktree checkout for " + v.WorkspaceName + "?"
		}
		a.prompt = &shortcutPrompt{title: title, confirm: true, focus: true, submit: func(string) { w.runShortcut(op, nil) }}
		return true
	case "copy_mode":
		w.readHistory(func(text string) {
			a.copy = newCopyState(text)
			a.copyPane = v.PaneID
			a.copyTarget = v.TargetID
			a.copyEpoch = v.Epoch
			for _, pane := range v.Panes {
				if pane.ID == v.PaneID {
					a.copyTerminal = pane.TerminalID
				}
			}
		})
		return true
	case "edit_scrollback":
		w.readHistory(func(text string) {
			if w.hooks.EditText != nil {
				w.shortcutError(w.hooks.EditText(text))
			}
		})
		return true
	}
	op, ok := w.shortcutOperation(action)
	if !ok {
		return true
	}
	if strings.HasPrefix(action, "command[") {
		if a.copy != nil && a.copy.selecting && a.copyPane == op.PaneID && a.copyTerminal == op.TerminalID && a.copyTarget == op.TargetID && a.copyEpoch == op.Epoch {
			selected := a.copy.selectedText()
			op.SelectedText = &selected
		}
		if w.hooks.InvokeCommand != nil {
			a.busy = true
			w.hooks.InvokeCommand(index, op, func(err error) {
				a.busy = false
				w.shortcutError(err)
				if w.terminal != nil {
					w.terminal.session.Refresh()
				}
			})
		}
		return true
	}
	if action == "move_tab_previous" || action == "move_tab_next" {
		for i, tab := range v.Tabs {
			if tab.ID == v.TabID {
				op.Index = i
				break
			}
		}
		if len(v.Tabs) < 2 {
			return true
		}
		if action == "move_tab_previous" {
			if op.Index == 0 {
				op.Index = len(v.Tabs)
			} else {
				op.Index--
			}
		} else {
			if op.Index+1 >= len(v.Tabs) {
				op.Index = 0
			} else {
				op.Index += 2
			}
		}

	}
	w.runShortcut(op, func(r session.OperationResult) {
		if action == "reload_config" {
			b, err := input.LoadBindings("")
			w.ConfigureBindings(b, err)
			var status struct {
				Status      string   `json:"status"`
				Diagnostics []string `json:"diagnostics"`
			}
			_ = json.Unmarshal(r.Data, &status)
			if len(status.Diagnostics) > 0 {
				w.controller.message = strings.Join(status.Diagnostics, "\n")
			} else if status.Status != "applied" {
				w.controller.message = "Configuration reload: " + status.Status
			}
		}
	})
	return true
}

func wrap(i, n int) int {
	if n == 0 {
		return -1
	}
	return ((i % n) + n) % n
}
func (w *Window) shortcutError(err error) {
	if err != nil {
		w.controller.message = err.Error()
	}
}
func (w *Window) shortcutOperation(action string) (session.Operation, bool) {
	if w.terminal == nil || w.terminal.session == nil {
		return session.Operation{}, false
	}
	v := w.currentView()
	p := w.terminal.session.Latest()
	if p.State != session.Ready || p.Stale || p.Snapshot == nil || v.TargetID != p.TargetID || v.Epoch != p.Epoch {
		return session.Operation{}, false
	}
	return session.Operation{Kind: session.Shortcut, Action: action, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: v.WorkspaceID, TabID: v.TabID, PaneID: v.PaneID, TerminalID: func() string { pane, _ := p.Snapshot.Pane(v.PaneID); return pane.TerminalID }()}, true
}
func (w *Window) runShortcut(op session.Operation, done func(session.OperationResult)) {
	a := &w.controller.shortcut
	if a.busy {
		return
	}
	if w.terminal == nil || w.terminal.session == nil {
		a.busy = false
		w.controller.message = "No attached session."
		return
	}
	a.busy = true
	revision := w.app.ViewRevision()
	publicationRevision := w.app.Publication().Revision
	w.execute(op, func(r session.OperationResult) {
		a.busy = false
		p := w.terminal.session.Latest()
		if p.TargetID != op.TargetID || p.Epoch != op.Epoch {
			return
		}
		if r.Status != session.Succeeded {
			if op.Action == "close_tab" && w.offerGroupClose(op, r) {
				return
			}
			w.controller.message = fmt.Sprintf("%s: %s", r.Status, r.Error)
			w.terminal.session.Refresh()
			return
		}
		if r.PaneID != "" && w.app.ViewRevision() == revision {
			a.pendingPane = r.PaneID
			a.pendingRevision = revision
			a.pendingTarget = op.TargetID
			a.pendingEpoch = op.Epoch
			a.pendingPublication = publicationRevision
			a.pendingUntil = time.Now().Add(5 * time.Second)
		}
		if done != nil {
			done(r)
		}
		w.terminal.session.Refresh()
	})
}
func (w *Window) readHistory(done func(string)) {
	op, ok := w.shortcutOperation("read_scrollback")
	if !ok {
		return
	}
	revision := w.app.ViewRevision()
	w.runShortcut(op, func(r session.OperationResult) {
		if w.app.ViewRevision() != revision {
			return
		}
		var response struct {
			Read struct {
				Text      string `json:"text"`
				PaneID    string `json:"pane_id"`
				Truncated bool   `json:"truncated"`
			} `json:"read"`
		}
		if err := json.Unmarshal(r.Data, &response); err != nil || response.Read.PaneID != op.PaneID {
			w.controller.message = "History response did not match the selected pane"
			return
		}
		if response.Read.Truncated {
			w.controller.message = "History was truncated by the server"
		}
		done(response.Read.Text)
	})
}

func (w *Window) navigateKey(key string) bool {
	a := &w.controller.shortcut
	if key == "enter" {
		w.app.SelectWorkspace(a.navigateWorkspace)
		a.navigate = false
		w.BlurInput()
		return true
	}
	if key == "esc" || key == w.controller.machine.Bindings.Prefix {
		a.navigate = false
		return true
	}
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		w.DispatchIndex("switch_workspace", int(key[0]-'1'))
		return true
	}
	action := w.controller.machine.Bindings.Navigation[key]
	if key == "left" {
		action = "navigate_pane_left"
	}
	if key == "right" {
		action = "navigate_pane_right"
	}
	if key == "tab" {
		w.Dispatch("cycle_pane_next")
		a.navigate = true
		return true
	}
	if key == "shift+tab" {
		w.Dispatch("cycle_pane_previous")
		a.navigate = true
		return true
	}
	switch action {
	case "navigate_workspace_up":
		w.navigateWorkspace(-1)
	case "navigate_workspace_down":
		w.navigateWorkspace(1)
	case "navigate_pane_left", "navigate_pane_down", "navigate_pane_up", "navigate_pane_right":
		w.Dispatch(strings.Replace(action, "navigate_", "focus_", 1))
	default:
		result := w.controller.machine.Handle(key, false)
		if result.Action == "" {
			w.controller.machine.Awaiting = true
			result = w.controller.machine.Handle(key, false)
		}
		if result.Action != "" {
			switch result.Action {
			case "rename_workspace", "close_workspace", "new_worktree", "open_worktree", "remove_worktree", "new_tab":
				w.app.SelectWorkspace(a.navigateWorkspace)
			}
			w.DispatchIndex(result.Action, result.Index)
		}
		return true
	}
	a.navigate = true
	return true
}

func (w *Window) ReportActionError(err error) { w.shortcutError(err) }

type commandFocusIntent struct {
	target, epoch, viewRevision, publicationRevision uint64
	sourcePane                                       string
	expires                                          time.Time
}

// FollowCommandFocus uses intent captured before command.invoke. Capturing it
// after completion would let an older command steal a newer native selection.
func (w *Window) FollowCommandFocus(target, epoch, viewRevision, publicationRevision uint64, sourcePane string) {
	if w.controller == nil || sourcePane == "" {
		return
	}
	p := w.app.Publication()
	if p.TargetID != target || p.Epoch != epoch || w.app.ViewRevision() != viewRevision {
		return
	}
	w.controller.shortcut.followCommand = &commandFocusIntent{target: target, epoch: epoch, viewRevision: viewRevision, publicationRevision: publicationRevision, sourcePane: sourcePane, expires: time.Now().Add(5 * time.Second)}
}

// selectShortcutPane keeps normal navigation local. A hidden zoomed pane first
// needs authoritative focus so the server exposes its terminal rectangle.
func (w *Window) selectShortcutPane(id string) {
	p := w.app.Publication()
	if p.Snapshot == nil || p.State != session.Ready || p.Stale {
		return
	}
	pane, ok := p.Snapshot.Pane(id)
	if !ok {
		return
	}
	layout, hasLayout := p.Snapshot.Layout(pane.TabID)
	if hasLayout && layout.Zoomed && layout.FocusedPaneID != id {
		if w.terminal == nil || w.terminal.session == nil || w.controller == nil {
			return
		}
		latest := w.terminal.session.Latest()
		if latest.TargetID != p.TargetID || latest.Epoch != p.Epoch || latest.Revision != p.Revision {
			return
		}
		op := session.Operation{Kind: session.Shortcut, Action: "select_pane", TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: pane.WorkspaceID, TabID: pane.TabID, PaneID: pane.ID, TerminalID: pane.TerminalID}
		w.runShortcut(op, nil)
		return
	}
	w.app.SelectPane(id)
	w.BlurInput()
}

func (w *Window) SurfaceSize() (uint16, uint16) {
	v := w.currentView()
	return uint16(min(65535, max(1, v.Layout.Area.W))), uint16(min(65535, max(1, v.Layout.Area.H)))
}

// A rejected final pane/tab close may require consent to close the linked group.
// This is a new, explicitly confirmed operation; an uncertain mutation is never retried.
func (w *Window) offerGroupClose(op session.Operation, r session.OperationResult) bool {
	if r.Status != session.Rejected || !strings.Contains(r.Error, "confirmation_required") || w.controller == nil {
		return false
	}
	title := "Close workspace " + op.WorkspaceID + " and all linked workspaces?"
	op.Kind = session.Shortcut
	op.Action = "close_workspace"
	w.controller.shortcut.prompt = &shortcutPrompt{title: title, confirm: true, focus: true, submit: func(string) { w.app.ClearActionMessage(); w.runShortcut(op, nil) }}
	return true
}

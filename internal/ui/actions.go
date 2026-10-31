package ui

import (
	"fmt"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/Maidang1/herdr-ui/internal/terminalhost"
	"github.com/egoist/mygo/transfer"
	native "github.com/egoist/mygo/ui"
)

type actionController struct {
	machine         input.Machine
	bindingError    string
	message         string
	help            bool
	revision        uint64
	target, epoch   uint64
	guarded         bool
	guardKnown      bool
	ratioGeneration uint64
	shortcut        shortcutState
	drag            *ratioDrag
}

type Action struct{ ID, Label string }

var Actions = []Action{
	{"pane_details", "Agent and process details"}, {"watch_output", "Notify when output matches"},
	{"help", "Keyboard reference"}, {"settings", "Settings"}, {"new_workspace", "New workspace"}, {"new_worktree", "New worktree"}, {"open_worktree", "Open worktree"}, {"remove_worktree", "Remove worktree"}, {"rename_workspace", "Rename workspace"}, {"close_workspace", "Close workspace"}, {"workspace_picker", "Workspace navigator"}, {"goto", "Session navigator"}, {"detach", "Detach window"}, {"reload_config", "Reload configuration"}, {"open_notification_target", "Open notification target"}, {"previous_workspace", "Previous workspace"}, {"next_workspace", "Next workspace"}, {"previous_agent", "Previous agent"}, {"next_agent", "Next agent"}, {"focus_agent", "Focus agent by number"}, {"remote_image_paste", "Paste clipboard image"}, {"new_tab", "New tab"}, {"rename_tab", "Rename tab"}, {"previous_tab", "Previous tab"}, {"next_tab", "Next tab"}, {"move_tab_previous", "Move tab left"}, {"move_tab_next", "Move tab right"}, {"switch_tab", "Switch tab by number"}, {"switch_workspace", "Switch workspace by number"}, {"close_tab", "Close tab"}, {"rename_pane", "Rename pane"}, {"edit_scrollback", "Edit scrollback"}, {"copy_mode", "Copy mode"}, {"focus_pane_left", "Focus left"}, {"focus_pane_down", "Focus down"}, {"focus_pane_up", "Focus up"}, {"focus_pane_right", "Focus right"}, {"swap_pane_left", "Swap left"}, {"swap_pane_down", "Swap down"}, {"swap_pane_up", "Swap up"}, {"swap_pane_right", "Swap right"}, {"cycle_pane_next", "Next pane"}, {"cycle_pane_previous", "Previous pane"}, {"last_pane", "Last pane"}, {"split_vertical", "Split right"}, {"split_horizontal", "Split down"}, {"close_pane", "Close pane"}, {"zoom", "Zoom / restore"}, {"resize_mode", "Resize mode"}, {"resize_pane_left", "Resize left"}, {"resize_pane_down", "Resize down"}, {"resize_pane_up", "Resize up"}, {"resize_pane_right", "Resize right"}, {"toggle_sidebar", "Toggle sidebar"}, {"zoom_in", "Bigger text"}, {"zoom_out", "Smaller text"}, {"zoom_reset", "Actual size"}, {"attention", "Jump to agent needing you"}, {"find", "Find in terminal"}, {"find_next", "Find next"}, {"find_previous", "Find previous"}, {"clear_pane", "Clear pane"}, {"zen", "Toggle zen mode"},
	{"agent_prompt", "Send prompt to agent"}, {"agent_wait", "Notify when agent stops"}, {"agent_start", "Start agent in pane"}, {"agent_view_next", "Next agent view"},
	{"move_pane_new_tab", "Move pane to new tab"}, {"move_pane_new_workspace", "Move pane to new workspace"}, {"move_workspace_up", "Move workspace up"}, {"move_workspace_down", "Move workspace down"},
	{"save_layout", "Save tab layout"}, {"apply_layout", "New tab from layout"},
}

func (w *Window) ConfigureBindings(b input.Bindings, err error) {
	if w.controller == nil {
		w.controller = &actionController{}
	}
	w.controller.machine.Bindings = b
	w.controller.bindingError = ""
	if err != nil {
		w.controller.bindingError = err.Error()
	}
}
func (w *Window) BlurInput() {
	if w.terminal != nil {
		w.terminal.focused = ""
	}
	if w.controller != nil {
		w.controller.machine.Reset()
		w.controller.drag = nil
	}
}
func (w *Window) syncInput() {
	if w.controller == nil {
		return
	}
	a := w.controller
	v := w.currentView()
	cp := &a.shortcut
	if cp.copy != nil && cp.copyPane != "" {
		p := w.app.Publication()
		valid := false
		if p.Snapshot != nil {
			pane, exists := p.Snapshot.Pane(cp.copyPane)
			valid = exists && pane.TerminalID == cp.copyTerminal
		}
		if !valid || cp.copyPane != v.PaneID || cp.copyTarget != v.TargetID || cp.copyEpoch != v.Epoch {
			cp.copy = nil
			cp.prompt = nil
		}
	}
	if a.revision != w.app.ViewRevision() || a.target != v.TargetID || a.epoch != v.Epoch {
		w.BlurInput()
		a.revision = w.app.ViewRevision()
		a.target = v.TargetID
		a.epoch = v.Epoch
	}
}
func (w *Window) Key(binding terminalhost.Binding, mods native.Modifiers, key native.Key, state terminalhost.InputState) terminalhost.KeyResult {
	var published session.PublishedSnapshot
	if w.terminal != nil && w.terminal.session != nil {
		published = w.terminal.session.Latest()
	}
	return w.keyPublished(binding, mods, key, state, published)
}
func (w *Window) keyPublished(binding terminalhost.Binding, mods native.Modifiers, key native.Key, state terminalhost.InputState, published session.PublishedSnapshot) terminalhost.KeyResult {
	w.syncInput()
	a := w.controller
	if a == nil {
		return terminalhost.KeyResult{}
	}
	t := w.paneForBinding(binding)
	if t == nil || !bindingMatchesPublication(binding, t.spec, published) {
		a.machine.Reset()
		return terminalhost.KeyResult{}
	}
	if w.currentView().PaneID != t.spec.PaneID {
		w.app.SelectPane(t.spec.PaneID)
		w.syncInput()
	}
	a.guarded = state.Guarded
	if state.Composing || !state.Guarded || a.bindingError != "" {
		a.machine.Reset()
		return terminalhost.KeyResult{}
	}
	label, ok := input.NativeKey(mods, key)
	if !ok {
		a.machine.Reset()
		return terminalhost.KeyResult{}
	}
	if a.shortcut.navigate && w.navigateKey(label) {
		return terminalhost.KeyResult{Consumed: true}
	}
	result := a.machine.Handle(label, false)
	if result.Action != "" {
		if result.Action == "remote_image_paste" && w.hooks.ReadImage != nil && len(w.hooks.ReadImage()) == 0 {
			return terminalhost.KeyResult{}
		}
		w.DispatchIndex(result.Action, result.Index)
	}
	return terminalhost.KeyResult{Consumed: result.Consumed, Send: result.Send}
}
func bindingMatchesPublication(binding terminalhost.Binding, spec terminalhost.Spec, published session.PublishedSnapshot) bool {
	if binding.TargetID != published.TargetID || published.Snapshot == nil {
		return false
	}
	pane, ok := published.Snapshot.Pane(spec.PaneID)
	return ok && pane.TerminalID == binding.TerminalID && pane.TerminalID == spec.TerminalID
}
func (w *Window) operation(kind session.OperationKind, direction string) (session.Operation, bool) {
	if w.terminal == nil || w.terminal.session == nil {
		return session.Operation{}, false
	}
	v := w.currentView()
	p := w.terminal.session.Latest()
	if p.State != session.Ready || p.Stale || p.TargetID != v.TargetID || p.Epoch != v.Epoch || p.Snapshot == nil {
		return session.Operation{}, false
	}
	pane, ok := p.Snapshot.Pane(v.PaneID)
	if !ok || pane.WorkspaceID != v.WorkspaceID || pane.TabID != v.TabID {
		return session.Operation{}, false
	}
	return session.Operation{Kind: kind, TargetID: p.TargetID, Epoch: p.Epoch, WorkspaceID: pane.WorkspaceID, TabID: pane.TabID, PaneID: pane.ID, TerminalID: pane.TerminalID, Direction: direction}, true
}
func actionKind(action string) session.OperationKind {
	switch {
	case strings.HasPrefix(action, "split_"):
		return session.Split
	case strings.HasPrefix(action, "focus_pane_"):
		return session.Neighbor
	case action == "zoom":
		return session.Zoom
	case action == "close_pane":
		return session.ClosePane
	}
	return 0
}
func (w *Window) Can(action string) bool {
	if handled, enabled := w.canShortcut(action); handled {
		return enabled
	}
	if action == "help" || action == "toggle_sidebar" || action == "zen" {
		return true
	}
	// Text zoom, the attention jump and the find bar are local to the
	// window: they need an attached session, but no Herdr-side operation.
	switch action {
	case "zoom_in", "zoom_out", "zoom_reset":
		return w.currentView().PaneID != ""
	case "attention":
		return w.currentView().PaneID != ""
	case "find", "find_next", "find_previous":
		return w.terminal != nil && w.currentView().PaneID != ""
	}
	v := w.currentView()
	if action == "next_tab" || action == "previous_tab" {
		return len(v.Tabs) > 1
	}
	kind := actionKind(action)
	if kind == 0 {
		kind = session.ExportLayout
	}
	_, ok := w.operation(kind, "")
	return ok && !w.app.Pending(session.ClosePane, v.PaneID) && !w.app.Pending(kind, v.PaneID) && (kind != session.ExportLayout || (!v.Layout.Zoomed && len(v.Layout.Splits) > 0))
}
func (w *Window) Dispatch(action string) { w.DispatchIndex(action, 0) }
func (w *Window) DispatchIndex(action string, index int) {
	if w.controller == nil {
		return
	}
	a := w.controller
	if w.dispatchShortcut(action, index) {
		return
	}
	known := false
	for _, item := range Actions {
		if item.ID == action {
			known = true
		}
	}
	if !known && !strings.HasPrefix(action, "resize_") {
		a.message = "Unknown action: " + action
		return
	}
	if !w.Can(action) {
		return
	}
	if (action != "help" || a.help) && w.terminal != nil {
		w.terminal.focused = ""
	}
	if action != "resize_mode" && !strings.HasPrefix(action, "resize_") {
		a.machine.Reset()
	}
	switch action {
	case "toggle_sidebar":
		if w.sidebarVisible {
			w.hidden = true
		} else {
			w.hidden = false
		}
		return
	case "help":
		a.help = !a.help
		w.helpFocusPending = a.help
		return
	case "next_tab":
		w.app.CycleTab(1)
		w.BlurInput()
		return
	case "previous_tab":
		w.app.CycleTab(-1)
		w.BlurInput()
		return
	case "resize_mode":
		a.machine.Resize = true
		return
	case "zoom_in":
		w.applyFontZoom(+1)
		return
	case "zoom_out":
		w.applyFontZoom(-1)
		return
	case "zoom_reset":
		w.applyFontZoom(0)
		return
	case "attention":
		w.jumpToAttention()
		return
	case "zen":
		// A view state of this window alone, as Herdr's zen: the session,
		// the sidebar preference and the server's focus stay untouched.
		w.zen = !w.zen
		if w.zen && w.referenceOpen() {
			w.controller.help = false
			w.helpFocusPending = false
		}
		w.BlurInput()
		return
	case "find":
		w.findOpenAction()
		return
	case "find_next":
		w.findStep(false)
		return
	case "find_previous":
		w.findStep(true)
		return
	}
	if strings.HasPrefix(action, "resize_") {
		w.resize(strings.TrimPrefix(action, "resize_"))
		return
	}
	kind := actionKind(action)
	direction := ""
	switch action {
	case "split_vertical":
		direction = "right"
	case "split_horizontal":
		direction = "down"
	}
	if kind == session.Neighbor {
		direction = strings.TrimPrefix(action, "focus_pane_")
	}
	op, ok := w.operation(kind, direction)
	if !ok {
		return
	}
	if kind == session.Neighbor && w.currentView().Layout.Zoomed {
		op.Kind = session.Shortcut
		op.Action = "focus_pane_direction"
		w.runShortcut(op, nil)
		return
	}
	intent, ok := w.app.BeginOperation(op)
	if !ok {
		return
	}
	w.execute(op, func(r session.OperationResult) {
		w.app.CompleteOperation(intent, r)
		if op.Kind == session.ClosePane {
			w.offerGroupClose(op, r)
		}
		w.terminal.session.Refresh()
	})
}
func (w *Window) execute(op session.Operation, done func(session.OperationResult)) {
	t := w.terminal
	result := t.session.Execute(t.ctx, op)
	go func() { r := <-result; t.post(func() { done(r) }) }()
}
func (w *Window) resize(direction string) {
	op, ok := w.operation(session.ExportLayout, "")
	if !ok {
		return
	}
	w.controller.ratioGeneration++
	generation := w.controller.ratioGeneration
	revision := w.app.ViewRevision()
	w.execute(op, func(r session.OperationResult) {
		if r.Status != session.Succeeded {
			w.controller.message = r.Error
			return
		}
		if generation != w.controller.ratioGeneration || !w.sameIntent(op, revision) {
			return
		}
		path, err := layout.SplitForPane(r.Root, op.PaneID, direction)
		if err != nil {
			w.controller.message = err.Error()
			return
		}
		node, _ := layout.ResolvePath(r.Root, path)
		step := .03
		if direction == "left" || direction == "up" {
			step = -step
		}
		op.Kind = session.SetRatio
		op.Path = path
		op.Fingerprint = r.Fingerprint
		op.Ratio = clampRatio(node.Ratio + step)
		w.sendRatio(op, nil, generation)
	})
}
func clampRatio(r float64) float64 { return max(.1, min(.9, r)) }
func (w *Window) sameIntent(op session.Operation, revision uint64) bool {
	if w.terminal == nil {
		return false
	}
	v := w.currentView()
	latest := w.terminal.session.Latest()
	if revision != w.app.ViewRevision() || latest.TargetID != op.TargetID || latest.Epoch != op.Epoch || v.TabID != op.TabID || v.PaneID != op.PaneID || latest.Snapshot == nil {
		return false
	}
	pane, ok := latest.Snapshot.Pane(op.PaneID)
	return ok && pane.TerminalID == op.TerminalID && pane.TabID == op.TabID && pane.WorkspaceID == op.WorkspaceID
}
func (w *Window) sendRatio(op session.Operation, drag *ratioDrag, generation uint64) {
	revision := w.app.ViewRevision()
	w.execute(op, func(r session.OperationResult) {
		w.completeRatio(r, drag, w.sameIntent(op, revision), generation)
		w.terminal.session.Refresh()
	})
}
func (w *Window) completeRatio(r session.OperationResult, drag *ratioDrag, current bool, generation uint64) {
	if !current || generation != w.controller.ratioGeneration {
		return
	}
	if r.Status != session.Succeeded && r.Status != session.Superseded {
		w.controller.message = fmt.Sprintf("%s: %s", r.Status, r.Error)
		if drag != nil && w.controller.drag == drag {
			w.controller.drag = nil
		}
	}
}
func (w *Window) BindingLabel(action string) string {
	if w.controller == nil {
		return ""
	}
	b := w.controller.machine.Bindings
	best := ""
	for k, v := range b.Actions {
		if v == action && (best == "" || k < best) {
			best = k
		}
	}
	return strings.ReplaceAll(best, "prefix+", b.Prefix+" → ")
}

func (w *Window) SetActionStateChanged(fn func(string, bool)) { w.actionState = fn }

// ActionLabel describes the action's current effect for every native entry.
func (w *Window) ActionLabel(id string) string {
	switch id {
	case "watch_output":
		if w.outputWatch.cancel != nil {
			return "Cancel output reminder"
		}
	case "agent_wait":
		if w.agentWatch.cancel != nil {
			return "Cancel agent reminder"
		}
	case "toggle_sidebar":
		if w.sidebarVisible {
			return "Hide sidebar"
		}
		return "Show sidebar"
	case "zoom":
		if w.currentView().Layout.Zoomed {
			return "Restore pane"
		}
		return "Zoom pane"
	}
	for _, action := range Actions {
		if action.ID == id {
			return action.Label
		}
	}
	return id
}

// Paste routes native clipboard images through Herdr's image transport. Text
// falls through to the terminal plugin, including its bracketed paste handling.
func (w *Window) Paste(binding terminalhost.Binding, data transfer.Data) bool {
	var published session.PublishedSnapshot
	if w.terminal != nil && w.terminal.session != nil {
		published = w.terminal.session.Latest()
	}
	return w.pastePublished(binding, data, published)
}

func (w *Window) pastePublished(binding terminalhost.Binding, data transfer.Data, published session.PublishedSnapshot) bool {
	image, err := data.Read(transfer.PNG)
	if err != nil || len(image) == 0 {
		return false
	}
	panel := w.paneForBinding(binding)
	// Consume stale image pastes so they cannot reach a replacement terminal.
	if panel == nil || !bindingMatchesPublication(binding, panel.spec, published) {
		return true
	}
	if w.hooks.PasteImage == nil {
		return false
	}
	w.shortcutError(w.hooks.PasteImage(panel.spec.PaneID, image))
	return true
}

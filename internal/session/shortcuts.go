package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

// shortcutRequest is a closed mapping: shortcuts cannot issue arbitrary RPCs.
func shortcutRequest(op Operation) (string, map[string]any, string, error) {
	p := map[string]any{}
	method, response := "", "ok"
	switch op.Action {
	case "new_workspace":
		method, response = "workspace.create", "workspace_created"
		if op.WorkspaceID != "" {
			p["source_workspace_id"] = op.WorkspaceID
		}
		if op.Text != "" {
			p["cwd"] = op.Text
		}
	case "new_tab":
		method, response = "tab.create", "tab_created"
		p["workspace_id"] = op.WorkspaceID
	case "rename_workspace", "close_workspace":
		p["workspace_id"] = op.WorkspaceID
		if op.Action == "rename_workspace" {
			method, response = "workspace.rename", "workspace_info"
			p["label"] = op.Text
		} else {
			method = "workspace.close"
			p["close_group"] = true
		}
	case "rename_tab", "close_tab", "move_tab_previous", "move_tab_next":
		p["tab_id"] = op.TabID
		switch op.Action {
		case "rename_tab":
			method, response = "tab.rename", "tab_info"
			p["label"] = op.Text
		case "close_tab":
			method = "tab.close"
		default:
			method, response = "tab.move", "tab_list"
			p["insert_index"] = op.Index
		}
	case "select_pane":
		method, response = "pane.focus", "pane_info"
		p["pane_id"] = op.PaneID
	case "focus_pane_direction":
		if op.Direction != "left" && op.Direction != "right" && op.Direction != "up" && op.Direction != "down" {
			return "", nil, "", fmt.Errorf("invalid focus direction %q", op.Direction)
		}
		method, response = "pane.focus_direction", "pane_focus_direction"
		p["pane_id"], p["direction"] = op.PaneID, op.Direction
	case "rename_pane":
		method, response = "pane.rename", "pane_info"
		p["pane_id"], p["label"] = op.PaneID, op.Text
	case "swap_pane_left", "swap_pane_down", "swap_pane_up", "swap_pane_right":
		method, response = "pane.swap", "pane_swap"
		p["pane_id"], p["direction"] = op.PaneID, strings.TrimPrefix(op.Action, "swap_pane_")
	case "resize_pane_left", "resize_pane_down", "resize_pane_up", "resize_pane_right":
		method, response = "pane.resize", "pane_resize"
		p["pane_id"], p["direction"] = op.PaneID, strings.TrimPrefix(op.Action, "resize_pane_")
	case "edit_scrollback":
		method = "pane.edit_scrollback"
		p["pane_id"] = op.PaneID
	case "reload_config":
		method, response = "server.reload_config", "config_reload"
	case "new_worktree", "open_worktree":
		p["workspace_id"] = op.WorkspaceID
		if op.Action == "new_worktree" {
			method, response = "worktree.create", "worktree_created"
			p["branch"] = op.Text
		} else {
			method, response = "worktree.open", "worktree_opened"
			p["path"] = op.Text
		}
	case "remove_worktree":
		method, response = "worktree.remove", "worktree_removed"
		p["workspace_id"] = op.WorkspaceID
	case "read_scrollback":
		method, response = "pane.read", "pane_read"
		p["pane_id"], p["source"], p["lines"] = op.PaneID, "recent", 100000
	case "send_text":
		method = "pane.send_text"
		p["pane_id"], p["text"] = op.PaneID, op.Text
	case "invoke_command":
		method = "command.invoke"
		p["command_id"], p["workspace_id"], p["tab_id"], p["pane_id"] = op.CommandID, op.WorkspaceID, op.TabID, op.PaneID
	default:
		return "", nil, "", fmt.Errorf("unknown shortcut action %q", op.Action)
	}
	return method, p, response, nil
}

func (s *Session) executeShortcut(ctx context.Context, client *herdr.Client, q *queuedOperation) error {
	if q.operation.Action == "read_scrollback" {
		return s.readHistory(ctx, client, q)
	}
	method, params, expected, err := shortcutRequest(q.operation)
	if err != nil {
		q.complete(Rejected, err.Error(), ScrollInfo{})
		return nil
	}
	if q.operation.Action == "invoke_command" && q.operation.SelectedText != nil {
		selection, selectionErr := resolveCommandSelection(ctx, client, q.operation)
		if selectionErr != nil {
			q.complete(Rejected, selectionErr.Error(), ScrollInfo{})
			var remote *herdr.RemoteError
			var rejected selectionRejection
			if errors.As(selectionErr, &remote) || errors.As(selectionErr, &rejected) {
				return nil
			}
			return selectionErr
		}
		params["selection"] = selection
	}
	snapshot := s.Latest().Snapshot
	var data json.RawMessage
	if err := client.Call(ctx, method, params, &data); err != nil {
		var remote *herdr.RemoteError
		if errors.As(err, &remote) {
			q.complete(Rejected, err.Error(), ScrollInfo{})
			return nil
		}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	var envelope struct {
		Type     string `json:"type"`
		RootPane struct {
			ID string `json:"pane_id"`
		} `json:"root_pane"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Type != expected {
		err := &herdr.ProtocolError{Reason: method + " returned an invalid acknowledgement"}
		q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
		return err
	}
	paneID := envelope.RootPane.ID
	if q.operation.Action == "select_pane" || q.operation.Action == "focus_pane_direction" {
		paneID, err = shortcutFocusDestination(q.operation, data, snapshot)
		if err != nil {
			q.complete(OutcomeUnknown, err.Error(), ScrollInfo{})
			return err
		}
	}
	q.finish(OperationResult{Status: Succeeded, Data: data, PaneID: paneID})
	return nil
}

// Capture full history through selection extraction; pane.read intentionally
// caps recent output to 1,000 lines and cannot serve an editor/copy buffer.
func (s *Session) readHistory(ctx context.Context, client *herdr.Client, q *queuedOperation) error {
	op := q.operation
	fail := func(err error) error {
		q.complete(Rejected, err.Error(), ScrollInfo{})
		var remote *herdr.RemoteError
		if errors.As(err, &remote) {
			return nil
		}
		return err
	}
	var info struct {
		Pane struct {
			ID       string `json:"pane_id"`
			Terminal string `json:"terminal_id"`
			Scroll   *struct {
				Max  uint64 `json:"max_offset_from_bottom"`
				Rows uint64 `json:"viewport_rows"`
			} `json:"scroll"`
		} `json:"pane"`
	}
	if err := client.Call(ctx, "pane.get", map[string]any{"pane_id": op.PaneID}, &info); err != nil {
		return fail(err)
	}
	if info.Pane.ID != op.PaneID || info.Pane.Terminal != op.TerminalID || info.Pane.Scroll == nil {
		return fail(&herdr.ProtocolError{Reason: "history target or scroll metrics missing"})
	}
	point := func(row uint64, col uint16) map[string]any { return map[string]any{"row": row, "col": col} }
	last := info.Pane.Scroll.Max + info.Pane.Scroll.Rows
	if last > 0 {
		last--
	}
	if last > uint64(^uint32(0)) {
		return fail(fmt.Errorf("history exceeds addressable row range"))
	}
	var motion struct {
		PaneID   string `json:"pane_id"`
		Revision uint64 `json:"content_revision"`
		Cursor   struct {
			Row uint64 `json:"row"`
			Col uint16 `json:"col"`
		} `json:"cursor"`
	}
	if err := client.Call(ctx, "pane.copy_motion", map[string]any{"pane_id": op.PaneID, "cursor": point(last, 0), "motion": "line_end"}, &motion); err != nil {
		return fail(err)
	}
	if motion.PaneID != op.PaneID {
		return fail(&herdr.ProtocolError{Reason: "history revision target mismatch"})
	}

	var selection struct {
		PaneID string `json:"pane_id"`
		Text   string `json:"text"`
	}
	if err := client.Call(ctx, "pane.selection.read", map[string]any{"pane_id": op.PaneID, "anchor": point(0, 0), "cursor": point(motion.Cursor.Row, motion.Cursor.Col), "content_revision": motion.Revision}, &selection); err != nil {
		return fail(err)
	}
	if selection.PaneID != op.PaneID {
		return fail(&herdr.ProtocolError{Reason: "history selection target mismatch"})
	}
	data, err := json.Marshal(map[string]any{"type": "pane_read", "read": map[string]any{"pane_id": op.PaneID, "text": selection.Text, "revision": motion.Revision, "truncated": false}})
	if err != nil {
		return fail(err)
	}
	q.finish(OperationResult{Status: Succeeded, Data: data})
	return nil
}

// shortcutFocusDestination checks the reply against the immutable snapshot
// from which the request was issued, never a later selection or implicit focus.
func shortcutFocusDestination(op Operation, data []byte, snapshot *Snapshot) (string, error) {
	invalid := func() (string, error) {
		return "", &herdr.ProtocolError{Reason: "pane focus response identity mismatch"}
	}
	if snapshot == nil {
		return invalid()
	}
	source, ok := snapshot.Pane(op.PaneID)
	if !ok || source.TerminalID != op.TerminalID || source.TabID != op.TabID || source.WorkspaceID != op.WorkspaceID {
		return invalid()
	}
	if op.Action == "select_pane" {
		var response struct {
			Pane struct {
				ID        string `json:"pane_id"`
				Terminal  string `json:"terminal_id"`
				Workspace string `json:"workspace_id"`
				Tab       string `json:"tab_id"`
			} `json:"pane"`
		}
		if json.Unmarshal(data, &response) != nil {
			return invalid()
		}
		pane := response.Pane
		if pane.ID != op.PaneID || pane.Terminal != op.TerminalID || pane.Workspace != op.WorkspaceID || pane.Tab != op.TabID {
			return invalid()
		}
		return pane.ID, nil
	}
	var response struct {
		Focus struct {
			Changed     bool   `json:"changed"`
			Source      string `json:"source_pane_id"`
			Destination string `json:"focused_pane_id"`
			Layout      struct {
				Workspace string `json:"workspace_id"`
				Tab       string `json:"tab_id"`
				Focused   string `json:"focused_pane_id"`
			} `json:"layout"`
		} `json:"focus"`
	}
	if json.Unmarshal(data, &response) != nil {
		return invalid()
	}
	focus := response.Focus
	if focus.Source != op.PaneID || focus.Layout.Workspace != op.WorkspaceID || focus.Layout.Tab != op.TabID {
		return invalid()
	}
	if focus.Destination == "" {
		if focus.Changed {
			return invalid()
		}
		return "", nil
	}
	pane, ok := snapshot.Pane(focus.Destination)
	if !ok || pane.WorkspaceID != op.WorkspaceID || pane.TabID != op.TabID || focus.Layout.Focused != pane.ID {
		return invalid()
	}
	if !focus.Changed {
		return "", nil
	}
	return pane.ID, nil
}

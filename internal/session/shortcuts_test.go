package session

import (
	"context"
	"testing"
)

func TestShortcutRequestsKeepExplicitScope(t *testing.T) {
	for _, test := range []struct{ action, method, kind, target string }{
		{"new_workspace", "workspace.create", "workspace_created", "source_workspace_id"},
		{"new_tab", "tab.create", "tab_created", "workspace_id"},
		{"rename_workspace", "workspace.rename", "workspace_info", "workspace_id"},
		{"close_workspace", "workspace.close", "ok", "workspace_id"},
		{"rename_tab", "tab.rename", "tab_info", "tab_id"},
		{"move_tab_next", "tab.move", "tab_list", "tab_id"},
		{"close_tab", "tab.close", "ok", "tab_id"},
		{"rename_pane", "pane.rename", "pane_info", "pane_id"},
		{"swap_pane_left", "pane.swap", "pane_swap", "pane_id"},
		{"resize_pane_right", "pane.resize", "pane_resize", "pane_id"},
		{"new_worktree", "worktree.create", "worktree_created", "workspace_id"},
		{"open_worktree", "worktree.open", "worktree_opened", "workspace_id"},
		{"remove_worktree", "worktree.remove", "worktree_removed", "workspace_id"},
		{"clear_pane", "pane.clear", "ok", "pane_id"},
	} {
		t.Run(test.action, func(t *testing.T) {
			method, params, kind, err := shortcutRequest(Operation{Action: test.action, WorkspaceID: "workspace", TabID: "tab", PaneID: "pane", Text: "value", Index: 2})
			if err != nil || method != test.method || kind != test.kind {
				t.Fatal(method, kind, err)
			}
			expected := map[string]string{"workspace_id": "workspace", "source_workspace_id": "workspace", "tab_id": "tab", "pane_id": "pane"}[test.target]
			if params[test.target] != expected {
				t.Fatal(params)
			}
			if test.action == "move_tab_next" && params["insert_index"] != 2 {
				t.Fatal(params)
			}
			if test.action == "resize_pane_right" && params["direction"] != "right" {
				t.Fatal(params)
			}
			if test.action == "close_workspace" && params["close_group"] != true {
				t.Fatal(params)
			}
		})
	}
	if _, _, _, err := shortcutRequest(Operation{Action: "server.stop"}); err == nil {
		t.Fatal("unregistered method accepted")
	}
}

func TestSelectionRejectionDoesNotDisconnectSession(t *testing.T) {
	empty := ""
	q := &queuedOperation{operation: Operation{Kind: Shortcut, Action: "invoke_command", CommandID: "opaque", SelectedText: &empty}, result: make(chan OperationResult, 1)}
	if err := (&Session{}).executeShortcut(context.Background(), nil, q); err != nil {
		t.Fatalf("business rejection became transport failure: %v", err)
	}
	if result := <-q.result; result.Status != Rejected {
		t.Fatal(result)
	}
}

func TestMoveShortcutRequests(t *testing.T) {
	method, params, response, err := shortcutRequest(Operation{Action: "move_workspace", WorkspaceID: "w2", Index: 3})
	if err != nil || method != "workspace.move" || params["insert_index"] != 3 || params["workspace_id"] != "w2" || response != "" {
		t.Fatalf("%s %v %s %v", method, params, response, err)
	}
	if _, _, _, err := shortcutRequest(Operation{Action: "move_workspace", Index: -1}); err == nil {
		t.Fatal("accepted a negative position")
	}
	method, params, _, err = shortcutRequest(Operation{Action: "move_pane_to_tab", PaneID: "p", TabID: "t1", Text: "t2", Direction: "sideways"})
	destination, _ := params["destination"].(map[string]any)
	if err != nil || method != "pane.move" || destination["tab_id"] != "t2" || destination["split"] != "right" {
		t.Fatalf("%s %v %v", method, params, err)
	}
	if _, _, _, err := shortcutRequest(Operation{Action: "move_pane_to_tab", PaneID: "p", TabID: "t1", Text: "t1"}); err == nil {
		t.Fatal("accepted moving a pane to its own tab")
	}
	_, params, _, _ = shortcutRequest(Operation{Action: "move_pane_new_workspace", PaneID: "p"})
	if destination, _ := params["destination"].(map[string]any); destination["type"] != "new_workspace" {
		t.Fatalf("%v", params)
	}
}

package session

import (
	"encoding/json"
	"testing"
)

func focusSnapshot(t *testing.T) *Snapshot {
	t.Helper()
	s, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
		s["panes"] = append(s["panes"].([]any), map[string]any{"pane_id": "other", "terminal_id": "other-term", "workspace_id": "w", "tab_id": "t"})
	}))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestZoomFocusRequestsUseExplicitSource(t *testing.T) {
	for _, tt := range []struct{ action, method, kind string }{{"select_pane", "pane.focus", "pane_info"}, {"focus_pane_direction", "pane.focus_direction", "pane_focus_direction"}} {
		m, p, k, e := shortcutRequest(Operation{Action: tt.action, PaneID: "p", Direction: "left"})
		if e != nil || m != tt.method || k != tt.kind || p["pane_id"] != "p" {
			t.Fatal(m, p, k, e)
		}
	}
	if _, _, _, e := shortcutRequest(Operation{Action: "focus_pane_direction", Direction: "diagonal"}); e == nil {
		t.Fatal("invalid direction accepted")
	}
}
func TestFocusReplyIdentityAndSnapshotScope(t *testing.T) {
	s := focusSnapshot(t)
	op := Operation{Action: "select_pane", PaneID: "p", TerminalID: "term", WorkspaceID: "w", TabID: "t"}
	pane := map[string]any{"pane_id": "p", "terminal_id": "term", "workspace_id": "w", "tab_id": "t"}
	encode := func() []byte { b, _ := json.Marshal(map[string]any{"pane": pane}); return b }
	if id, e := shortcutFocusDestination(op, encode(), s); e != nil || id != "p" {
		t.Fatal(id, e)
	}
	for _, field := range []string{"pane_id", "terminal_id", "workspace_id", "tab_id"} {
		original := pane[field]
		pane[field] = "wrong"
		if _, e := shortcutFocusDestination(op, encode(), s); e == nil {
			t.Fatal("accepted mismatch", field)
		}
		pane[field] = original
	}
	op.Action = "focus_pane_direction"
	focus := map[string]any{"changed": true, "source_pane_id": "p", "focused_pane_id": "other", "layout": map[string]any{"workspace_id": "w", "tab_id": "t", "focused_pane_id": "other"}}
	reply := func() []byte { b, _ := json.Marshal(map[string]any{"focus": focus}); return b }
	if id, e := shortcutFocusDestination(op, reply(), s); e != nil || id != "other" {
		t.Fatal(id, e)
	}
	focus["focused_pane_id"] = "not-in-request-snapshot"
	if _, e := shortcutFocusDestination(op, reply(), s); e == nil {
		t.Fatal("accepted unknown destination")
	}
	focus["focused_pane_id"] = "other"
	focus["source_pane_id"] = "wrong"
	if _, e := shortcutFocusDestination(op, reply(), s); e == nil {
		t.Fatal("accepted wrong source")
	}
}

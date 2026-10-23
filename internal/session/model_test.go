package session

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func modelFixture(t *testing.T, change func(map[string]any)) []byte {
	t.Helper()
	var s map[string]any
	err := json.Unmarshal([]byte(`{"version":"1","protocol":1,"focused_workspace_id":"w","focused_tab_id":"t","focused_pane_id":"p","workspaces":[{"workspace_id":"w","number":1,"label":"Workspace","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t","number":1,"label":"Tab","focused":true}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","focused":true,"agent_status":"future_state"}],"layouts":[{"workspace_id":"w","tab_id":"t","area":{"x":0,"y":0,"width":80,"height":24},"focused_pane_id":"p","panes":[{"pane_id":"p","rect":{"x":0,"y":0,"width":80,"height":24}}],"splits":[]}],"agents":[]}`), &s)
	if err != nil {
		t.Fatal(err)
	}
	if change != nil {
		change(s)
	}
	data, err := json.Marshal(map[string]any{"type": "session_snapshot", "snapshot": s})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestSnapshotValuesAndImmutability(t *testing.T) {
	data := modelFixture(t, nil)
	s, err := DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	for i := range data {
		data[i] = 0
	}
	ws := s.Workspaces()
	ws[0].Label = "changed"
	ts := s.Tabs("w")
	ts[0].Label = "changed"
	ps := s.Panes("t")
	ps[0].Title = "changed"
	ids := s.PaneIDs()
	ids[0] = "changed"
	w, _ := s.Workspace("w")
	tab, _ := s.Tab("t")
	p, _ := s.Pane("p")
	if w.Label != "Workspace" || tab.Label != "Tab" || p.Title != "p" || !reflect.DeepEqual(s.PaneIDs(), []string{"p"}) {
		t.Fatalf("mutable snapshot: %#v %#v %#v", w, tab, p)
	}
	if w.CheckoutPath != "" || w.RepoRoot != "" || p.CWD != "" || p.Agent != "" || p.Status != "future_state" {
		t.Fatalf("invented or lost fields: %#v %#v", w, p)
	}
	if s.Metadata().FocusedPaneID != "p" {
		t.Fatal("lost focus")
	}
}

func TestSnapshotLayoutImmutabilityAndZoom(t *testing.T) {
	s, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
		l := s["layouts"].([]any)[0].(map[string]any)
		l["zoomed"] = true
		l["splits"] = []any{map[string]any{"id": "root", "direction": "right", "ratio": .5, "rect": map[string]any{"x": 0, "y": 0, "width": 80, "height": 24}}}
		s["panes"] = append(s["panes"].([]any), map[string]any{"pane_id": "hidden", "terminal_id": "hidden-term", "workspace_id": "w", "tab_id": "t"})
	}))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := s.Layout("t")
	if !ok || !l.Zoomed || l.WorkspaceID != "w" || l.TabID != "t" || l.FocusedPaneID != "p" || l.Area.W != 80 || len(l.Panes) != 1 || len(s.Panes("t")) != 2 || len(l.Splits) != 1 {
		t.Fatalf("layout lost: %+v", l)
	}
	l.Panes[0].PaneID = "changed"
	l.Splits[0].Ratio = .7
	l.Area.W = 1
	fresh, _ := s.Layout("t")
	if fresh.Panes[0].PaneID != "p" || fresh.Splits[0].Ratio != .5 || fresh.Area.W != 80 {
		t.Fatal("mutable layout", fresh)
	}
	if _, ok := s.Layout("missing"); ok {
		t.Fatal("invented layout")
	}
}
func TestSnapshotValidation(t *testing.T) {
	cases := map[string]func(map[string]any){
		"duplicate workspace": func(s map[string]any) { s["workspaces"] = append(s["workspaces"].([]any), s["workspaces"].([]any)[0]) },
		"duplicate tab":       func(s map[string]any) { s["tabs"] = append(s["tabs"].([]any), s["tabs"].([]any)[0]) },
		"duplicate pane":      func(s map[string]any) { s["panes"] = append(s["panes"].([]any), s["panes"].([]any)[0]) },
		"missing terminal":    func(s map[string]any) { delete(s["panes"].([]any)[0].(map[string]any), "terminal_id") },
		"unknown workspace":   func(s map[string]any) { s["tabs"].([]any)[0].(map[string]any)["workspace_id"] = "missing" },
		"unknown pane tab":    func(s map[string]any) { s["panes"].([]any)[0].(map[string]any)["tab_id"] = "missing" },
		"unknown focus":       func(s map[string]any) { s["focused_pane_id"] = "missing" },
		"missing rect":        func(s map[string]any) { delete(s["layouts"].([]any)[0].(map[string]any), "area") },
		"partial rect": func(s map[string]any) {
			delete(s["layouts"].([]any)[0].(map[string]any)["area"].(map[string]any), "width")
		},
		"missing layout panes": func(s map[string]any) { delete(s["layouts"].([]any)[0].(map[string]any), "panes") },
	}
	for _, field := range []string{"workspaces", "tabs", "panes", "layouts", "agents"} {
		field := field
		cases["missing "+field] = func(s map[string]any) { delete(s, field) }
		cases["null "+field] = func(s map[string]any) { s[field] = nil }
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSnapshot(modelFixture(t, change)); err == nil {
				t.Fatal("accepted invalid snapshot")
			}
		})
	}
}
func TestLayoutUnknownPaneReference(t *testing.T) {
	_, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
		s["layouts"].([]any)[0].(map[string]any)["panes"].([]any)[0].(map[string]any)["pane_id"] = "gone"
	}))
	var ref *ReferenceError
	if !errors.As(err, &ref) {
		t.Fatalf("want ReferenceError, got %v", err)
	}
}
func TestEmptySnapshotAndMissingLayout(t *testing.T) {
	if _, err := DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"1","protocol":1,"workspaces":[],"tabs":[],"panes":[],"layouts":[],"agents":[]}}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) { s["layouts"] = []any{} })); err != nil {
		t.Fatal(err)
	}
}
func TestAgentIdentityAndTitleFallback(t *testing.T) {
	data := modelFixture(t, func(s map[string]any) {
		s["panes"] = append(s["panes"].([]any), map[string]any{"pane_id": "p2", "terminal_id": "term2", "workspace_id": "w", "tab_id": "t", "terminal_title_stripped": "Terminal 2", "agent_status": "idle"})
		s["agents"] = []any{map[string]any{"pane_id": "p", "terminal_id": "term", "workspace_id": "w", "tab_id": "t", "name": "same", "agent": "codex", "agent_status": "working"}, map[string]any{"pane_id": "p2", "terminal_id": "term2", "workspace_id": "w", "tab_id": "t", "name": "same", "agent": "claude", "agent_status": "waiting"}}
	})
	s, err := DecodeSnapshot(data)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Pane("p")
	p2, _ := s.Pane("p2")
	if p.Agent != "codex" || p.Status != "working" || p2.Agent != "claude" || p2.Status != "waiting" || p2.Title != "Terminal 2" {
		t.Fatalf("incorrect agent association: %#v %#v", p, p2)
	}
	for _, field := range []string{"terminal_id", "workspace_id", "tab_id", "pane_id"} {
		t.Run(field, func(t *testing.T) {
			var result map[string]any
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			result["snapshot"].(map[string]any)["agents"].([]any)[0].(map[string]any)[field] = "stale"
			invalid, _ := json.Marshal(result)
			if _, err := DecodeSnapshot(invalid); err == nil {
				t.Fatal("accepted stale agent identity")
			}
		})
	}
}

func TestSnapshotOrderAndWorktreeMetadata(t *testing.T) {
	s, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
		s["workspaces"].([]any)[0].(map[string]any)["worktree"] = map[string]any{"repo_key": "repo", "repo_root": "/repo", "checkout_path": "/checkouts/topic", "is_linked_worktree": true}
		s["panes"] = append([]any{map[string]any{"pane_id": "z", "terminal_id": "term-z", "workspace_id": "w", "tab_id": "t"}}, s["panes"].([]any)...)
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.PaneIDs(), []string{"z", "p"}) || s.Panes("t")[0].ID != "z" {
		t.Fatal("lost authoritative order")
	}
	w, _ := s.Workspace("w")
	if w.CheckoutPath != "/checkouts/topic" || w.RepoRoot != "/repo" || w.RepoKey != "repo" || !w.Linked {
		t.Fatalf("lost worktree metadata: %#v", w)
	}
	if _, ok := s.Workspace("missing"); ok {
		t.Fatal("invented workspace")
	}
	if _, ok := s.Tab("missing"); ok {
		t.Fatal("invented tab")
	}
	if _, ok := s.Pane("missing"); ok {
		t.Fatal("invented pane")
	}
}

func TestPaneTitlePriority(t *testing.T) {
	fields := []string{"label", "title", "terminal_title_stripped", "terminal_title"}
	for i, want := range fields {
		t.Run(want, func(t *testing.T) {
			s, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
				p := s["panes"].([]any)[0].(map[string]any)
				for _, field := range fields[i:] {
					p[field] = field
				}
			}))
			if err != nil {
				t.Fatal(err)
			}
			p, _ := s.Pane("p")
			if p.Title != want {
				t.Fatalf("got %q, want %q", p.Title, want)
			}
		})
	}
}

func TestSnapshotRejectsIncoherentIdentity(t *testing.T) {
	cases := map[string]func(map[string]any){
		"duplicate terminal": func(s map[string]any) {
			s["panes"] = append(s["panes"].([]any), map[string]any{"pane_id": "other", "terminal_id": "term", "workspace_id": "w", "tab_id": "t"})
		},
		"active tab":         func(s map[string]any) { s["workspaces"].([]any)[0].(map[string]any)["active_tab_id"] = "missing" },
		"layout tab":         func(s map[string]any) { s["layouts"].([]any)[0].(map[string]any)["tab_id"] = "missing" },
		"layout workspace":   func(s map[string]any) { s["layouts"].([]any)[0].(map[string]any)["workspace_id"] = "missing" },
		"negative rectangle": func(s map[string]any) { s["layouts"].([]any)[0].(map[string]any)["area"].(map[string]any)["x"] = -1 },
		"duplicate layout":   func(s map[string]any) { s["layouts"] = append(s["layouts"].([]any), s["layouts"].([]any)[0]) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSnapshot(modelFixture(t, change)); err == nil {
				t.Fatal("accepted incoherent snapshot")
			}
		})
	}
}

func TestSnapshotFocusedPaneUsesTabLayout(t *testing.T) {
	s, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) {
		s["tabs"] = append(s["tabs"].([]any), map[string]any{"workspace_id": "w", "tab_id": "inactive", "number": 2, "label": "Inactive", "focused": false})
		s["panes"] = append(s["panes"].([]any), map[string]any{"pane_id": "other-first", "terminal_id": "other-term-first", "workspace_id": "w", "tab_id": "inactive", "focused": false}, map[string]any{"pane_id": "other-focused", "terminal_id": "other-term-focused", "workspace_id": "w", "tab_id": "inactive", "focused": false})
		s["layouts"] = append(s["layouts"].([]any), map[string]any{"workspace_id": "w", "tab_id": "inactive", "focused_pane_id": "other-focused", "area": map[string]any{"x": 0, "y": 0, "width": 80, "height": 24}, "panes": []any{map[string]any{"pane_id": "other-focused", "rect": map[string]any{"x": 0, "y": 0, "width": 80, "height": 24}}}, "splits": []any{}})
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.FocusedPane("inactive"); got != "other-focused" {
		t.Fatalf("inactive tab focus = %q, want other-focused", got)
	}
	if got := s.FocusedPane("t"); got != "p" {
		t.Fatalf("active tab focus = %q, want p", got)
	}
	if got := s.FocusedPane("missing"); got != "" {
		t.Fatalf("unknown tab focus = %q", got)
	}
	if s.Metadata().FocusedPaneID != "p" {
		t.Fatal("scoped focus changed global observation")
	}
	withoutLayout, err := DecodeSnapshot(modelFixture(t, func(s map[string]any) { s["layouts"] = []any{} }))
	if err != nil {
		t.Fatal(err)
	}
	if got := withoutLayout.FocusedPane("t"); got != "" {
		t.Fatalf("invented focus without layout: %q", got)
	}
}

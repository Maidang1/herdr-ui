package app

import (
	"testing"
	"time"
)

func TestNoticeTextUsesProjectAndTaskContext(t *testing.T) {
	s := snapshot(t, `[{"workspace_id":"w1","label":"herdr-ui"}]`, `[{"tab_id":"a","workspace_id":"w1","label":"2"}]`, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"codex","label":"Fix notifications","title":"3"}]`)
	for _, tt := range []struct{ kind, title string }{
		{"NeedsAttention", "codex is waiting for input"},
		{"Finished", "codex completed the task"},
	} {
		t.Run(tt.kind, func(t *testing.T) {
			n := Notice{Kind: tt.kind, Title: "codex finished", Body: "old-project · 1 · 2", WorkspaceID: "w1", TabID: "a", PaneID: "p1", Position: "top-left", Received: time.Unix(1, 0)}
			got := NoticeText(n, s, "codex")
			if got.Title != tt.title || got.Body != "herdr-ui · Fix notifications" {
				t.Fatalf("unhelpful notification: %#v", got)
			}
			n.Title, n.Body = got.Title, got.Body
			if got != n {
				t.Fatalf("formatting changed delivery/target metadata: %#v", got)
			}
		})
	}
}

func TestNoticeTextKeepsEventAgentWhenPaneOccupantChanges(t *testing.T) {
	s := snapshot(t, `[{"workspace_id":"w1","label":"herdr-ui"}]`, `[{"tab_id":"a","workspace_id":"w1"}]`, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"claude"}]`)
	got := NoticeText(Notice{Kind: "Finished", PaneID: "p1"}, s, "codex")
	if got.Title != "codex completed the task" {
		t.Fatalf("notification was attributed to the new pane occupant: %q", got.Title)
	}
}

func TestNoticeTextPreservesUsefulMessageAndNumericTaskDetails(t *testing.T) {
	s := snapshot(t, `[{"workspace_id":"w1","label":"herdr-ui"}]`, `[{"tab_id":"a","workspace_id":"w1"}]`, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","agent":"claude","title":"Fix issue 123"}]`)
	n := Notice{Kind: "NeedsAttention", Body: "Approve running the migration for issue 123.", PaneID: "p1"}
	got := NoticeText(n, s, "")
	if got.Title != "claude is waiting for input" || got.Body != "herdr-ui · Fix issue 123\nApprove running the migration for issue 123." {
		t.Fatalf("lost useful context or message: %#v", got)
	}
}

func TestNoticeTextOmitsDefaultPaneIdentifiersAndNumericLabels(t *testing.T) {
	for _, label := range []string{"", "2"} {
		s := snapshot(t, `[{"workspace_id":"w1","label":"herdr-ui"}]`, `[{"tab_id":"a","workspace_id":"w1","label":"3"}]`, `[{"pane_id":"p1","terminal_id":"t1","workspace_id":"w1","tab_id":"a","label":"`+label+`"}]`)
		got := NoticeText(Notice{Kind: "Finished", PaneID: "p1"}, s, "codex")
		if got.Body != "herdr-ui" {
			t.Fatalf("identifier leaked into message: %#v", got)
		}
	}
}

func TestNoticeTextFallbackRemovesLocationNumbers(t *testing.T) {
	for _, tt := range []struct{ body, want string }{
		{"herdr-ui · 1 · 2", "herdr-ui"},
		{"herdr-ui · 1 · Review tests", "herdr-ui · Review tests"},
		{"", ""},
	} {
		got := NoticeText(Notice{Kind: "Finished", Title: "codex finished", Body: tt.body}, nil, "codex")
		if got.Title != "codex completed the task" || got.Body != tt.want {
			t.Fatalf("fallback leaked location or lost context: %#v", got)
		}
	}
	got := NoticeText(Notice{Kind: "NeedsAttention"}, nil, "")
	if got.Title != "Agent is waiting for input" {
		t.Fatal(got)
	}
}

func TestNoticeTextPreservesCustomAndUpdateMessages(t *testing.T) {
	for _, kind := range []string{"Custom", "UpdateInstalled", "Unknown"} {
		n := Notice{Kind: kind, Title: "Build 42 is ready", Body: "Window 2 · Keep this user message"}
		if got := NoticeText(n, nil, ""); got != n {
			t.Fatalf("changed user/server message: %#v", got)
		}
	}
}

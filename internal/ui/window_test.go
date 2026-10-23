package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestWindowReadsRealStateAndLocalSelection(t *testing.T) {
	s, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Real Workspace","active_tab_id":"t"}],"tabs":[{"workspace_id":"w","tab_id":"t","number":1,"label":"Dev"}],"panes":[{"pane_id":"p","terminal_id":"term","workspace_id":"w","tab_id":"t","cwd":"/real/project","label":"Shell","agent":"codex","agent_status":"blocked"}],"layouts":[],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: s, Revision: 1})
	w := New(a, func() {})
	tt := native.NewTester(w.View, 1200, 800)
	if !tt.HasText("Real Workspace") || !tt.HasText("/real/project") || !tt.HasText("Shell") {
		t.Fatal(tt.Texts())
	}
	if _, ok := tt.Find("Current tab"); ok {
		t.Fatal("removed information header remains visible")
	}
	if _, ok := tt.Find("Terminal layout"); ok {
		t.Fatal("removed header layout toolbar remains visible")
	}
	if dir := os.Getenv("HERDR_UI_RENDER_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			tt.Frame()
			name := "light.png"
			if dark {
				name = "dark.png"
			}
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, size := range [][2]int{{820, 560}, {1200, 800}} {
		tt.SetSize(size[0], size[1])
		tt.SetDark(true)
		tt.Frame()
		if _, ok := tt.Find("Shell"); !ok {
			t.Fatal("pane disappeared at small width")
		}
	}
	// Unknown states retain their real value in the accessible description.
	_, _, label := status("future-state", false)
	if !strings.Contains(label, "future-state") {
		t.Fatal(label)
	}
}

func TestWindowRetryAndEmptyStates(t *testing.T) {
	a := app.New()
	a.Update(session.PublishedSnapshot{State: session.Reconnecting, Stale: true, Error: "test disconnect"})
	retried := false
	w := New(a, func() { retried = true })
	tt := native.NewTester(w.View, 820, 560)
	if !tt.HasText("test disconnect") {
		t.Fatal(tt.Texts())
	}
	if err := tt.Click("Retry connection"); err != nil {
		t.Fatal(err)
	}
	if !retried {
		t.Fatal("Retry did not dispatch")
	}
}

func TestProjectionCacheFollowsLocalWorkspaceSelection(t *testing.T) {
	s, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"a","label":"One"},{"workspace_id":"b","label":"Two"}],"tabs":[{"workspace_id":"a","tab_id":"ta"},{"workspace_id":"b","tab_id":"tb"}],"panes":[{"pane_id":"pa","terminal_id":"a","workspace_id":"a","tab_id":"ta","title":"First pane"},{"pane_id":"pb","terminal_id":"b","workspace_id":"b","tab_id":"tb","title":"Second pane"}],"agents":[],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: s})
	w := New(a, nil)
	tt := native.NewTester(w.View, 1200, 800)
	if tt.HasText("Second pane") {
		t.Fatal("unselected workspace rendered pane")
	}
	if err := tt.Click("Two · Unknown"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("Second pane") {
		t.Fatal("local selection did not invalidate projection")
	}
	before := a.Version()
	tt.Frame()
	if a.Version() != before {
		t.Fatal("paint mutated local selection")
	}
}

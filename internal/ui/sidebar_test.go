package ui

import (
	"encoding/json"
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/Maidang1/herdr-ui/internal/sidebarinfo"
	native "github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func workspaceSidebarFixture(t *testing.T) *Window {
	t.Helper()
	snap, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w","label":"Repo","agent_status":"idle"},{"workspace_id":"other","label":"Shell only"}],"tabs":[{"workspace_id":"w","tab_id":"t"},{"workspace_id":"other","tab_id":"ot"}],"panes":[{"pane_id":"shell","terminal_id":"tshell","workspace_id":"w","tab_id":"t","title":"user@HOST:/repo","cwd":"/repo"},{"pane_id":"agent","terminal_id":"tagent","workspace_id":"w","tab_id":"t","title":"Codex terminal prompt","agent":"codex","agent_status":"idle"},{"pane_id":"other-shell","terminal_id":"tother","workspace_id":"other","tab_id":"ot","title":"Other terminal prompt","cwd":"/other"}],"layouts":[],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	published := session.PublishedSnapshot{TargetID: 1, State: session.Ready, Snapshot: snap}
	a := app.New()
	a.Update(published)
	source := sidebarinfo.BuildSource(published)
	a.UpdateSidebar(sidebarinfo.Publication{TargetID: 1, Entries: map[string]sidebarinfo.Entry{"w": {Fingerprint: sidebarinfo.Fingerprint(source.Workspaces[0]), Info: sidebarinfo.Info{Path: "/repo", Branch: "feat/sidebar", GitKnown: true, Remote: "github", Ahead: 13, Behind: 11, PortsKnown: true, Ports: []int{8080, 8081, 9200}}}}})
	return New(a, nil)
}
func TestSidebarShowsSpacesDetailsAndOnlyActualAgents(t *testing.T) {
	w := workspaceSidebarFixture(t)
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	for _, text := range []string{"Spaces", "Repo", "feat/sidebar", "↑13", "↓11", ":8080 :8081 :9200", "Agents", "codex", "Idle"} {
		if !tester.HasText(text) {
			t.Fatalf("missing sidebar info %q: %v", text, tester.Texts())
		}
	}
	for _, text := range []string{"user@HOST:/repo", "Codex terminal prompt", "Other terminal prompt", "/repo"} {
		if tester.HasText(text) {
			t.Fatal("terminal title/path leaked into workspace sidebar", text)
		}
	}
	if err := tester.Click("Repo · codex · Idle · agent"); err != nil {
		t.Fatal(err)
	}
	if w.app.View().PaneID != "agent" {
		t.Fatal("agent click did not select its exact pane")
	}
}

func TestSidebarFilterNarrowsSpacesAndAgents(t *testing.T) {
	w := workspaceSidebarFixture(t)
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if err := tester.Click("Filter spaces"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Type("repo")
	tester.Frame()
	if tester.HasText("Shell only") {
		t.Fatal("filter left non-matching workspace visible")
	}
	for _, text := range []string{"Repo", "codex", "Clear filter"} {
		if !tester.HasText(text) {
			t.Fatalf("missing filtered sidebar info %q: %v", text, tester.Texts())
		}
	}
	tester.Key(0, native.KeyEscape)
	tester.Frame()
	if !tester.HasText("Shell only") {
		t.Fatal("escape did not leave the filter field")
	}
	if w.sidebarFilterOn {
		t.Fatal("escape did not deactivate the filter")
	}
}

func TestSidebarAgentSortToggleFlattensGroups(t *testing.T) {
	w := workspaceSidebarFixture(t)
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if err := tester.Click("Sort agents: grouped"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if tester.HasText("Agents in Repo") {
		t.Fatal("priority mode still groups agents under workspaces")
	}
	if !tester.HasText("Sort agents: priority") {
		t.Fatal("toggle did not report the priority mode")
	}
	if err := tester.Click("Sort agents: priority"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !tester.HasText("Agents in Repo") {
		t.Fatal("grouped mode lost its workspace group")
	}
}

func TestSidebarUpdatePromptReplacesTheConnectionStatus(t *testing.T) {
	w := workspaceSidebarFixture(t)
	checks := 0
	w.SetActionHooks(ActionHooks{UpdateNotice: func() UpdateNotice { return UpdateNotice{Available: "0.0.8"} }, CheckForUpdates: func() { checks++ }})
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if tester.HasText("Local") {
		t.Fatal("connection status still occupies the sidebar footer")
	}
	if err := tester.Click("Update to 0.0.8"); err != nil {
		t.Fatal(err)
	}
	if checks != 1 {
		t.Fatalf("update prompt checked %d times", checks)
	}
	w.SetActionHooks(ActionHooks{})
	tester.Frame()
	if tester.HasText("Update to 0.0.8") {
		t.Fatal("an up-to-date app still prompts")
	}
}

func TestSidebarMenuOpensWindowCommands(t *testing.T) {
	w := workspaceSidebarFixture(t)
	tester := native.NewTester(func(c *native.Context) { w.sidebar(c, w.currentView(), design.Colors(false)) }, 248, 800)
	if !tester.HasText("Settings") {
		t.Fatal("footer settings shortcut is missing")
	}
	for _, text := range []string{"Reload configuration", "Detach window"} {
		if tester.HasText(text) {
			t.Fatal("menu content visible while closed", text)
		}
	}
	if err := tester.Click("Sidebar menu"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	for _, text := range []string{"Settings", "Keyboard reference", "Reload configuration", "Detach window"} {
		if !tester.HasText(text) {
			t.Fatalf("menu missing item %q: %v", text, tester.Texts())
		}
	}
}

func TestSidebarRenderArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export sidebar renders")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		w := workspaceSidebarFixture(t)
		tester := native.NewTester(w.View, 1200, 800)
		tester.SetDark(dark)
		tester.Frame()
		file, err := os.Create(filepath.Join(dir, fmt.Sprintf("sidebar-dark-%t.png", dark)))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tester.Image())
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// An opt-in local render for comparing real read-only snapshot/metadata files.
// It creates no terminal host and makes no service calls.
func TestSidebarLiveRenderArtifact(t *testing.T) {
	snapshotFile, metadataFile := os.Getenv("HERDR_UI_SIDEBAR_SNAPSHOT"), os.Getenv("HERDR_UI_SIDEBAR_METADATA")
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if snapshotFile == "" || metadataFile == "" || dir == "" {
		t.Skip("set local snapshot, metadata and render paths")
	}
	snapshotData, err := os.ReadFile(snapshotFile)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(snapshotData, &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Result) > 0 {
		snapshotData = envelope.Result
	}
	var probe struct {
		Type     string          `json:"type"`
		Snapshot json.RawMessage `json:"snapshot"`
	}
	if err := json.Unmarshal(snapshotData, &probe); err != nil {
		t.Fatal(err)
	}
	if probe.Type == "" {
		if len(probe.Snapshot) > 0 {
			snapshotData = probe.Snapshot
		}
		snapshotData, err = json.Marshal(map[string]any{"type": "session_snapshot", "snapshot": json.RawMessage(snapshotData)})
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := session.DecodeSnapshot(snapshotData)
	if err != nil {
		t.Fatal(err)
	}
	metadataData, err := os.ReadFile(metadataFile)
	if err != nil {
		t.Fatal(err)
	}
	var metadata sidebarinfo.Publication
	if err := json.Unmarshal(metadataData, &metadata); err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{TargetID: metadata.TargetID, Snapshot: snapshot, State: session.Ready})
	a.UpdateSidebar(metadata)
	w := New(a, nil)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		tester := native.NewTester(func(c *native.Context) {
			p := w.appearancePalette(c)
			root := native.Row(c).Fill().Padding(8).AlignItems(native.Stretch)
			w.styleShell(c, root, p)
			root.Children(func() { w.sidebar(c, w.currentView(), p) })
		}, 264, 800)
		tester.SetDark(dark)
		tester.Frame()
		file, err := os.Create(filepath.Join(dir, fmt.Sprintf("live-sidebar-dark-%t.png", dark)))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, tester.Image())
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

package ui

import (
	"encoding/json"
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/app"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func TestOutputReminderCancelsOnEpochChange(t *testing.T) {
	w := populatedChromeWindow(t)
	called := 0
	v := w.currentView()
	w.outputWatch.op = session.Operation{TargetID: v.TargetID, Epoch: v.Epoch + 1, PaneID: v.PaneID}
	w.outputWatch.cancel = func() { called++ }
	w.SyncFeatureTargets()
	w.SyncFeatureTargets()
	if called != 1 || w.outputWatch.cancel != nil {
		t.Fatalf("cancel=%d", called)
	}
}
func TestDetailsLateCompletionIsDiscarded(t *testing.T) {
	w := populatedChromeWindow(t)
	var complete func(herdr.PaneDetails, error)
	w.hooks.PaneDetails = func(_ session.Operation, done func(herdr.PaneDetails, error)) { complete = done }
	w.details.open = true
	w.details.op = session.Operation{PaneID: "pane", TerminalID: "terminal"}
	w.loadDetails()
	if complete == nil {
		t.Fatal("details did not request data")
	}
	w.details.generation++
	complete(herdr.PaneDetails{Agent: herdr.AgentDetails{Name: "late"}}, nil)
	if w.details.data.Agent.Name != "" {
		t.Fatal("late response installed")
	}
}
func TestOutputReminderDeliversOnceWithoutNavigation(t *testing.T) {
	w := populatedChromeWindow(t)
	before := w.currentView()
	w.settings.snapshot.Toast.Delivery = app.DeliverySystem
	count := 0
	w.hooks.PostNotification = func(n app.Notice) { count++ }
	w.deliverOutputNotice(app.Notice{Kind: "OutputMatched", Title: "Output matched", PaneID: before.PaneID, TabID: before.TabID, Received: time.Now()})
	w.PumpNotifications()
	after := w.currentView()
	if count != 1 {
		t.Fatalf("notification count %d", count)
	}
	if before.PaneID != after.PaneID || before.TabID != after.TabID {
		t.Fatal("notification changed selection")
	}
}
func TestDetailsReminderKeepsDisplayedTarget(t *testing.T) {
	w := populatedChromeWindow(t)
	v := w.currentView()
	op := session.Operation{TargetID: v.TargetID, Epoch: v.Epoch, WorkspaceID: "w", TabID: "t", PaneID: "p", TerminalID: "term"}
	w.details.open, w.details.op = true, op
	w.app.SelectTab("other")
	var watched session.Operation
	w.hooks.WatchOutput = func(target session.Operation, _ herdr.OutputWatch, _ func(herdr.OutputMatch, error)) func() {
		watched = target
		return func() {}
	}
	tester := native.NewTester(func(c *native.Context) { w.outputWatchControls(c, w.appearancePalette(c)) }, 700, 500)
	if err := tester.Click("Watch output…"); err != nil {
		t.Fatal(err)
	}
	if w.controller.shortcut.prompt == nil {
		t.Fatal("missing watch prompt")
	}
	w.controller.shortcut.prompt.submit("ready")
	if watched.PaneID != "p" || watched.TabID != "t" {
		t.Fatalf("watched selected pane instead of displayed pane: %+v", watched)
	}
}

func TestRenderFeatureArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR")
	}
	os.MkdirAll(dir, 0755)
	for _, dark := range []bool{false, true} {
		for _, page := range []int{6, 7, 8, -1} {
			w := populatedChromeWindow(t)
			w.settings.page = page
			w.settings.plugins = pluginSettings{loaded: true, fresh: true, catalog: herdr.PluginCatalog{Plugins: []herdr.Plugin{{ID: "git-review", Name: "Git review", Version: "1.2.0", Enabled: true, Description: "Review repository changes", Warnings: []string{"Example warning"}}}, Actions: []herdr.PluginAction{{PluginID: "git-review", ID: "review", Title: "Review changes"}}, Logs: []herdr.PluginLog{{PluginID: "git-review", Status: "failed", Command: []string{"review", "--working-tree"}, Stderr: "Working directory unavailable"}}}}
			w.settings.restore = restoreSettingsUI{loaded: true, value: herdr.RestoreSettings{ResumeAgents: true}}
			w.settings.machines = machineSettings{loaded: true, catalog: herdr.MachineCatalog{Machines: []herdr.Machine{{ID: "dev", Label: "Development", Target: "user@host", Session: "work", Enabled: true}}, Status: []herdr.MachineStatus{{ID: "dev", Status: "reachable"}}, Sessions: []herdr.LocalSession{{Name: "default", Default: true, Running: true}}}}
			render := w.SettingsView
			width, height := 720, 520
			if page < 0 {
				render = w.View
				width, height = 1000, 667
				w.details.open = true
				w.details.data = herdr.PaneDetails{Agent: herdr.AgentDetails{Agent: "codex", Name: "Repository review", Status: "idle", CWD: "/workspace/project", Session: &herdr.AgentSession{Agent: "codex", Kind: "session_id", Value: "available-reference"}}, Process: herdr.ProcessDetails{TTY: "/dev/ttys004", Processes: []herdr.ForegroundProcess{{PID: 1234, Name: "codex", Command: "codex --resume available-reference", CWD: "/workspace/project"}}}, Explain: json.RawMessage(`{"source":"process","interactive_ready":true,"status":"idle"}`)}
			}
			tester := native.NewTester(render, width, height)
			tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
			tester.SetDark(dark)
			tester.Frame()
			tester.Frame()
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("features-page-%d-dark-%t.png", page, dark)))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tester.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

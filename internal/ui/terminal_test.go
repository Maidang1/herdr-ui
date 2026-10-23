package ui

import (
	"context"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/herdr"
	"github.com/bytedance/herdr-ui/internal/session"
	"github.com/bytedance/herdr-ui/internal/terminalhost"
	native "github.com/egoist/mygo/ui"
	"testing"
)

func TestNewBindingDropsAbandonedScrollOffset(t *testing.T) {
	panel := &terminalPanel{}
	binding := terminalhost.Binding{TargetID: 1, TerminalID: "t", Generation: 1}
	spec := terminalhost.Spec{TargetID: 1, TerminalID: "t", PaneID: "p"}
	panel.install(binding, spec)
	published := session.PublishedSnapshot{TargetID: 1, Epoch: 1, State: session.Ready}
	pane := session.Pane{ID: "p", TerminalID: "t", Scroll: session.ScrollInfo{MaxOffset: 100, Known: true}}
	panel.tracker.Wheel(published, pane, -3)
	binding.Generation = 2
	panel.install(binding, spec)
	op, _, ok := panel.tracker.Wheel(published, pane, -3)
	if !ok || op.Offset != 3 {
		t.Fatalf("replaced binding retained abandoned scroll: %+v", op)
	}
}

func TestTerminalPanelKeepsNoWorkspacesState(t *testing.T) {
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[],"tabs":[],"panes":[],"agents":[],"layouts":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{State: session.Ready, Snapshot: snapshot})
	state := session.New(herdr.Target{})
	defer state.Close(context.Background())
	host := terminalhost.New(terminalhost.Config{Post: func(fn func()) { fn() }})
	defer host.Close(context.Background())
	w := New(a, nil)
	w.ConnectTerminal(host, state, context.Background(), func(fn func()) { fn() })
	tt := native.NewTester(w.View, 820, 560)
	if !tt.HasText("No workspaces") || !tt.HasText("Press the prefix then Shift+N to create a workspace.") {
		t.Fatal(tt.Texts())
	}
}

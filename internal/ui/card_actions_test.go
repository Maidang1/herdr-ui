package ui

import (
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
	"testing"
)

func TestCardOperationTargetsClickedCardAndRejectsReplacement(t *testing.T) {
	published := keyPublication(t, "terminal")
	frame := app.View{TargetID: 1, Epoch: 1, WorkspaceID: "w", TabID: "t", PaneID: "another-focused-pane"}
	clicked := app.PaneRow{ID: "pane", TerminalID: "terminal"}
	for _, action := range []string{"split_vertical", "split_horizontal", "zoom", "close_pane"} {
		op, ok := cardOperation(action, clicked, frame, published)
		if !ok || op.PaneID != "pane" || op.TerminalID != "terminal" {
			t.Fatal("card action used focused pane", action, op)
		}
		if _, ok := cardOperation(action, clicked, frame, keyPublication(t, "replacement")); ok {
			t.Fatal("stale card acted on replacement", action)
		}
	}
	frame.Epoch++
	if _, ok := cardOperation("close_pane", clicked, frame, published); ok {
		t.Fatal("old epoch card action accepted")
	}
}
func TestCardControlsRemainVisibleWhenZoomedToOnePane(t *testing.T) {
	w := New(app.New(), nil)
	frame := app.View{Layout: session.PaneLayout{Zoomed: true}}
	tester := native.NewTester(func(c *native.Context) {
		native.Row(c).FillWidth().Children(func() { w.paneToolbarControls(c, app.PaneRow{ID: "pane"}, frame, design.Colors(false)) })
	}, 300, 200)
	for _, label := range []string{"Split right", "Split down", "Restore pane", "Close pane"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatal("missing card control", label)
		}
	}
}

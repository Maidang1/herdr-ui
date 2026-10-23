package ui

import (
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/session"
	native "github.com/egoist/mygo/ui"
)

func focusPublication(t *testing.T, revision uint64, focused string, zoom bool) session.PublishedSnapshot {
	t.Helper()
	s, err := session.DecodeSnapshot([]byte(fmt.Sprintf(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"focused_workspace_id":"w","focused_tab_id":"t","focused_pane_id":%q,"workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"p1","terminal_id":"term1","workspace_id":"w","tab_id":"t"},{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"t"}],"layouts":[{"workspace_id":"w","tab_id":"t","zoomed":%t,"focused_pane_id":%q,"area":{"x":0,"y":0,"width":80,"height":24},"panes":[{"pane_id":"p1","rect":{"x":0,"y":0,"width":40,"height":24}},{"pane_id":"p2","rect":{"x":40,"y":0,"width":40,"height":24}}],"splits":[]}],"agents":[]}}`, focused, zoom, focused)))
	if err != nil {
		t.Fatal(err)
	}
	return session.PublishedSnapshot{TargetID: 7, Epoch: 3, Revision: revision, State: session.Ready, Snapshot: s}
}
func focusWindow(t *testing.T, zoom bool) *Window {
	a := app.New()
	a.Update(focusPublication(t, 10, "p1", zoom))
	a.SelectPane("p1")
	return &Window{app: a, controller: &actionController{}}
}
func TestCommandFocusUsesOriginalIntentAndNewPublication(t *testing.T) {
	w := focusWindow(t, false)
	rev := w.app.ViewRevision()
	w.FollowCommandFocus(7, 3, rev, 10, "p1")
	// An already-published server focus is not evidence of the command's result.
	p := focusPublication(t, 10, "p2", false)
	w.app.Update(p)
	w.applyShortcutFocus(p, time.Now())
	if w.currentView().PaneID != "p1" || w.controller.shortcut.followCommand == nil {
		t.Fatal("old publication followed")
	}
	p = focusPublication(t, 11, "p2", false)
	w.app.Update(p)
	w.applyShortcutFocus(p, time.Now())
	if w.currentView().PaneID != "p2" || w.controller.shortcut.followCommand != nil {
		t.Fatal("new command focus not applied")
	}
}
func TestCommandFocusNeverOverridesNewNativeIntent(t *testing.T) {
	w := focusWindow(t, false)
	rev := w.app.ViewRevision()
	w.app.SelectPane("p2")
	w.FollowCommandFocus(7, 3, rev, 10, "p1")
	if w.controller.shortcut.followCommand != nil {
		t.Fatal("late completion captured a new intent")
	}
	w.FollowCommandFocus(7, 3, w.app.ViewRevision(), 10, "p2")
	w.app.SelectPane("p1")
	p := focusPublication(t, 11, "p2", false)
	w.app.Update(p)
	w.applyShortcutFocus(p, time.Now())
	if w.currentView().PaneID != "p1" || w.controller.shortcut.followCommand != nil {
		t.Fatal("pending command overrode newer selection")
	}
}
func TestCommandFocusExpiresAndRejectsSessionChange(t *testing.T) {
	for _, which := range []string{"expire", "target", "epoch"} {
		t.Run(which, func(t *testing.T) {
			w := focusWindow(t, false)
			w.FollowCommandFocus(7, 3, w.app.ViewRevision(), 10, "p1")
			p := focusPublication(t, 11, "p2", false)
			now := time.Now()
			switch which {
			case "expire":
				now = now.Add(6 * time.Second)
			case "target":
				p.TargetID++
			case "epoch":
				p.Epoch++
			}
			w.applyShortcutFocus(p, now)
			if w.controller.shortcut.followCommand != nil {
				t.Fatal("stale follower retained")
			}
		})
	}
}
func TestZoomedFocusWaitsForAuthoritativeVisiblePane(t *testing.T) {
	w := focusWindow(t, true)
	a := &w.controller.shortcut
	a.pendingPane = "p2"
	a.pendingTarget = 7
	a.pendingEpoch = 3
	a.pendingRevision = w.app.ViewRevision()
	a.pendingPublication = 10
	a.pendingUntil = time.Now().Add(time.Second)
	p := focusPublication(t, 11, "p1", true)
	w.app.Update(p)
	w.applyShortcutFocus(p, time.Now())
	if w.currentView().PaneID != "p1" || a.pendingPane != "p2" {
		t.Fatal("ack selected hidden zoom pane")
	}
	p = focusPublication(t, 12, "p2", true)
	w.app.Update(p)
	w.applyShortcutFocus(p, time.Now())
	if w.currentView().PaneID != "p2" || a.pendingPane != "" {
		t.Fatal("visible zoom target not selected")
	}
	if w.app.LastPaneID() != "p1" {
		t.Fatal("zoom focus lost last-pane history", w.app.LastPaneID())
	}
}
func TestShortcutSelectionStaysLocalUnlessZoomHidesDestination(t *testing.T) {
	w := focusWindow(t, false)
	w.selectShortcutPane("p2")
	if w.currentView().PaneID != "p2" {
		t.Fatal("normal shortcut selection failed")
	}
	w = focusWindow(t, true)
	w.selectShortcutPane("p2")
	if w.currentView().PaneID != "p1" {
		t.Fatal("hidden zoom pane selected without server acknowledgement")
	}
}
func TestCopyCaptureRetiresOnPaneChange(t *testing.T) {
	w := focusWindow(t, false)
	a := &w.controller.shortcut
	a.copy = newCopyState("captured")
	a.copyPane = "p1"
	a.copyTerminal = "term1"
	a.copyTarget = 7
	a.copyEpoch = 3
	w.app.SelectPane("p2")
	w.syncInput()
	if a.copy != nil {
		t.Fatal("copy capture followed unrelated pane")
	}
}

func TestCancelCopySearchRestoresKeyboardFocus(t *testing.T) {
	for _, escape := range []bool{false, true} {
		w := navigationWindow(t)
		a := &w.controller.shortcut
		a.copy = newCopyState("first\nsecond")
		a.copy.focus = false
		a.prompt = &shortcutPrompt{title: "Search history", focus: true}
		tester := native.NewTester(w.View, 1000, 800)
		if escape {
			tester.Key(0, native.KeyEscape)
		} else if err := tester.Click("Cancel"); err != nil {
			t.Fatal(err)
		}
		if a.prompt != nil {
			t.Fatal("prompt not canceled")
		}
		tester.Key(0, native.KeyK)
		if a.copy.row != 0 {
			t.Fatal("copy keyboard focus not restored", escape, a.copy.row)
		}
	}
}

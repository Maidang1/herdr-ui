package ui

import (
	"context"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/layout"
	"github.com/Maidang1/herdr-ui/internal/session"
	"github.com/Maidang1/herdr-ui/internal/terminalhost"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// findWindow assembles a window over a split view of two panes, the first
// focused, with a terminal host whose panes render without a live session.
func findWindow(t *testing.T) *Window {
	t.Helper()
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"focused_workspace_id":"w","focused_tab_id":"t","focused_pane_id":"p1","workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"p1","terminal_id":"term1","workspace_id":"w","tab_id":"t","title":"Shell"},{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"t","title":"Other"}],"layouts":[{"workspace_id":"w","tab_id":"t","focused_pane_id":"p1","area":{"x":0,"y":0,"width":80,"height":24},"panes":[{"pane_id":"p1","rect":{"x":0,"y":0,"width":40,"height":24}},{"pane_id":"p2","rect":{"x":40,"y":0,"width":40,"height":24}}],"splits":[]}],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	a := app.New()
	a.Update(session.PublishedSnapshot{TargetID: 7, Epoch: 3, State: session.Ready, Snapshot: snapshot})
	a.SelectPane("p1")
	host := terminalhost.New(terminalhost.Config{Binary: "/usr/bin/true", Target: herdr.Target{Socket: "/tmp/herdr-ui-find-test.sock"}, Post: func(fn func()) { fn() }})
	t.Cleanup(func() { _ = host.Close(context.Background()) })
	w := New(a, nil)
	bindings, err := input.Resolve(nil, "test")
	if err != nil {
		t.Fatal(err)
	}
	w.ConfigureBindings(bindings, nil)
	state := session.New(herdr.Target{})
	t.Cleanup(func() { _ = state.Close(context.Background()) })
	w.ConnectTerminal(host, state, context.Background(), func(fn func()) { fn() })
	// A session the test cannot drive leaves the panes installed by hand;
	// their specs match the view, so the panel keeps them.
	w.terminal.panes = map[string]*terminalPanel{
		"p1": {host: host, session: state, binding: terminalhost.Binding{TargetID: 7, TerminalID: "term1", Generation: 1}, spec: terminalhost.Spec{TargetID: 7, TerminalID: "term1", PaneID: "p1"}},
		"p2": {host: host, session: state, binding: terminalhost.Binding{TargetID: 7, TerminalID: "term2", Generation: 2}, spec: terminalhost.Spec{TargetID: 7, TerminalID: "term2", PaneID: "p2"}},
	}
	return w
}

func TestFindBarLifecycle(t *testing.T) {
	w := findWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	tester.Frame()
	if !w.Can("find") {
		t.Fatal("find unavailable with a connected terminal")
	}
	w.Dispatch("find")
	tester.Frame()
	if !w.findOpen() || w.find.pane != "p1" {
		t.Fatal("find did not open over the focused pane")
	}
	if _, ok := tester.Find("Find in terminal"); !ok {
		t.Fatal("missing find bar")
	}
	if !tester.Focused("Find in terminal") {
		t.Fatal("find field did not take the keyboard")
	}
	tester.Type("error")
	tester.Frame()
	if w.find.query != "error" {
		t.Fatal("typed text missed the find field")
	}
	w.SetSearchStatus(w.find.binding, terminal.SearchStatus{Text: "error", Total: 3, Selected: 1})
	tester.Frame()
	if !tester.HasText("1/3") {
		t.Fatal("match count missing")
	}
	if err := tester.Click("Find next"); err != nil {
		t.Fatal(err)
	}
	if !w.findOpen() {
		t.Fatal("stepping closed the bar")
	}
	tester.Key(0, native.KeyEscape)
	tester.Frame()
	if w.findOpen() {
		t.Fatal("Escape left the bar open")
	}
	if _, ok := tester.Find("Find in terminal"); ok {
		t.Fatal("closed bar still rendered")
	}
	// The bar reopens with the query last searched over the pane.
	w.Dispatch("find_next")
	tester.Frame()
	if !w.findOpen() || w.find.query != "error" {
		t.Fatal("reopening lost the last query")
	}
}

func TestFindCountFollowsOnlyItsPane(t *testing.T) {
	w := findWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	tester.Frame()
	w.Dispatch("find")
	tester.Frame()
	tester.Type("err")
	tester.Frame()
	// A report for another binding must not move the bar's count.
	w.SetSearchStatus(terminalhost.Binding{TargetID: 7, TerminalID: "term2", Generation: 99}, terminal.SearchStatus{Text: "other", Total: 5, Selected: 2})
	w.SetSearchStatus(w.find.binding, terminal.SearchStatus{Text: "", Total: 2, Selected: 2, Searching: true})
	tester.Frame()
	if tester.HasText("2/5") {
		t.Fatal("foreign search moved the count")
	}
	if !tester.HasText("…") {
		t.Fatal("ongoing search shows no ellipsis")
	}
	w.SetSearchStatus(w.find.binding, terminal.SearchStatus{Text: "", Total: 0})
	tester.Frame()
	if tester.HasText("…") {
		t.Fatal("finished search kept the ellipsis")
	}
}

func TestFindRequiresConnectedTerminal(t *testing.T) {
	w := populatedChromeWindow(t)
	if w.Can("find") || w.Can("find_next") || w.Can("find_previous") {
		t.Fatal("find available without a terminal")
	}
	w.Dispatch("find")
	tester := native.NewTester(w.View, 1200, 800)
	tester.Frame()
	if _, ok := tester.Find("Find in terminal"); ok {
		t.Fatal("find bar opened without a terminal")
	}
}

func TestFindLeavesWhenItsPaneLeaves(t *testing.T) {
	w := findWindow(t)
	tester := native.NewTester(w.View, 1200, 800)
	tester.Frame()
	w.Dispatch("find")
	tester.Frame()
	if !w.findOpen() {
		t.Fatal("find did not open")
	}
	w.app.SelectPane("p2")
	tester.Frame()
	// The bar stays while its pane is still on screen beside the focus.
	if !w.findOpen() {
		t.Fatal("bar retired while its pane was still shown")
	}
	// Zooming another pane over it takes the bar down, keyboard and all.
	zoomed, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"focused_workspace_id":"w","focused_tab_id":"t","focused_pane_id":"p2","workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"p1","terminal_id":"term1","workspace_id":"w","tab_id":"t","title":"Shell"},{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"t","title":"Other"}],"layouts":[{"workspace_id":"w","tab_id":"t","zoomed":true,"focused_pane_id":"p2","area":{"x":0,"y":0,"width":80,"height":24},"panes":[{"pane_id":"p2","rect":{"x":0,"y":0,"width":80,"height":24}}],"splits":[]}],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w.app.Update(session.PublishedSnapshot{TargetID: 7, Epoch: 3, State: session.Ready, Snapshot: zoomed})
	tester.Frame()
	if w.findOpen() {
		t.Fatal("zoomed-away pane kept its bar")
	}
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"focused_workspace_id":"w","focused_tab_id":"t","focused_pane_id":"p2","workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"t"}],"panes":[{"pane_id":"p2","terminal_id":"term2","workspace_id":"w","tab_id":"t"}],"layouts":[{"workspace_id":"w","tab_id":"t","focused_pane_id":"p2","area":{"x":0,"y":0,"width":80,"height":24},"panes":[{"pane_id":"p2","rect":{"x":0,"y":0,"width":80,"height":24}}],"splits":[]}],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	w.app.Update(session.PublishedSnapshot{TargetID: 7, Epoch: 3, State: session.Ready, Snapshot: snapshot})
	tester.Frame()
	if w.findOpen() {
		t.Fatal("bar outlived its pane")
	}
}

func TestUnfocusedCardDimPassesPointer(t *testing.T) {
	w := New(app.New(), nil)
	clicked := false
	tester := native.NewTester(func(c *native.Context) {
		w.paneCard(c, layout.Rect{W: 300, H: 200}, app.PaneRow{ID: "dimmed", Title: "w13:p8"}, app.View{}, false, true, design.Colors(false), func() {
			if native.Button(c, "Body target").Grow(1).FillWidth().Clicked() {
				clicked = true
			}
		})
	}, 300, 200)
	tester.Frame()
	if err := tester.Click("Body target"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("dim layer swallowed the pointer")
	}
}

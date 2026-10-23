package ui

import (
	"strings"

	"github.com/bytedance/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

type navigatorEntry struct{ label, pane, workspace string }

func (w *Window) navigatorEntries(query string) []navigatorEntry {
	var entries []navigatorEntry
	snapshot := w.app.Publication().Snapshot
	if snapshot == nil {
		return entries
	}
	query = strings.ToLower(strings.TrimSpace(query))
	for _, workspace := range snapshot.Workspaces() {
		name := workspace.Label
		if name == "" {
			name = workspace.ID
		}
		if strings.Contains(strings.ToLower(name+" "+workspace.CheckoutPath), query) {
			entries = append(entries, navigatorEntry{label: name, workspace: workspace.ID})
		}
		for _, tab := range snapshot.Tabs(workspace.ID) {
			for _, pane := range snapshot.Panes(tab.ID) {
				label := name + " / " + tab.Label + " / " + pane.Title
				if pane.Agent != "" {
					label += " · " + pane.Agent
				}
				if strings.Contains(strings.ToLower(label+" "+pane.CWD), query) {
					entries = append(entries, navigatorEntry{label: label, pane: pane.ID, workspace: workspace.ID})
				}
			}
		}
	}
	return entries
}
func (w *Window) navigatorPanel(c *native.Context, p design.Palette) {
	a := &w.controller.shortcut
	if !a.navigator {
		return
	}
	panel := native.Column(c.Key("session-navigator")).FillWidth().Padding(12).Gap(6).Background(p.Elevated)
	panel.Children(func() {
		native.Text(c, "Go to workspace, tab, pane or agent").Bold()
		field := native.TextInput(c.Key("navigator-query"), &a.navigatorQuery).Label("Search session").FillWidth()
		if a.navigatorFocus {
			field.Focus()
			a.navigatorFocus = false
		}
		entries := w.navigatorEntries(a.navigatorQuery)
		if field.Changed() {
			a.navigatorIndex = 0
		}
		if panel.Shortcut(0, native.KeyDown) {
			a.navigatorIndex++
		}
		if panel.Shortcut(0, native.KeyUp) {
			a.navigatorIndex--
		}
		a.navigatorIndex = max(0, min(a.navigatorIndex, len(entries)-1))
		choose := func(e navigatorEntry) {
			if e.pane != "" {
				w.selectShortcutPane(e.pane)
			} else {
				w.app.SelectWorkspace(e.workspace)
			}
			a.navigator = false
			w.BlurInput()
		}
		if (field.Submitted() || panel.Shortcut(0, native.KeyEnter)) && len(entries) > 0 {
			choose(entries[a.navigatorIndex])
		}
		if panel.Shortcut(0, native.KeyEscape) {
			a.navigator = false
			w.BlurInput()
		}
		start := max(0, a.navigatorIndex-4)
		for i := start; i < min(len(entries), start+9); i++ {
			entry := entries[i]
			button := native.Button(c.Key("goto:"+entry.workspace+":"+entry.pane), entry.label).FillWidth()
			if i == a.navigatorIndex {
				button.Background(p.Inset)
			}
			if button.Clicked() {
				choose(entry)
			}
		}
		if len(entries) == 0 {
			native.Text(c, "No matching items").TextColor(p.Secondary)
		}
	})
}

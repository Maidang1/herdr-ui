package ui

import (
	"sort"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

type navigatorEntry struct {
	label, pane, workspace, action, hint string
}

// navigatorEntries mixes the session's destinations with the command
// catalog, as Kero's and Herdr's palettes do: workspaces, tabs, panes and
// agents first, then every action by its label, ranked by how often each
// was chosen from here.
func (w *Window) navigatorEntries(query string) []navigatorEntry {
	var entries []navigatorEntry
	query = strings.ToLower(strings.TrimSpace(query))
	add := func(label, pane, workspace, action, hint string, weight int) {
		if !strings.Contains(strings.ToLower(label+" "+hint), query) {
			return
		}
		entries = append(entries, navigatorEntry{label: label, pane: pane, workspace: workspace, action: action, hint: hint})
	}
	if snapshot := w.app.Publication().Snapshot; snapshot != nil {
		for _, workspace := range snapshot.Workspaces() {
			name := workspace.Label
			if name == "" {
				name = workspace.ID
			}
			add(name, "", workspace.ID, "", workspace.CheckoutPath, 0)
			for _, tab := range snapshot.Tabs(workspace.ID) {
				for _, pane := range snapshot.Panes(tab.ID) {
					label := name + " / " + tab.Label + " / " + pane.Title
					if pane.Agent != "" {
						label += " · " + pane.Agent
					}
					add(label, pane.ID, workspace.ID, "", pane.CWD, 0)
				}
			}
		}
	}
	for _, action := range Actions {
		hint := w.BindingLabel(action.ID)
		label := w.ActionLabel(action.ID)
		add(label, "", "", action.ID, hint, 0)
	}
	// Frecency: chosen entries climb, recent choices above older ones, so
	// the palette learns the workflow without a settings surface.
	sort.SliceStable(entries, func(i, j int) bool {
		wi, wj := w.navigatorWeight(entries[i]), w.navigatorWeight(entries[j])
		if wi != wj {
			return wi > wj
		}
		isDesti, isDestj := entries[i].action == "", entries[j].action == ""
		if isDesti != isDestj {
			return isDesti
		}
		return entries[i].label < entries[j].label
	})
	return entries
}

// navigatorWeight is a pure frecency score: the count of past choices,
// with one decay tick per new palette session so last week's habits yield
// to yesterday's.
func (w *Window) navigatorWeight(entry navigatorEntry) int {
	key := entry.key()
	score := 0
	for _, use := range w.navigatorUses {
		if use.key == key {
			score = use.count - (w.navigatorSessions - use.session)
		}
	}
	return max(0, score)
}

func (e navigatorEntry) key() string {
	if e.action != "" {
		return "action:" + e.action
	}
	if e.pane != "" {
		return "pane:" + e.pane
	}
	return "workspace:" + e.workspace
}

func (w *Window) navigatorChose(entry navigatorEntry) {
	key := entry.key()
	for i := range w.navigatorUses {
		if w.navigatorUses[i].key == key {
			w.navigatorUses[i].count++
			w.navigatorUses[i].session = w.navigatorSessions
			return
		}
	}
	w.navigatorUses = append(w.navigatorUses, navigatorUse{key: key, count: 1, session: w.navigatorSessions})
}

type navigatorUse struct {
	key            string
	count, session int
}

func (w *Window) navigatorPanel(c *native.Context, p design.Palette) {
	a := &w.controller.shortcut
	if !a.navigator {
		return
	}
	panel := native.Column(c.Key("session-navigator")).FillWidth().Padding(12).Gap(6).Background(p.Elevated)
	panel.Children(func() {
		native.Text(c, "Go to anything, or run a command").Bold()
		field := native.TextInput(c.Key("navigator-query"), &a.navigatorQuery).Label("Search session and commands").FillWidth()
		if a.navigatorFocus {
			field.Focus()
			a.navigatorFocus = false
		}
		if field.Changed() {
			a.navigatorIndex = 0
		}
		if panel.Shortcut(0, native.KeyDown) {
			a.navigatorIndex++
		}
		if panel.Shortcut(0, native.KeyUp) {
			a.navigatorIndex--
		}
		entries := w.navigatorEntries(a.navigatorQuery)
		a.navigatorIndex = max(0, min(a.navigatorIndex, len(entries)-1))
		choose := func(e navigatorEntry) {
			w.navigatorChose(e)
			a.navigator = false
			if e.action != "" {
				w.Dispatch(e.action)
			} else if e.pane != "" {
				w.selectShortcutPane(e.pane)
			} else {
				w.app.SelectWorkspace(e.workspace)
			}
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
			button := native.Button(c.Key("goto:"+entry.workspace+":"+entry.pane+":"+entry.action), entry.label).FillWidth()
			if i == a.navigatorIndex {
				button.Background(p.Inset)
			}
			button.Children(func() {
				native.Row(c).FillWidth().AlignItems(native.Center).Children(func() {
					name := native.Text(c, entry.label).Grow(1).SingleLine()
					if entry.action != "" {
						name.FontSize(13)
					}
					if entry.hint != "" {
						native.Text(c, entry.hint).FontSize(11).TextColor(p.Secondary)
					}
				})
			})
			if button.Clicked() {
				choose(entry)
			}
		}
		if len(entries) == 0 {
			native.Text(c, "No matching items").TextColor(p.Secondary)
		}
	})
}

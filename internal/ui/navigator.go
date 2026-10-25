package ui

import (
	"sort"
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
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
					label := name
					if tab.Label != "" {
						label += " / " + tab.Label
					}
					label += " / " + pane.Title
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

// navigatorPanel floats the palette over the window, as Spotlight does: a
// large search field over one list of destinations and commands, each row
// marked by its kind and showing its shortcut at the trailing edge.
func (w *Window) navigatorPanel(c *native.Context, p design.Palette) {
	a := &w.controller.shortcut
	if !a.navigator {
		return
	}
	open := true
	width, height := c.Size()
	native.DialogBase(c.Key("session-navigator-dialog"), &open, func(backdrop, panel native.Element) {
		backdrop.Background(native.RGBA(0, 0, 0, .14))
		panel.Label("Go to anything, or run a command").Width(min(620, max(0, width-48))).MaxHeight(min(520, max(0, height-96))).Padding(design.MenuPadding*2).Gap(4).Radius(w.shape.Dialog).Shadow(0, 16, 48, 0, p.ContentShadow)
		w.floating(c, panel, p, glassDense)
		native.Row(c).FillWidth().Height(44).Shrink(0).Padding(0, 10).Gap(10).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.Search).Size(18, 18).Shrink(0).TextColor(p.Tertiary)
			field := native.TextInputBase(c.Key("navigator-query"), &a.navigatorQuery).Label("Search session and commands").Placeholder("Spaces, panes and commands").FontSize(17).TextColor(p.Text).FillWidth()
			if a.navigatorFocus {
				field.Focus()
				a.navigatorFocus = false
			}
			if field.Changed() {
				a.navigatorIndex = 0
			}
			entries := w.navigatorEntries(a.navigatorQuery)
			a.navigatorIndex = max(0, min(a.navigatorIndex, len(entries)-1))
			if (field.Submitted() || panel.Shortcut(0, native.KeyEnter)) && len(entries) > 0 {
				w.navigatorChoose(entries[a.navigatorIndex])
			}
		})
		if !a.navigator {
			return
		}
		if panel.Shortcut(0, native.KeyDown) {
			a.navigatorIndex++
		}
		if panel.Shortcut(0, native.KeyUp) {
			a.navigatorIndex--
		}
		native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
		entries := w.navigatorEntries(a.navigatorQuery)
		a.navigatorIndex = max(0, min(a.navigatorIndex, len(entries)-1))
		native.Scroll(c.Key("navigator-results")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			native.Column(c).FillWidth().Gap(1).PaddingY(2).Children(func() {
				for i, entry := range entries {
					w.navigatorRow(c, entry, i == a.navigatorIndex, p)
				}
				if len(entries) == 0 {
					native.Text(c, "No matching items").FontSize(13).TextColor(p.Secondary).Padding(12, 10)
				}
			})
		})
	})
	if !open && a.navigator {
		a.navigator = false
		w.BlurInput()
	}
}

// navigatorRow is one palette result. The highlighted row takes the accent,
// as the system's own lists do.
func (w *Window) navigatorRow(c *native.Context, entry navigatorEntry, highlighted bool, p design.Palette) {
	button := native.ButtonBase(c.Key("goto:"+entry.workspace+":"+entry.pane+":"+entry.action)).FillWidth().MinHeight(36).Radius(w.shape.Dialog-design.MenuPadding*2).Padding(6, 10).Label(entry.label)
	text, secondary := p.Text, p.Secondary
	if highlighted {
		button.Background(p.Accent)
		text, secondary = c.Theme().AccentText, c.Theme().AccentText.Alpha(.8)
	} else {
		w.controlState(button, p)
	}
	button.Children(func() {
		native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
			native.Icon(c, navigatorIcon(entry)).Size(15, 15).Shrink(0).TextColor(secondary)
			native.Text(c, entry.label).FontSize(13).SingleLine().Grow(1).MinWidth(0).TextColor(text)
			hint := entry.hint
			if entry.action != "" {
				hint = keyGlyphs(hint)
				if direct := directKeyLabel(entry.action); direct != "" {
					native.Text(c, direct).FontSize(11).SingleLine().Shrink(0).TextColor(text)
				}
			}
			if hint != "" {
				native.Text(c, hint).FontSize(11).SingleLine().Shrink(1).MinWidth(0).MaxWidth(220).TextColor(secondary)
			}
		})
	})
	if button.Clicked() {
		w.navigatorChoose(entry)
	}
}

func navigatorIcon(entry navigatorEntry) *native.SVG {
	switch {
	case entry.action != "":
		return icons.ChevronRight
	case entry.pane != "":
		return icons.Terminal
	}
	return icons.Folder
}

func (w *Window) navigatorChoose(e navigatorEntry) {
	a := &w.controller.shortcut
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

// keyGlyphs writes a binding label such as "ctrl+b → shift+x" in the Mac's
// own key symbols, "⌃B  ⇧X", as menus and the system's palettes show keys.
func keyGlyphs(label string) string {
	if label == "" {
		return ""
	}
	symbols := map[string]string{"ctrl": "⌃", "control": "⌃", "alt": "⌥", "option": "⌥", "shift": "⇧", "cmd": "⌘", "super": "⌘", "command": "⌘",
		"enter": "↩", "return": "↩", "tab": "⇥", "esc": "⎋", "escape": "⎋", "space": "Space", "backspace": "⌫", "delete": "⌦",
		"left": "←", "right": "→", "up": "↑", "down": "↓"}
	steps := strings.Split(label, " → ")
	for i, step := range steps {
		parts := strings.Split(step, "+")
		var out strings.Builder
		for j, part := range parts {
			if part == "" && j == len(parts)-1 {
				out.WriteString("+")
				continue
			}
			if glyph, ok := symbols[strings.ToLower(part)]; ok {
				out.WriteString(glyph)
			} else if len(part) == 1 {
				out.WriteString(strings.ToUpper(part))
			} else {
				out.WriteString(strings.ToUpper(part[:1]) + part[1:])
			}
		}
		steps[i] = out.String()
	}
	return strings.Join(steps, "  ")
}

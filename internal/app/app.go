// Package app projects authoritative session state into a window's local view.
// Its methods are called serially by the window owner and perform no IO.
package app

import (
	"strconv"
	"time"

	"github.com/bytedance/herdr-ui/internal/session"
)

type View struct {
	State                                           session.State
	Stale                                           bool
	Error                                           string
	Revision                                        uint64
	TargetID, Epoch                                 uint64
	Updated                                         time.Time
	Rows                                            []WorkspaceRow
	WorkspaceID, WorkspaceName, Path, TabID, PaneID string
	Tabs                                            []TabRow
	Panes                                           []PaneRow
}
type WorkspaceRow struct {
	ID, Name, Path, PaneID, Status, Agent string
	Depth                                 int
	Selected                              bool
	Panes                                 []PaneRow
}
type TabRow struct {
	ID, Name          string
	Number, PaneCount int
	Selected          bool
}
type PaneRow struct {
	ID, Title, Agent, Status, TerminalID, CWD string
	Selected                                  bool
	Scroll                                    session.ScrollInfo
}

type App struct {
	version           uint64
	publication       session.PublishedSnapshot
	snapshot          *session.Snapshot
	workspaceID       string
	tabs              map[string]string
	panes             map[string]string
	visitedWorkspaces map[string]bool
	visitedTabs       map[string]bool
}

func New() *App {
	return &App{publication: session.PublishedSnapshot{State: session.Disconnected, Stale: true}, tabs: make(map[string]string), panes: make(map[string]string), visitedWorkspaces: make(map[string]bool), visitedTabs: make(map[string]bool)}
}

// Update replaces authoritative data while preserving valid local selections.
func (a *App) Update(p session.PublishedSnapshot) {
	a.version++
	if a.tabs == nil {
		a.tabs = make(map[string]string)
		a.panes = make(map[string]string)
		a.visitedWorkspaces = make(map[string]bool)
		a.visitedTabs = make(map[string]bool)
	}
	if a.publication.TargetID != p.TargetID {
		a.snapshot = nil
		a.workspaceID = ""
		clear(a.tabs)
		clear(a.panes)
		clear(a.visitedWorkspaces)
		clear(a.visitedTabs)
	}
	old := a.snapshot
	a.publication = p
	if p.Snapshot != nil {
		a.snapshot = p.Snapshot
	}
	if a.snapshot == nil {
		return
	}
	s := a.snapshot
	if _, ok := s.Workspace(a.workspaceID); !ok {
		a.workspaceID = s.Metadata().FocusedWorkspaceID
		if a.workspaceID == "" {
			if ws := s.Workspaces(); len(ws) > 0 {
				a.workspaceID = ws[0].ID
			}
		}
	}
	for _, w := range s.Workspaces() {
		ts := s.Tabs(w.ID)
		selected := a.tabs[w.ID]
		if t, ok := s.Tab(selected); !a.visitedWorkspaces[w.ID] || !ok || t.WorkspaceID != w.ID {
			selected = ""
			// A closed local tab keeps the user's position in the tab order.
			if a.visitedWorkspaces[w.ID] && old != nil && a.tabs[w.ID] != "" && len(ts) > 0 {
				for i, t := range old.Tabs(w.ID) {
					if t.ID == a.tabs[w.ID] {
						selected = ts[min(i, len(ts)-1)].ID
						break
					}
				}
			}
			if selected == "" {
				selected = focusedTab(s, w, ts)
			}
		}
		a.tabs[w.ID] = selected
		for _, t := range ts {
			ps := s.Panes(t.ID)
			id := a.panes[t.ID]
			if p, ok := s.Pane(id); !a.visitedTabs[t.ID] || !ok || p.TabID != t.ID || p.WorkspaceID != w.ID {
				id = ""
				if p, ok := s.Pane(s.Metadata().FocusedPaneID); ok && p.TabID == t.ID {
					id = p.ID
				}
				if id == "" {
					id = s.FocusedPane(t.ID)
				}
				if id == "" {
					for _, p := range ps {
						if p.Focused {
							id = p.ID
							break
						}
					}
				}
				if id == "" && len(ps) > 0 {
					id = ps[0].ID
				}
			}
			a.panes[t.ID] = id
		}
	}
	// Remove memory for objects no longer in the authoritative snapshot.
	for id := range a.tabs {
		if _, ok := s.Workspace(id); !ok {
			delete(a.tabs, id)
			delete(a.visitedWorkspaces, id)
		}
	}
	for id := range a.panes {
		if _, ok := s.Tab(id); !ok {
			delete(a.panes, id)
			delete(a.visitedTabs, id)
		}
	}
	a.rememberCurrent()
}

// Only scopes actually displayed become local memories. Other projections keep
// following current server focus until the user first visits them.
func (a *App) rememberCurrent() {
	if a.workspaceID != "" {
		a.visitedWorkspaces[a.workspaceID] = true
	}
	if id := a.tabs[a.workspaceID]; id != "" {
		a.visitedTabs[id] = true
	}
}

func focusedTab(s *session.Snapshot, w session.Workspace, tabs []session.Tab) string {
	if t, ok := s.Tab(s.Metadata().FocusedTabID); ok && t.WorkspaceID == w.ID {
		return t.ID
	}
	if w.ActiveTabID != "" {
		return w.ActiveTabID
	}
	for _, t := range tabs {
		if t.Focused {
			return t.ID
		}
	}
	if len(tabs) > 0 {
		return tabs[0].ID
	}
	return ""
}

func (a *App) SelectWorkspace(id string) {
	if a.snapshot != nil {
		if _, ok := a.snapshot.Workspace(id); ok {
			a.version++
			a.workspaceID = id
			a.rememberCurrent()
		}
	}
}
func (a *App) SelectTab(id string) {
	if a.snapshot != nil {
		if t, ok := a.snapshot.Tab(id); ok && t.WorkspaceID == a.workspaceID {
			a.version++
			a.tabs[t.WorkspaceID] = id
			a.rememberCurrent()
		}
	}
}
func (a *App) SelectPane(id string) {
	if a.snapshot != nil {
		if p, ok := a.snapshot.Pane(id); ok {
			a.version++
			a.workspaceID = p.WorkspaceID
			a.tabs[p.WorkspaceID] = p.TabID
			a.panes[p.TabID] = id
			a.rememberCurrent()
		}
	}
}

// Version changes only when the authoritative publication or local selection
// changes. Terminal-only repaint must not rebuild the workspace projection.
func (a *App) Version() uint64 { return a.version }

func paneRow(p session.Pane, selected bool) PaneRow {
	return PaneRow{ID: p.ID, Title: p.Title, Agent: p.Agent, Status: p.Status, TerminalID: p.TerminalID, CWD: p.CWD, Selected: selected, Scroll: p.Scroll}
}

// View returns independently owned slices, including nested pane rows.
func (a *App) View() View {
	v := View{State: a.publication.State, Stale: a.publication.Stale, Error: a.publication.Error, Revision: a.publication.Revision, Updated: a.publication.Updated, TargetID: a.publication.TargetID, Epoch: a.publication.Epoch}
	if a.snapshot == nil {
		return v
	}
	s := a.snapshot
	v.WorkspaceID = a.workspaceID
	v.TabID = a.tabs[v.WorkspaceID]
	v.PaneID = a.panes[v.TabID]
	rows := make(map[string]WorkspaceRow)
	ws := s.Workspaces()
	for _, w := range ws {
		id := a.panes[a.tabs[w.ID]]
		p, _ := s.Pane(id)
		r := WorkspaceRow{ID: w.ID, Name: w.Label, Path: w.CheckoutPath, PaneID: id, Agent: p.Agent, Status: p.Status, Selected: w.ID == v.WorkspaceID}
		if r.Path == "" {
			r.Path = p.CWD
		}
		for _, t := range s.Tabs(w.ID) {
			ps := s.Panes(t.ID)
			for _, p := range ps {
				r.Panes = append(r.Panes, paneRow(p, r.Selected && p.ID == v.PaneID))
			}
			if r.Selected {
				name := t.Label
				if name == "" {
					name = strconv.Itoa(t.Number)
				}
				v.Tabs = append(v.Tabs, TabRow{ID: t.ID, Name: name, Number: t.Number, PaneCount: len(ps), Selected: t.ID == v.TabID})
				if t.ID == v.TabID {
					for _, p := range ps {
						v.Panes = append(v.Panes, paneRow(p, p.ID == v.PaneID))
					}
				}
			}
		}
		if len(r.Panes) <= 1 {
			r.Panes = nil
		}
		if r.Selected {
			v.WorkspaceName = r.Name
			v.Path = r.Path
		}
		rows[w.ID] = r
	}
	parents := make(map[string]string)
	for _, child := range ws {
		if !child.Linked || child.RepoRoot == "" {
			continue
		}
		parent := ""
		count := 0
		for _, candidate := range ws {
			if candidate.Linked || candidate.ID == child.ID {
				continue
			}
			if child.RepoKey != "" && candidate.RepoKey != "" && child.RepoKey != candidate.RepoKey {
				continue
			}
			if child.RepoRoot == candidate.CheckoutPath || child.RepoRoot == candidate.RepoRoot {
				parent = candidate.ID
				count++
			}
		}
		if count == 1 {
			parents[child.ID] = parent
		}
	}
	for _, w := range ws {
		if parents[w.ID] != "" {
			continue
		}
		v.Rows = append(v.Rows, rows[w.ID])
		for _, child := range ws {
			if parents[child.ID] == w.ID {
				r := rows[child.ID]
				r.Depth = 1
				v.Rows = append(v.Rows, r)
			}
		}
	}
	return v
}

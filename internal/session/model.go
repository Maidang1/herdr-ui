package session

import (
	"encoding/json"
	"fmt"

	"github.com/bytedance/herdr-ui/internal/layout"
)

// Metadata contains the server's identity and focus observations. Focus is not a
// command to change a window's local selection.
type Metadata struct {
	Version                                         string
	Protocol                                        int
	FocusedWorkspaceID, FocusedTabID, FocusedPaneID string
}

type Workspace struct {
	ID, Label, ActiveTabID, CheckoutPath, RepoRoot, RepoKey, Status string
	Linked                                                          bool
	Number                                                          int
}
type Tab struct {
	ID, WorkspaceID, Label string
	Number                 int
	Focused                bool
}
type Pane struct {
	ID, TerminalID, WorkspaceID, TabID, CWD, Title, Label, Agent, Status string
	Focused                                                              bool
	Scroll                                                               ScrollInfo
}

// ScrollInfo is server history, not the attach client's local scrollback.
type ScrollInfo struct {
	Offset, MaxOffset, Rows uint64
	Known                   bool
}

// PaneLayout retains only the server's visible rectangles, including zoom.
type PaneLayout struct {
	WorkspaceID, TabID, FocusedPaneID string
	Zoomed                            bool
	Area                              layout.Rect
	Panes                             []layout.PaneRect
	Splits                            []layout.SplitRect
}

// Snapshot owns all indexes and authoritative ordering. Once decoded, it is
// immutable and may be shared between readers without synchronization.
type Snapshot struct {
	metadata       Metadata
	workspaces     []Workspace
	tabs           []Tab
	panes          []Pane
	workspaceIndex map[string]Workspace
	tabIndex       map[string]Tab
	paneIndex      map[string]Pane
	focusedPanes   map[string]string
	layouts        map[string]PaneLayout
}

// Layout returns a value copy with independently owned geometry slices.
func (s *Snapshot) Layout(tabID string) (PaneLayout, bool) {
	v, ok := s.layouts[tabID]
	v.Panes = append([]layout.PaneRect(nil), v.Panes...)
	v.Splits = append([]layout.SplitRect(nil), v.Splits...)
	return v, ok
}

// FocusedPane returns the layout's focus within a tab, including inactive tabs.
// An absent layout or focus returns an empty ID.
func (s *Snapshot) FocusedPane(tabID string) string { return s.focusedPanes[tabID] }

func (s *Snapshot) Metadata() Metadata      { return s.metadata }
func (s *Snapshot) Workspaces() []Workspace { return append([]Workspace(nil), s.workspaces...) }
func (s *Snapshot) Tabs(workspaceID string) []Tab {
	var out []Tab
	for _, t := range s.tabs {
		if t.WorkspaceID == workspaceID {
			out = append(out, t)
		}
	}
	return out
}
func (s *Snapshot) Panes(tabID string) []Pane {
	var out []Pane
	for _, p := range s.panes {
		if p.TabID == tabID {
			out = append(out, p)
		}
	}
	return out
}
func (s *Snapshot) PaneIDs() []string {
	out := make([]string, len(s.panes))
	for i, p := range s.panes {
		out[i] = p.ID
	}
	return out
}
func (s *Snapshot) Workspace(id string) (Workspace, bool) {
	w, ok := s.workspaceIndex[id]
	return w, ok
}
func (s *Snapshot) Tab(id string) (Tab, bool)   { t, ok := s.tabIndex[id]; return t, ok }
func (s *Snapshot) Pane(id string) (Pane, bool) { p, ok := s.paneIndex[id]; return p, ok }

// ReferenceError identifies a layout referring to a pane absent from the same
// snapshot. The coordinator may refresh once to resolve this transient race.
type ReferenceError struct{ PaneID string }

func (e *ReferenceError) Error() string {
	return fmt.Sprintf("layout references missing pane %q", e.PaneID)
}

type snapshotWire struct {
	Version            string          `json:"version"`
	Protocol           int             `json:"protocol"`
	FocusedWorkspaceID string          `json:"focused_workspace_id"`
	FocusedTabID       string          `json:"focused_tab_id"`
	FocusedPaneID      string          `json:"focused_pane_id"`
	Workspaces         []workspaceWire `json:"workspaces"`
	Tabs               []tabWire       `json:"tabs"`
	Panes              []paneWire      `json:"panes"`
	Layouts            []layoutWire    `json:"layouts"`
	Agents             []agentWire     `json:"agents"`
}
type workspaceWire struct {
	ID          string `json:"workspace_id"`
	Label       string `json:"label"`
	ActiveTabID string `json:"active_tab_id"`
	Status      string `json:"agent_status"`
	Number      int    `json:"number"`
	Worktree    *struct {
		CheckoutPath string `json:"checkout_path"`
		RepoRoot     string `json:"repo_root"`
		RepoKey      string `json:"repo_key"`
		Linked       bool   `json:"is_linked_worktree"`
	} `json:"worktree"`
}
type tabWire struct {
	ID          string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	Focused     bool   `json:"focused"`
}
type paneWire struct {
	ID                    string `json:"pane_id"`
	TerminalID            string `json:"terminal_id"`
	WorkspaceID           string `json:"workspace_id"`
	TabID                 string `json:"tab_id"`
	CWD                   string `json:"cwd"`
	Label                 string `json:"label"`
	Title                 string `json:"title"`
	TerminalTitleStripped string `json:"terminal_title_stripped"`
	TerminalTitle         string `json:"terminal_title"`
	Agent                 string `json:"agent"`
	DisplayAgent          string `json:"display_agent"`
	Status                string `json:"agent_status"`
	Focused               bool   `json:"focused"`
	Scroll                *struct {
		Offset    uint64 `json:"offset_from_bottom"`
		MaxOffset uint64 `json:"max_offset_from_bottom"`
		Rows      uint64 `json:"viewport_rows"`
	} `json:"scroll"`
}
type agentWire struct {
	PaneID       string `json:"pane_id"`
	TerminalID   string `json:"terminal_id"`
	WorkspaceID  string `json:"workspace_id"`
	TabID        string `json:"tab_id"`
	Agent        string `json:"agent"`
	DisplayAgent string `json:"display_agent"`
	Status       string `json:"agent_status"`
}
type rectWire struct {
	X      *uint16 `json:"x"`
	Y      *uint16 `json:"y"`
	Width  *uint16 `json:"width"`
	Height *uint16 `json:"height"`
}

func (r *rectWire) valid() bool {
	return r != nil && r.X != nil && r.Y != nil && r.Width != nil && r.Height != nil
}

func (r *rectWire) value() layout.Rect {
	return layout.Rect{X: float64(*r.X), Y: float64(*r.Y), W: float64(*r.Width), H: float64(*r.Height)}
}

type layoutWire struct {
	Zoomed        bool      `json:"zoomed"`
	WorkspaceID   string    `json:"workspace_id"`
	TabID         string    `json:"tab_id"`
	FocusedPaneID string    `json:"focused_pane_id"`
	Area          *rectWire `json:"area"`
	Panes         []struct {
		PaneID string    `json:"pane_id"`
		Rect   *rectWire `json:"rect"`
	} `json:"panes"`
	Splits []struct {
		ID        string    `json:"id"`
		Direction string    `json:"direction"`
		Ratio     *float64  `json:"ratio"`
		Rect      *rectWire `json:"rect"`
	} `json:"splits"`
}

func firstValue(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// DecodeSnapshot validates the session.snapshot RPC result, accepting unknown
// additive fields and status strings while rejecting incomplete identity data.
func DecodeSnapshot(data []byte) (*Snapshot, error) {
	var result struct {
		Type     string        `json:"type"`
		Snapshot *snapshotWire `json:"snapshot"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode snapshot: %w", err)
	}
	if result.Type != "session_snapshot" || result.Snapshot == nil {
		return nil, fmt.Errorf("expected session_snapshot result")
	}
	w := result.Snapshot
	if w.Version == "" || w.Protocol < 1 {
		return nil, fmt.Errorf("missing snapshot version or protocol")
	}
	if w.Workspaces == nil || w.Tabs == nil || w.Panes == nil || w.Layouts == nil || w.Agents == nil {
		return nil, fmt.Errorf("snapshot requires workspaces, tabs, panes, layouts and agents arrays")
	}
	s := &Snapshot{metadata: Metadata{w.Version, w.Protocol, w.FocusedWorkspaceID, w.FocusedTabID, w.FocusedPaneID}, workspaceIndex: make(map[string]Workspace), tabIndex: make(map[string]Tab), paneIndex: make(map[string]Pane), focusedPanes: make(map[string]string)}
	s.layouts = make(map[string]PaneLayout)
	for _, v := range w.Workspaces {
		if _, exists := s.workspaceIndex[v.ID]; v.ID == "" || exists {
			return nil, fmt.Errorf("missing or duplicate workspace ID %q", v.ID)
		}
		value := Workspace{ID: v.ID, Label: v.Label, ActiveTabID: v.ActiveTabID, Number: v.Number, Status: v.Status}
		if v.Worktree != nil {
			value.CheckoutPath = v.Worktree.CheckoutPath
			value.RepoRoot = v.Worktree.RepoRoot
			value.RepoKey = v.Worktree.RepoKey
			value.Linked = v.Worktree.Linked
		}
		s.workspaces = append(s.workspaces, value)
		s.workspaceIndex[value.ID] = value
	}
	for _, v := range w.Tabs {
		if _, exists := s.tabIndex[v.ID]; v.ID == "" || exists {
			return nil, fmt.Errorf("missing or duplicate tab ID %q", v.ID)
		}
		if _, ok := s.workspaceIndex[v.WorkspaceID]; !ok {
			return nil, fmt.Errorf("tab %q references unknown workspace %q", v.ID, v.WorkspaceID)
		}
		value := Tab{v.ID, v.WorkspaceID, v.Label, v.Number, v.Focused}
		s.tabs = append(s.tabs, value)
		s.tabIndex[value.ID] = value
	}
	for _, v := range s.workspaces {
		if v.ActiveTabID != "" {
			tab, ok := s.tabIndex[v.ActiveTabID]
			if !ok || tab.WorkspaceID != v.ID {
				return nil, fmt.Errorf("workspace %q has invalid active tab", v.ID)
			}
		}
	}
	terminals := make(map[string]bool)
	for _, v := range w.Panes {
		if _, exists := s.paneIndex[v.ID]; v.ID == "" || exists {
			return nil, fmt.Errorf("missing or duplicate pane ID %q", v.ID)
		}
		if v.TerminalID == "" || terminals[v.TerminalID] {
			return nil, fmt.Errorf("missing or duplicate terminal ID %q", v.TerminalID)
		}
		terminals[v.TerminalID] = true
		tab, ok := s.tabIndex[v.TabID]
		if !ok || tab.WorkspaceID != v.WorkspaceID {
			return nil, fmt.Errorf("pane %q has invalid workspace/tab reference", v.ID)
		}
		value := Pane{ID: v.ID, TerminalID: v.TerminalID, WorkspaceID: v.WorkspaceID, TabID: v.TabID, CWD: v.CWD, Label: v.Label, Title: firstValue(v.Label, v.Title, v.TerminalTitleStripped, v.TerminalTitle, v.ID), Agent: firstValue(v.DisplayAgent, v.Agent), Status: v.Status, Focused: v.Focused}
		if v.Scroll != nil {
			if v.Scroll.Offset > v.Scroll.MaxOffset {
				return nil, fmt.Errorf("pane %q has invalid scroll metrics", v.ID)
			}
			value.Scroll = ScrollInfo{v.Scroll.Offset, v.Scroll.MaxOffset, v.Scroll.Rows, true}
		}
		s.panes = append(s.panes, value)
		s.paneIndex[value.ID] = value
	}
	agents := make(map[string]bool)
	for _, v := range w.Agents {
		p, ok := s.paneIndex[v.PaneID]
		if !ok || agents[v.PaneID] || p.TerminalID != v.TerminalID || p.WorkspaceID != v.WorkspaceID || p.TabID != v.TabID {
			return nil, fmt.Errorf("agent has inconsistent identity for pane %q", v.PaneID)
		}
		agents[v.PaneID] = true
		if a := firstValue(v.DisplayAgent, v.Agent); a != "" {
			p.Agent = a
		}
		if v.Status != "" {
			p.Status = v.Status
		}
		s.paneIndex[p.ID] = p
	}
	for i, p := range s.panes {
		s.panes[i] = s.paneIndex[p.ID]
	}
	if w.FocusedWorkspaceID != "" {
		if _, ok := s.workspaceIndex[w.FocusedWorkspaceID]; !ok {
			return nil, fmt.Errorf("invalid focused workspace")
		}
	}
	if w.FocusedTabID != "" {
		t, ok := s.tabIndex[w.FocusedTabID]
		if !ok || (w.FocusedWorkspaceID != "" && t.WorkspaceID != w.FocusedWorkspaceID) {
			return nil, fmt.Errorf("invalid focused tab")
		}
	}
	if w.FocusedPaneID != "" {
		p, ok := s.paneIndex[w.FocusedPaneID]
		if !ok || (w.FocusedTabID != "" && p.TabID != w.FocusedTabID) || (w.FocusedWorkspaceID != "" && p.WorkspaceID != w.FocusedWorkspaceID) {
			return nil, fmt.Errorf("invalid focused pane")
		}
	}
	layouts := make(map[string]bool)
	for _, v := range w.Layouts {
		tab, ok := s.tabIndex[v.TabID]
		if !ok || tab.WorkspaceID != v.WorkspaceID || layouts[v.TabID] {
			return nil, fmt.Errorf("layout has invalid or duplicate workspace/tab reference")
		}
		layouts[v.TabID] = true
		if !v.Area.valid() || v.Panes == nil || v.Splits == nil {
			return nil, fmt.Errorf("layout %q missing geometry", v.TabID)
		}
		value := PaneLayout{WorkspaceID: v.WorkspaceID, TabID: v.TabID, FocusedPaneID: v.FocusedPaneID, Zoomed: v.Zoomed, Area: v.Area.value()}
		seen := make(map[string]bool)
		checkPane := func(id string) error {
			if id == "" {
				return fmt.Errorf("layout missing pane ID")
			}
			p, ok := s.paneIndex[id]
			if !ok {
				return &ReferenceError{PaneID: id}
			}
			if p.TabID != v.TabID || p.WorkspaceID != v.WorkspaceID {
				return fmt.Errorf("layout pane %q belongs to another tab", id)
			}
			return nil
		}
		for _, p := range v.Panes {
			if !p.Rect.valid() || seen[p.PaneID] {
				return nil, fmt.Errorf("layout has invalid pane rectangle or duplicate pane")
			}
			if err := checkPane(p.PaneID); err != nil {
				return nil, err
			}
			seen[p.PaneID] = true
			value.Panes = append(value.Panes, layout.PaneRect{PaneID: p.PaneID, Rect: p.Rect.value()})
		}
		if v.FocusedPaneID != "" {
			if err := checkPane(v.FocusedPaneID); err != nil {
				return nil, err
			}
			s.focusedPanes[v.TabID] = v.FocusedPaneID
		}
		splitIDs := make(map[string]bool)
		for _, split := range v.Splits {
			if split.ID == "" || splitIDs[split.ID] || !split.Rect.valid() || split.Ratio == nil || *split.Ratio <= 0 || *split.Ratio >= 1 || (split.Direction != "right" && split.Direction != "down") {
				return nil, fmt.Errorf("layout has invalid split")
			}
			splitIDs[split.ID] = true
			value.Splits = append(value.Splits, layout.SplitRect{ID: split.ID, Direction: split.Direction, Ratio: *split.Ratio, Rect: split.Rect.value()})
		}
		// Herdr exports underlying rectangles even while zoomed. Visibility follows
		// the explicit zoom flag and scoped focus, never the global focus.
		if value.Zoomed {
			if !seen[value.FocusedPaneID] {
				return nil, fmt.Errorf("zoomed layout has no visible focused pane")
			}
			value.Panes = []layout.PaneRect{{PaneID: value.FocusedPaneID, Rect: value.Area}}
			value.Splits = nil
		}
		s.layouts[v.TabID] = value
	}
	return s, nil
}

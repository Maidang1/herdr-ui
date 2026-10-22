package herdr

import (
	"encoding/json"
	"fmt"
)

// The daemon sends the session as a snapshot: the workspaces, the tabs in
// them, the panes in those tabs and the agents that work in the panes. A
// client renders the snapshot and follows the next one when something
// changes.

// Snapshot is the session as the daemon sees it.
type Snapshot struct {
	BootID                string             `json:"boot_id"`
	Revision              uint64             `json:"revision"`
	ConfigDiagnostic      *string            `json:"config_diagnostic"`
	ProductAnnouncement   *Announcement      `json:"product_announcement"`
	UpdateAvailable       *string            `json:"update_available"`
	UpdateInstallCommand  string             `json:"update_install_command"`
	ServerKeybindingsTOML *string            `json:"server_keybindings_toml"`
	LatestReleaseNotes    bool               `json:"latest_release_notes_available"`
	IntegrationUpdates    bool               `json:"integration_updates_available"`
	WorktreeDirectory     string             `json:"worktree_directory"`
	ReleaseNotes          *ReleaseNotes      `json:"release_notes"`
	FocusedWorkspaceID    *string            `json:"focused_workspace_id"`
	FocusedTabID          *string            `json:"focused_tab_id"`
	FocusedPaneID         *string            `json:"focused_pane_id"`
	TabBarRight           []TabStatusSegment `json:"tab_bar_right"`
	TabBarRightSeparator  string             `json:"tab_bar_right_separator"`
	AgentViewLabel        *string            `json:"agent_view_label"`
	AgentOrder            []string           `json:"agent_order"`
	Workspaces            []Workspace        `json:"workspaces"`
	Tabs                  []Tab              `json:"tabs"`
	Panes                 []Pane             `json:"panes"`
	Agents                []Agent            `json:"agents"`
	Commands              []Command          `json:"commands"`
}

// Announcement is a message the daemon shows above the session.
type Announcement struct {
	Version string `json:"version"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Preview bool   `json:"preview"`
}

// ReleaseNotes are the notes of the running version.
type ReleaseNotes struct {
	Version string `json:"version"`
	Body    string `json:"body"`
	Preview bool   `json:"preview"`
}

// TabStatusSegment is one piece of the status the tab bar shows on the right.
type TabStatusSegment struct {
	Text   string `json:"text"`
	Accent bool   `json:"accent"`
}

// Workspace is one row of the sidebar.
type Workspace struct {
	WorkspaceID     string          `json:"workspace_id"`
	ActiveTabID     string          `json:"active_tab_id"`
	NewWorkspaceCWD string          `json:"new_workspace_cwd"`
	Number          int             `json:"number"`
	Label           string          `json:"label"`
	CustomLabel     bool            `json:"custom_label"`
	Branch          *string         `json:"branch"`
	GitAheadBehind  *GitAheadBehind `json:"git_ahead_behind"`
	Tokens          []Token         `json:"tokens"`
	Worktree        *Worktree       `json:"worktree"`
	Focused         bool            `json:"focused"`
	AgentStatus     string          `json:"agent_status"`
}

// GitAheadBehind says how far a branch is from its remote.
type GitAheadBehind struct {
	Ahead  int `json:"ahead"`
	Behind int `json:"behind"`
}

// UnmarshalJSON reads the pair the daemon sends as [ahead, behind].
func (g *GitAheadBehind) UnmarshalJSON(data []byte) error {
	var pair [2]int
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	g.Ahead, g.Behind = pair[0], pair[1]
	return nil
}

// Token is one name and count the daemon shows.
type Token struct {
	Name  string
	Value string
}

// UnmarshalJSON reads the pair the daemon sends as [name, value].
func (t *Token) UnmarshalJSON(data []byte) error {
	var pair [2]string
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	t.Name, t.Value = pair[0], pair[1]
	return nil
}

// Worktree names the worktree a workspace lives in.
type Worktree struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	IsLinkedWorktree bool   `json:"is_linked_worktree"`
}

// Tab is one tab of a workspace.
type Tab struct {
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Number      int    `json:"number"`
	Label       string `json:"label"`
	CustomLabel bool   `json:"custom_label"`
	Zoomed      bool   `json:"zoomed"`
	Focused     bool   `json:"focused"`
	AgentStatus string `json:"agent_status"`
}

// Pane is one terminal in a tab.
type Pane struct {
	PaneID                string  `json:"pane_id"`
	WorkspaceID           string  `json:"workspace_id"`
	TabID                 string  `json:"tab_id"`
	Label                 *string `json:"label"`
	CWD                   *string `json:"cwd"`
	ForegroundCWD         *string `json:"foreground_cwd"`
	Focused               bool    `json:"focused"`
	RightClickPassthrough bool    `json:"right_click_passthrough"`
}

// Agent is one coding agent the daemon sees in a pane.
type Agent struct {
	PaneID             string  `json:"pane_id"`
	WorkspaceID        string  `json:"workspace_id"`
	TabID              string  `json:"tab_id"`
	Name               *string `json:"name"`
	DisplayAgent       *string `json:"display_agent"`
	Agent              *string `json:"agent"`
	Title              *string `json:"title"`
	TerminalTitle      *string `json:"terminal_title"`
	TerminalTitleShort *string `json:"terminal_title_stripped"`
	AgentStatus        string  `json:"agent_status"`
	StateChangeSeq     uint64  `json:"state_change_seq"`
	StateLabels        []Token `json:"state_labels"`
	Tokens             []Token `json:"tokens"`
	Focused            bool    `json:"focused"`
}

// Command is one command the daemon offers.
type Command struct {
	CommandID     string   `json:"command_id"`
	BindingLabel  string   `json:"binding_label"`
	BindingLabels []string `json:"binding_labels"`
	Action        string   `json:"action"`
	Description   string   `json:"description"`
}

// TabsOf returns the tabs of one workspace.
func (s *Snapshot) TabsOf(workspaceID string) []Tab {
	var tabs []Tab
	for _, tab := range s.Tabs {
		if tab.WorkspaceID == workspaceID {
			tabs = append(tabs, tab)
		}
	}
	return tabs
}

// PanesOf returns the panes of one tab.
func (s *Snapshot) PanesOf(tabID string) []Pane {
	var panes []Pane
	for _, pane := range s.Panes {
		if pane.TabID == tabID {
			panes = append(panes, pane)
		}
	}
	return panes
}

// Workspace returns the workspace with this ID.
func (s *Snapshot) Workspace(id string) (Workspace, bool) {
	for _, workspace := range s.Workspaces {
		if workspace.WorkspaceID == id {
			return workspace, true
		}
	}
	return Workspace{}, false
}

// Tab returns the tab with this ID.
func (s *Snapshot) Tab(id string) (Tab, bool) {
	for _, tab := range s.Tabs {
		if tab.TabID == id {
			return tab, true
		}
	}
	return Tab{}, false
}

// Pane returns the pane with this ID.
func (s *Snapshot) Pane(id string) (Pane, bool) {
	for _, pane := range s.Panes {
		if pane.PaneID == id {
			return pane, true
		}
	}
	return Pane{}, false
}

// AgentOf returns the agent that works in this pane.
func (s *Snapshot) AgentOf(paneID string) (Agent, bool) {
	for _, agent := range s.Agents {
		if agent.PaneID == paneID {
			return agent, true
		}
	}
	return Agent{}, false
}

// FocusedWorkspace returns the workspace the daemon focuses.
func (s *Snapshot) FocusedWorkspace() (Workspace, bool) {
	if s.FocusedWorkspaceID == nil {
		return Workspace{}, false
	}
	return s.Workspace(*s.FocusedWorkspaceID)
}

// FocusedTab returns the tab the daemon focuses.
func (s *Snapshot) FocusedTab() (Tab, bool) {
	if s.FocusedTabID == nil {
		return Tab{}, false
	}
	return s.Tab(*s.FocusedTabID)
}

// FocusedPane returns the pane the daemon focuses.
func (s *Snapshot) FocusedPane() (Pane, bool) {
	if s.FocusedPaneID == nil {
		return Pane{}, false
	}
	return s.Pane(*s.FocusedPaneID)
}

// String says which pane the snapshot focuses, for the logs.
func (s *Snapshot) String() string {
	return fmt.Sprintf("boot=%s workspaces=%d tabs=%d panes=%d agents=%d",
		s.BootID, len(s.Workspaces), len(s.Tabs), len(s.Panes), len(s.Agents))
}

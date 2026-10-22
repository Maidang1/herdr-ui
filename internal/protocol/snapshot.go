package protocol

import "encoding/json"

// The snapshot of the session.
//
// The daemon sends the snapshot as JSON, in a named control message. It is
// the state of everything the window shows: the workspaces, the tabs, the
// panes, the agents and the commands.

// AgentStatus is what an agent is doing.
type AgentStatus string

// The states an agent is in.
const (
	AgentIdle    AgentStatus = "idle"
	AgentWorking AgentStatus = "working"
	AgentBlocked AgentStatus = "blocked"
	AgentDone    AgentStatus = "done"
	AgentUnknown AgentStatus = "unknown"
)

// Snapshot is the state of the session.
type Snapshot struct {
	// BootID is the boot the snapshot belongs to.
	BootID string `json:"boot_id"`
	// Revision is how many times the snapshot changed.
	Revision uint64 `json:"revision"`
	// WorktreeDirectory is where the worktrees of the session live.
	WorktreeDirectory string `json:"worktree_directory"`
	// FocusedWorkspaceID is the workspace that has the keyboard.
	FocusedWorkspaceID string `json:"focused_workspace_id"`
	// FocusedTabID is the tab that has the keyboard.
	FocusedTabID string `json:"focused_tab_id"`
	// FocusedPaneID is the pane that has the keyboard.
	FocusedPaneID string `json:"focused_pane_id"`
	// TabBarRightSeparator is what stands between the tabs and the
	// meters.
	TabBarRightSeparator string `json:"tab_bar_right_separator"`
	// AgentViewLabel is the name of the view of the agents.
	AgentViewLabel string `json:"agent_view_label"`
	// TabBarRight are the pieces at the right of the strip of the tabs.
	TabBarRight []TabStatusSegment `json:"tab_bar_right"`
	// AgentOrder is the order of the agents.
	AgentOrder []string `json:"agent_order"`
	// Workspaces are the workspaces of the session.
	Workspaces []Workspace `json:"workspaces"`
	// Tabs are the tabs of the session.
	Tabs []Tab `json:"tabs"`
	// Panes are the panes of the session.
	Panes []Pane `json:"panes"`
	// Agents are the agents of the session.
	Agents []Agent `json:"agents"`
	// Commands are the commands the session has.
	Commands []Command `json:"commands"`
}

// TabStatusSegment is one piece at the right of the strip of the tabs.
type TabStatusSegment struct {
	// Text is what it says.
	Text string `json:"text"`
	// Accent tells that it is said in the accent.
	Accent bool `json:"accent"`
}

// Workspace is a workspace of the session.
type Workspace struct {
	// WorkspaceID is the workspace.
	WorkspaceID string `json:"workspace_id"`
	// ActiveTabID is the tab of the workspace that is in front.
	ActiveTabID string `json:"active_tab_id"`
	// Number is the number of the workspace.
	Number int `json:"number"`
	// Label is the name of the workspace.
	Label string `json:"label"`
	// CustomLabel tells that the name was given by the user.
	CustomLabel bool `json:"custom_label"`
	// Branch is the branch of the workspace.
	Branch string `json:"branch"`
	// Worktree is the worktree of the workspace.
	Worktree *Worktree `json:"worktree"`
	// Focused tells that the workspace has the keyboard.
	Focused bool `json:"focused"`
	// AgentStatus is what the agent of the workspace is doing.
	AgentStatus AgentStatus `json:"agent_status"`
}

// Worktree is the worktree of a workspace.
type Worktree struct {
	// Key is the name of the worktree.
	Key string `json:"key"`
	// Label is the name to show.
	Label string `json:"label"`
	// IsLinkedWorktree tells that the worktree is linked to another.
	IsLinkedWorktree bool `json:"is_linked_worktree"`
}

// Tab is a tab of the session.
type Tab struct {
	// TabID is the tab.
	TabID string `json:"tab_id"`
	// WorkspaceID is the workspace the tab is in.
	WorkspaceID string `json:"workspace_id"`
	// Number is the number of the tab.
	Number int `json:"number"`
	// Label is the name of the tab.
	Label string `json:"label"`
	// CustomLabel tells that the name was given by the user.
	CustomLabel bool `json:"custom_label"`
	// Zoomed tells that the tab is in front of the others.
	Zoomed bool `json:"zoomed"`
	// Focused tells that the tab has the keyboard.
	Focused bool `json:"focused"`
	// AgentStatus is what the agent of the tab is doing.
	AgentStatus AgentStatus `json:"agent_status"`
}

// Pane is a pane of the session.
type Pane struct {
	// PaneID is the pane.
	PaneID string `json:"pane_id"`
	// WorkspaceID is the workspace the pane is in.
	WorkspaceID string `json:"workspace_id"`
	// TabID is the tab the pane is in.
	TabID string `json:"tab_id"`
	// Label is the name of the pane.
	Label string `json:"label"`
	// Cwd is where the pane is.
	Cwd string `json:"cwd"`
	// Focused tells that the pane has the keyboard.
	Focused bool `json:"focused"`
}

// Agent is an agent that works in a pane.
type Agent struct {
	// PaneID is the pane the agent works in.
	PaneID string `json:"pane_id"`
	// WorkspaceID is the workspace of the agent.
	WorkspaceID string `json:"workspace_id"`
	// TabID is the tab of the agent.
	TabID string `json:"tab_id"`
	// Name is the name of the agent.
	Name string `json:"name"`
	// DisplayAgent is the name to show.
	DisplayAgent string `json:"display_agent"`
	// Title is the title of what the agent is doing.
	Title string `json:"title"`
	// TerminalTitle is the title of the terminal of the agent.
	TerminalTitle string `json:"terminal_title"`
	// AgentStatus is what the agent is doing.
	AgentStatus AgentStatus `json:"agent_status"`
	// StateLabels are the labels of the state of the agent.
	StateLabels []StateLabel `json:"state_labels"`
	// Focused tells that the agent has the keyboard.
	Focused bool `json:"focused"`
}

// StateLabel is one label of the state of an agent.
type StateLabel struct {
	// Name is the name of the label.
	Name string `json:"name"`
	// Value is what the label says.
	Value string `json:"value"`
}

// Command is a command of the session.
type Command struct {
	// CommandID is the command.
	CommandID string `json:"command_id"`
	// BindingLabel is the key that runs the command.
	BindingLabel string `json:"binding_label"`
	// BindingLabels are the keys that run the command.
	BindingLabels []string `json:"binding_labels"`
	// Description is what the command does.
	Description string `json:"description"`
}

// DecodeSnapshot reads a snapshot from its JSON.
func DecodeSnapshot(data string) (*Snapshot, error) {
	snapshot := &Snapshot{}
	if err := json.Unmarshal([]byte(data), snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

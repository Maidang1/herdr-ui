package ui

import (
	"herdr-ui/internal/design"
)

// The model of what the window shows.
//
// It is the shape the window draws, and nothing else: where the data comes
// from — the daemon for the session, the local machine for the rest — is
// the business of the code that fills it in.

import "github.com/egoist/mygo/ui"

// Role is the sense of a piece of the text of the terminal.
//
// The terminal's colors come from the daemon, but the window writes a few
// things of its own — the prompt of a session, the name of a tool — and
// those are written in the colors of the design.
type Role int

const (
	// RolePrimary is the text that matters most.
	RolePrimary Role = iota
	// RoleSecondary is the text that says less.
	RoleSecondary
	// RoleGreen is the text that says what went well.
	RoleGreen
	// RoleYellow is the text that says what wants attention.
	RoleYellow
	// RoleRed is the text that says what went wrong.
	RoleRed
)

// Segment is a piece of a line of the terminal.
type Segment struct {
	// Text is what is written.
	Text string
	// Role is the sense of the piece.
	Role Role
	// Weight is the weight of the piece, when it is not the usual one.
	Weight int
}

// Space is a workspace of the session, and what is in it.
type Space struct {
	// Name is the name of the workspace.
	Name string
	// Selected tells that this is the workspace that is chosen.
	Selected bool
	// Branch is the branch the workspace is on.
	Branch string
	// Ports is the ports the workspace listens on.
	Ports string
	// Agents are the agents that work in it.
	Agents []Agent
}

// Agent is a coding agent, and what it is doing.
type Agent struct {
	// Name is the name of the agent.
	Name string
	// Branch is the branch the agent works on.
	Branch string
	// Ports are the ports the agent listens on.
	Ports string
	// State is what the agent is doing: idle, working, blocked.
	State string
}

// Group is a group of agents.
type Group struct {
	// Name is the name of the group.
	Name string
	// Count is the number of agents in it.
	Count int
	// Agents are the agents of the group.
	Agents []Agent
}

// Tab is a tab of the strip of tabs.
type Tab struct {
	// Number is the number of the tab.
	Number int
	// Label is the name of the tab.
	Label string
	// Active tells that this is the tab that is chosen.
	Active bool
	// Dot is the tone of the dot at the left of the tab.
	Dot Tone
}

// Tone is the sense of a color of the window.
type Tone int

const (
	// ToneNeutral is a thing that says nothing in particular.
	ToneNeutral Tone = iota
	// ToneAccent is the thing the window is about.
	ToneAccent
	// ToneSuccess is what went well.
	ToneSuccess
	// ToneWarning is what wants attention.
	ToneWarning
	// ToneDanger is what went wrong.
	ToneDanger
	// ToneInfo is what is worth reading.
	ToneInfo
)

// Meter is one of the meters of the row of meters.
type Meter struct {
	// Icon is the name of the icon of it.
	Icon string
	// Value is how much of it is used, from 0 to 1.
	Value float32
	// Label is what it says.
	Label string
	// Tone is the sense of it.
	Tone Tone
}

// Session is the session the window shows.
type Session struct {
	// Title is the name of the session.
	Title string
	// Spaces are the workspaces of the session.
	Spaces []Space
	// Groups are the groups of agents.
	Groups []Group
	// Tabs are the tabs of the strip of tabs.
	Tabs []Tab
	// Lines are the lines of the terminal.
	Lines [][]Segment
	// Meters are the meters of the row of meters.
	Meters []Meter
	// Ports are the ports the session listens on.
	Ports string
	// CPU is how busy the machine is.
	CPU float32
	// Memory is how much of the memory is used.
	Memory float32
	// Date is the time, as the row of meters says it.
	Date string
}

// roleColor returns the color of a role of the text of the terminal.
func roleColor(t design.Tokens, role Role) ui.Color {
	switch role {
	case RolePrimary:
		return t.TextPrimary
	case RoleSecondary:
		return t.TextSecondary
	case RoleGreen:
		return t.GreenText
	case RoleYellow:
		return t.YellowText
	case RoleRed:
		return t.RedText
	}
	return t.TextPrimary
}

package design

// The measures of the window.
//
// The numbers here are the ones the design file's boards use. A measure that
// is not in this file is not a measure of the window.

// The type scale.
//
// The window has seven sizes of text. The meter row and the badges of the
// activity bar use a smaller one, of ten, that no other board has.
const (
	// FontMicro is the size of the text of the meters and of the badges.
	FontMicro = 10
	// FontSmall is the size of the least text of the window.
	FontSmall = 11
	// FontBody is the size of the secondary text of the window.
	FontBody = 12
	// FontUI is the size of the text of the window: the names of the
	// workspaces, the labels of the tabs.
	FontUI = 13
	// FontMedium is the size of the text of the smaller panels.
	FontMedium = 15
	// FontLarge is the size of the text of the larger panels.
	FontLarge = 17
	// FontTitle is the size of the title of a panel.
	FontTitle = 20
	// FontDisplay is the size of the title of the window.
	FontDisplay = 28
)

// The weights of the text.
//
// The design file makes the names of the workspaces and of the agents bold
// when they are the ones that are chosen.
const (
	// WeightNormal is the weight of the text that says nothing in
	// particular.
	WeightNormal = 400
	// WeightMedium is the weight of the label of a section.
	WeightMedium = 600
	// WeightBold is the weight of the text that matters most: the name
	// of the workspace that is chosen, the version of a tool.
	WeightBold = 700
)

// The spacing of the window.
//
// Eight steps, and nothing between them.
const (
	// SpaceXS is the space inside a tree.
	SpaceXS = 2
	// SpaceSM is the space between the rows of a tree.
	SpaceSM = 4
	// SpaceMD is the space inside a row.
	SpaceMD = 6
	// SpaceLG is the space between the parts of a row.
	SpaceLG = 8
	// SpaceXL is the space at the side of a section.
	SpaceXL = 12
	// Space2XL is the space between the three columns of the window.
	Space2XL = 16
	// Space3XL is the space around a card.
	Space3XL = 24
	// Space4XL is the space around the window.
	Space4XL = 32
)

// The corners of the window.
//
// Eight steps, from the square corner of a bar to the round corner of the
// card in the middle of the window.
const (
	// RadiusNone is the square corner.
	RadiusNone = 0
	// RadiusXS rounds the corners of a meter.
	RadiusXS = 2
	// RadiusSM rounds the corners of a small control.
	RadiusSM = 4
	// RadiusMD rounds the corners of the row that is chosen and of a tab.
	RadiusMD = 6
	// RadiusLG rounds the corners of a button of the activity bar.
	RadiusLG = 8
	// RadiusXL rounds the corners of a badge.
	RadiusXL = 10
	// Radius2XL rounds the corners of a panel.
	Radius2XL = 12
	// Radius3XL rounds the corners of the card in the middle of the
	// window.
	Radius3XL = 16
)

// The strokes of the window.
//
// A hairline for everything, and a line of two for what is chosen.
const (
	// StrokeNormal is the stroke of a border.
	StrokeNormal = 1
	// StrokeStrong is the stroke of what is chosen.
	StrokeStrong = 2
)

// The measures of the parts of the window.
const (
	// TitleBarHeight is the height of the bar at the top of the window.
	TitleBarHeight = 34
	// RailWidth is the width of the column of views at the left.
	RailWidth = 44
	// SidebarWidth is the width of the sidebar.
	SidebarWidth = 248

	// TabStripHeight is the height of the strip of the tabs.
	TabStripHeight = 36
	// TabHeight is the height of a tab.
	TabHeight = 24
	// MeterRowHeight is the height of the row of the meters.
	MeterRowHeight = 26

	// RailButton is the size of a button of the activity bar.
	RailButton = 32
	// RailIcon is the size of the icon of a button of the activity bar.
	RailIcon = 17
	// RailTop is the space above the first button of the activity bar.
	RailTop = 36
	// BadgeHeight is the height of the badge of the activity bar.
	BadgeHeight = 14
	// BadgeRadius rounds the corners of a badge.
	BadgeRadius = 8

	// IconSmall is the size of the icon in a row of a tree.
	IconSmall = 14
	// IconMedium is the size of the icon of a section and of the title
	// bar.
	IconMedium = 16

	// SidebarHeaderHeight is the height of the head of the sidebar.
	SidebarHeaderHeight = 34
	// SidebarSectionHeight is the height of the head of a section.
	SidebarSectionHeight = 28
	// SidebarFooterHeight is the height of the foot of the sidebar.
	SidebarFooterHeight = 30
	// SidebarDividerHeight is the height of the line between the
	// sections of the sidebar.
	SidebarDividerHeight = 1
	// TreeIndent is the width of the space a child row stands back.
	TreeIndent = 22
	// AgentStateIndent is the width of the space the state of an agent
	// stands back.
	AgentStateIndent = 40

	// WorkspaceRowHeight is the height of the row of a workspace.
	WorkspaceRowHeight = 24
	// BranchRowHeight is the height of the row of a branch.
	BranchRowHeight = 19
	// PortsRowHeight is the height of the row of the ports.
	PortsRowHeight = 18
	// AgentRowHeight is the height of the row of an agent.
	AgentRowHeight = 20
	// GroupRowHeight is the height of the row of a group of agents.
	GroupRowHeight = 22
	// StateRowHeight is the height of the row of the state of an agent.
	StateRowHeight = 18

	// TerminalLineHeight is the height of a line of the terminal.
	TerminalLineHeight = 19
	// TerminalFontSize is the size of the characters of the terminal.
	TerminalFontSize = 13
	// TerminalPadY is the space above and below the terminal.
	TerminalPadY = 16
	// TerminalPadX is the space at the side of the terminal.
	TerminalPadX = 18

	// MeterBarWidth is the width of the bar of a meter.
	MeterBarWidth = 28
	// MeterBarHeight is the height of the bar of a meter.
	MeterBarHeight = 4
	// CPUBarWidth is the width of the bar of the processor.
	CPUBarWidth = 44
	// CPUBarHeight is the height of the bar of the processor.
	CPUBarHeight = 4

	// BodyPadTop is the space above the body of the window.
	BodyPadTop = 10
	// BodyPadRight is the space at the right of the body of the window.
	BodyPadRight = 14
	// BodyPadBottom is the space below the body of the window.
	BodyPadBottom = 14
	// BodyGap is the space between the three columns of the window.
	BodyGap = Space2XL

	// WindowRadius rounds the corners of the window.
	WindowRadius = 12
)

// The fonts of the window.
//
// The design file names Inter and JetBrains Mono. Neither is certain to be
// on the machine, so both come with a list to fall back on.
const (
	// FamilyUI is the family of the text of the window. The design file
	// calls it font-ui.
	FamilyUI = "Inter, Helvetica Neue, Arial, sans-serif"
	// FamilyMono is the family of the characters of the terminal, of the
	// numbers of the ports and of the numbers of the tabs. The design
	// file calls it font-mono.
	FamilyMono = "JetBrains Mono, Menlo, SF Mono, monospace"
)

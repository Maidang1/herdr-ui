package ui

import (
	"herdr-ui/internal/design"
	"herdr-ui/internal/ui/widgets"

	"github.com/egoist/mygo/ui"
)

// Panel is the view of the activity bar that is chosen.
type Panel int

const (
	// PanelWorkspaces is the tree of the workspaces.
	PanelWorkspaces Panel = iota
	// PanelOverview is the list of what wants attention.
	PanelOverview
	// PanelPullRequests is the list of the pull requests.
	PanelPullRequests
	// PanelIssues is the list of the issues.
	PanelIssues
	// PanelSecurity is the list of the advisories.
	PanelSecurity
	// PanelUsage is the list of the quotas.
	PanelUsage
)

// String names the panel, as the name of the icon of it in the design.
func (p Panel) String() string {
	switch p {
	case PanelOverview:
		return "overview"
	case PanelPullRequests:
		return "pull-requests"
	case PanelIssues:
		return "issues"
	case PanelSecurity:
		return "security"
	case PanelUsage:
		return "usage"
	}
	return "workspaces"
}

// railButton is a button of the activity bar.
type railButton struct {
	// Panel is the view the button opens.
	Panel Panel
	// Label is the name of the button, for the screen reader.
	Label string
	// Icon is the name of the icon of the button.
	Icon string
	// Count is the number on the badge of the button.
	Count int
}

// The buttons of the activity bar, in the order of the design.
var railButtons = []railButton{
	{PanelWorkspaces, "Workspaces", "terminal", 0},
	{PanelOverview, "Overview", "activity", 3},
	{PanelPullRequests, "Pull requests", "git-pull-request", 12},
	{PanelIssues, "Issues", "circle-dot", 4},
	{PanelSecurity, "Security", "shield", 2},
	{PanelUsage, "Usage", "chart-column", 0},
}

// rail draws the column of views at the left of the window.
//
// The design file's rule is that the activity bar stands on the gradient of
// the window with nothing behind it, and that only the view that is chosen
// has a face of its own.
func (a *App) rail(c *ui.Context, t design.Tokens) {
	ui.Column(c).
		Width(design.RailWidth).
		Padding(design.RailTop, 0, 0, design.SpaceMD).
		Gap(design.SpaceSM).
		Children(func() {
			for _, button := range railButtons {
				a.railButton(c, t, button)
			}
			// The settings are at the bottom of the column, far from
			// the views.
			ui.Row(c).Grow(1)
			a.railButton(c, t, railButton{Panel: Panel(-1), Label: "Settings", Icon: "settings"})
		})
}

// railButton draws one button of the activity bar.
func (a *App) railButton(c *ui.Context, t design.Tokens, button railButton) {
	chosen := button.Panel == a.panel
	iconColor := t.TextTertiary
	if chosen {
		iconColor = t.TextPrimary
	}
	ui.Row(c).
		Size(design.RailButton, design.RailButton).
		Radius(design.RadiusLG).
		AlignItems(ui.Center).
		Justify(ui.Center).
		OnClick(func() {
			if button.Panel >= 0 {
				a.panel = button.Panel
			}
		}).
		Children(func() {
			if chosen {
				ui.Row(c).Size(design.RailButton, design.RailButton).Radius(design.RadiusLG).Background(t.Selection)
			}
			// The badge sits at the top right of the icon and hangs
			// over the edge of the button.
			ui.Row(c).
				AlignItems(ui.Start).
				Children(func() {
					widgets.Icon(c, t, button.Icon, design.RailIcon, iconColor)
					if button.Count > 0 {
						ui.Row(c).
							MarginStart(-3).
							Height(design.BadgeHeight).
							Background(t.AccentDeep).
							Radius(design.BadgeRadius).
							Padding(0, design.SpaceXS+1).
							AlignItems(ui.Center).
							Children(func() {
								widgets.Label(c, t, itoa(button.Count), widgets.LabelOptions{
									Size:  design.FontMicro,
									Color: ui.Hex("#FFFFFF"),
								})
							})
					}
				})
		})
}

// itoa writes a number the way the window writes it.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}

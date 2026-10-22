// Package ui draws the window of the client.
//
// The window is the design of the file docs.pen/01.pen: an activity bar at
// the left, the tree of the workspaces beside it, and the card of the
// terminal in the middle, with the strip of the tabs and the row of the
// meters under it.
package ui

import (
	"time"

	"herdr-ui/internal/design"
	"herdr-ui/internal/ui/widgets"

	"github.com/egoist/mygo/ui"
)

// App is the window of the client.
type App struct {
	// mode is the mode of the window: the light one, the dark one, or
	// the one of the system.
	mode Mode
	// panel is the view of the activity bar that is chosen.
	panel Panel
	// live is the session, as the daemon tells it.
	live *Live
	// session is the sample session, shown when the daemon is not there.
	session Session
	// systemDark is what the system answered when it was asked whether
	// it is in the dark mode.
	systemDark bool
	// systemDarkAt is when the system was asked last.
	systemDarkAt time.Time
}

// New returns a window that shows the session of the design.
//
// The session is a sample one for now: the data of the real session comes
// from the daemon, and this is where it will be put.
func New() *App {
	return &App{
		mode:    ModeSystem,
		panel:   PanelWorkspaces,
		live:    Connect(),
		session: sampleSession(),
	}
}

// NewSample returns a window that shows the sample session.
//
// It is the window of the drawings and of the tests: it is not attached
// to the daemon.
func NewSample() *App {
	return &App{
		mode:    ModeSystem,
		panel:   PanelWorkspaces,
		live:    &Live{},
		session: sampleSession(),
	}
}

// Live returns the session, as the daemon tells it.
func (a *App) Live() *Live { return a.live }

// SetMode sets the mode of the window.
//
// The mode is the light one, the dark one, or the one of the system.
func (a *App) SetMode(mode Mode) {
	a.mode = mode
}

// Mode returns the mode of the window.
func (a *App) Mode() Mode {
	return a.mode
}

// tokens returns the tokens of the mode the window is in.
func (a *App) tokens() design.Tokens {
	if a.mode.Dark(a.systemIsDark()) {
		return design.Dark()
	}
	return design.Light()
}

// systemIsDark asks the system whether it is in the dark mode.
//
// The question is worth asking once every few seconds: the system is a
// process of its own, and the window is drawn often.
func (a *App) systemIsDark() bool {
	now := time.Now()
	if a.systemDarkAt.IsZero() || now.Sub(a.systemDarkAt) > 2*time.Second {
		a.systemDark = systemIsDark()
		a.systemDarkAt = now
	}
	return a.systemDark
}

// View draws the window.
func (a *App) View(c *ui.Context) {
	t := a.tokens()
	c.SetTheme(t.MygoTheme())
	gradient := t.WindowGradient()
	stops := make([]ui.GradientStop, 0, len(gradient.Stops))
	for _, stop := range gradient.Stops {
		stops = append(stops, ui.GradientStop{Color: stop.Color, At: stop.At})
	}
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyD, func() {
		a.mode = a.mode.next()
	})
	// The daemon pushes what changes; the window looks at it again and
	// again, and draws it.
	if a.live.Connected() {
		c.After(Tick)
	}
	ui.Column(c).
		Fill().
		LinearGradient(ui.LinearGradient{Angle: gradient.Angle, Stops: stops}).
		Border(design.StrokeNormal, t.WindowFrameBorder()).
		Radius(design.WindowRadius).
		Children(func() {
			a.titleBar(c, t)
			a.body(c, t)
		})
}

// titleBar draws the bar at the top of the window.
//
// The room the window controls take is left to them, and the bar itself
// drags the window.
func (a *App) titleBar(c *ui.Context, t design.Tokens) {
	bar := c.TitleBar()
	height := bar.Height
	if height < design.TitleBarHeight {
		height = design.TitleBarHeight
	}
	ui.Row(c).
		Height(height).
		Padding(0, bar.Right, 0, bar.Left).
		Background(t.Titlebar).
		AlignItems(ui.Center).
		DragWindow().
		Children(func() {
			// The button that hides the sidebar.
			ui.Row(c).
				Size(28, 28).
				Radius(design.RadiusLG).
				AlignItems(ui.Center).
				Justify(ui.Center).
				Children(func() {
					widgets.Icon(c, t, "panel-left", design.IconMedium, t.TextSecondary)
				})
			// The arrows back and forward.
			ui.Row(c).
				Size(48, 24).
				Radius(design.RadiusMD).
				AlignItems(ui.Center).
				Children(func() {
					ui.Row(c).
						Size(24, 24).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "chevron-left", design.IconMedium, t.TextSecondary)
						})
					ui.Row(c).
						Size(24, 24).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "chevron-right", design.IconMedium, t.TextTertiary)
						})
				})
			ui.Row(c).Grow(1)
			widgets.Label(c, t, "Herdr", widgets.LabelOptions{
				Size:  design.FontUI,
				Color: t.TextSecondary,
			})
			ui.Row(c).Grow(1)
			// The account.
			ui.Row(c).
				Size(28, 28).
				AlignItems(ui.Center).
				Justify(ui.Center).
				Children(func() {
					ui.Row(c).
						Size(24, 24).
						Radius(design.RadiusXL).
						Background(t.Chip).
						AlignItems(ui.Center).
						Justify(ui.Center).
						Children(func() {
							widgets.Icon(c, t, "github", design.IconMedium, t.TextTertiary)
						})
				})
		})
}

// body draws the three columns of the window.
func (a *App) body(c *ui.Context, t design.Tokens) {
	ui.Row(c).
		Grow(1).
		Padding(design.BodyPadTop, design.BodyPadRight, design.BodyPadBottom, 0).
		Gap(design.BodyGap).
		AlignItems(ui.Stretch).
		Children(func() {
			a.rail(c, t)
			a.sidebar(c, t)
			a.mainCard(c, t)
		})
}

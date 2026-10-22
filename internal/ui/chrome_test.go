package ui

import (
	"image"
	"testing"

	"herdr-ui/internal/herdr"
	"herdr-ui/internal/palette"
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo/ui"
)

// The window is drawn without a screen here: the tester lays the view out
// in memory and hands back the picture, so the test can look at the colors
// and the places the chrome stands in.

// renderWindow draws the window and hands back the picture and the tester.
func renderWindow(t *testing.T) (*image.RGBA, *ui.Tester) {
	t.Helper()
	app := testApp()
	app.snapshot.Workspaces = append(app.snapshot.Workspaces, herdr.Workspace{
		WorkspaceID: "w2",
		Number:      2,
		Label:       "coze-loop-frontend",
		Branch:      stringPtr("release/20261021"),
	})
	app.snapshot.Agents = append(app.snapshot.Agents, herdr.Agent{
		PaneID:      "w1:p2",
		WorkspaceID: "w1",
		Agent:       stringPtr("codex"),
		AgentStatus: "idle",
	})
	tester := ui.NewTester(app.view, 1400, 860)
	tester.SetDark(true)
	tester.Frame()
	return tester.Image(), tester
}

// colorAt returns the color of a point of the picture.
func colorAt(picture *image.RGBA, x, y int) ui.Color {
	if x < 0 || y < 0 || x >= picture.Bounds().Dx() || y >= picture.Bounds().Dy() {
		return ui.RGBA(0, 0, 0, 0)
	}
	pixel := picture.RGBAAt(x, y)
	return ui.RGB(pixel.R, pixel.G, pixel.B)
}

func TestTheChromeStandsWhereTheRustClientPutsIt(t *testing.T) {
	app := testApp()
	picture, tester := renderWindow(t)
	colors := app.newChrome(true)

	// The rail of the views stands at the left of the window, the sidebar
	// beside it, and the terminal beside that.
	if got := colorAt(picture, 10, 400); got != colors.panel {
		t.Fatalf("the rail came out as %v, want %v", got, colors.panel)
	}
	if got := colorAt(picture, 200, 400); got != colors.sidebar {
		t.Fatalf("the sidebar came out as %v, want %v", got, colors.sidebar)
	}
	if got := colorAt(picture, 900, 400); got != colors.background {
		t.Fatalf("the terminal came out as %v, want %v", got, colors.background)
	}
	// The bar of the session stands at the bottom.
	if got := colorAt(picture, 900, 855); got != colors.panel {
		t.Fatalf("the bar of the session came out as %v, want %v", got, colors.panel)
	}
	// The rail ends before the sidebar, and the sidebar before the terminal.
	// The head of the sidebar stands at the width of the rail and the
	// padding of a row.
	head, ok := tester.Find("WORKSPACES")
	if !ok {
		t.Fatalf("the sidebar has no head; it shows %q", tester.Texts())
	}
	if head.X < railWidth+rowPadding-2 || head.X > railWidth+rowPadding+4 {
		t.Fatalf("the head of the sidebar stands at %v, want about %v", head.X, railWidth+rowPadding)
	}
}

func TestTheWindowShowsThePartsOfTheRustClient(t *testing.T) {
	_, tester := renderWindow(t)
	for _, want := range []string{
		"WORKSPACES",
		"spaces",
		"agents",
		"grouped",
		"new",
		"menu",
		"CPU",
		"MEM",
		"dsh",
		"coze-loop-frontend",
		"release/20261021",
	} {
		if !tester.HasText(want) {
			t.Errorf("the window does not show %q; it shows %q", want, tester.Texts())
		}
	}
}

func TestTheColorsOfTheWindowComeFromThePaletteOfTheDaemon(t *testing.T) {
	app := testApp()
	colors := app.newChrome(true)
	// The daemon keeps its theme in its own configuration, and the window
	// paints itself with it.
	name, err := theme.HerdrThemeName(false)
	if err != nil || name == "" {
		t.Skipf("the theme of the daemon cannot be read: %v", err)
	}
	found, ok := palette.ByName(name)
	if !ok {
		t.Skipf("the palette %q is not one the window knows", name)
	}
	if colors.text != colorOf(found.Text) {
		t.Fatalf("the text came out as %v, want %v", colors.text, colorOf(found.Text))
	}
	if colors.accent != colorOf(found.Accent) {
		t.Fatalf("the accent came out as %v, want %v", colors.accent, colorOf(found.Accent))
	}
}

func TestThePaletteOfTheDaemonIsTheOneItUses(t *testing.T) {
	// The palettes are the ones of the daemon, with the same colors.
	found, ok := palette.ByName("gruvbox")
	if !ok {
		t.Fatal("the palette of Gruvbox is missing")
	}
	if found.Accent != palette.RGB(215, 153, 33) {
		t.Fatalf("the accent of Gruvbox came out as %v", found.Accent)
	}
	if found.Text != palette.RGB(235, 219, 178) {
		t.Fatalf("the text of Gruvbox came out as %v", found.Text)
	}
	if _, ok := palette.ByName("no-such-palette"); ok {
		t.Fatal("a palette that is not there was found")
	}
}

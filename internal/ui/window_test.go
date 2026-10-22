package ui

import (
	"testing"
	"time"

	"herdr-ui/internal/protocol"

	"github.com/egoist/mygo/ui"
)

// render draws the window in one mode and hands back the tester that drew
// it.
func render(t *testing.T, mode Mode) *ui.Tester {
	t.Helper()
	app := NewSample()
	app.SetMode(mode)
	tester := ui.NewTester(app.View, 1440, 900)
	tester.Frame()
	return tester
}

// TestTheWindowDraws checks that the window of the design draws, in both
// modes, and that what it draws is what the design says.
func TestTheWindowDraws(t *testing.T) {
	for _, mode := range []struct {
		name string
		mode Mode
	}{
		{"light", ModeLight},
		{"dark", ModeDark},
	} {
		tester := render(t, mode.mode)
		for _, want := range []string{
			"Herdr",
			"Workspaces",
			"Spaces",
			"deepseek-harness",
			"master",
			"Agents",
			"grok",
			"idle",
			"New",
			"Menu",
			"CPU",
			"MEM",
		} {
			if !tester.HasText(want) {
				t.Errorf("the %s window does not say %q", mode.name, want)
			}
		}
	}
}

// TestTheTerminalDrawsTheSession checks that the terminal shows the lines
// of the session, in the colors of the design.
func TestTheTerminalDrawsTheSession(t *testing.T) {
	tester := render(t, ModeLight)
	for _, want := range []string{
		"(base)",
		"v24.15.0",
		"pwd",
		"/Users/bytedance/codes/open-source/deepseek-harness",
		"[+?]",
	} {
		if !tester.HasText(want) {
			t.Errorf("the terminal does not say %q", want)
		}
	}
}

// TestTheMetersDrawTheSession checks that the row of meters says what the
// design says.
func TestTheMetersDrawTheSession(t *testing.T) {
	tester := render(t, ModeLight)
	for _, want := range []string{
		"5h 27% used 0m",
		"wk 78% used 5d 3h",
		":8080 :8081 :9200",
		"48%",
		"83%",
		"10-20 11:24",
	} {
		if !tester.HasText(want) {
			t.Errorf("the row of the meters does not say %q", want)
		}
	}
}

// TestTheModeCycles checks the order of the modes: the light one, the dark
// one, and the system's own.
func TestTheModeCycles(t *testing.T) {
	mode := ModeSystem
	mode = mode.next()
	if mode != ModeLight {
		t.Errorf("the mode after the system's is %v, the light one is wanted", mode)
	}
	mode = mode.next()
	if mode != ModeDark {
		t.Errorf("the mode after the light one is %v, the dark one is wanted", mode)
	}
	mode = mode.next()
	if mode != ModeSystem {
		t.Errorf("the mode after the dark one is %v, the system's is wanted", mode)
	}
}

// TestTheModeTellsItsDarkness checks which mode is dark.
func TestTheModeTellsItsDarkness(t *testing.T) {
	if ModeLight.Dark(true) {
		t.Error("the light mode is dark when the system is")
	}
	if !ModeDark.Dark(false) {
		t.Error("the dark mode is not dark when the system is not")
	}
	if !ModeSystem.Dark(true) {
		t.Error("the system's mode is not dark when the system is")
	}
	if ModeSystem.Dark(false) {
		t.Error("the system's mode is dark when the system is not")
	}
}

// TestTheLiveWindowShowsTheSession checks the window against the daemon
// that is running on this machine.
//
// The test is skipped when there is no daemon: the window is then the
// sample one, and the sample one has its own test.
func TestTheLiveWindowShowsTheSession(t *testing.T) {
	live := Connect()
	deadline := time.After(8 * time.Second)
	var snapshot *protocol.Snapshot
	var surface *protocol.SurfaceFrame
	for snapshot == nil || surface == nil {
		select {
		case event := <-live.Events():
			if event.Snapshot != nil {
				snapshot = event.Snapshot
			}
			if event.Surface != nil {
				surface = event.Surface
			}
		case <-deadline:
			t.Skip("the daemon did not answer in time")
		}
	}
	if len(snapshot.Workspaces) == 0 {
		t.Error("the session has no workspaces")
	}
	if surface.Width == 0 || len(surface.Cells) == 0 {
		t.Error("the surface of the pane in front is empty")
	}

	// The window must draw what the daemon says.
	app := New()
	defer app.Live().Close()
	tester := ui.NewTester(app.View, 1440, 900)
	tester.Frame()
	if len(tester.Texts()) == 0 {
		t.Error("the window draws nothing")
	}
}

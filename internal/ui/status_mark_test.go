package ui

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

func statusMarkTester(value string, reduce bool) *native.Tester {
	tester := native.NewTester(func(c *native.Context) {
		native.Box(c).Fill().Center().Background(design.Colors(false).Content).Children(func() {
			statusMark(c, "test", value, 40, design.Colors(false))
		})
	}, 60, 60)
	tester.SetPreferences(native.Preferences{ReduceMotion: reduce})
	tester.Frame()
	return tester
}

func encoded(t *testing.T, tester *native.Tester) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, tester.Image()); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestWorkingAndWaitingMarksAnimateUnlessMotionIsReduced(t *testing.T) {
	for _, value := range []string{"working", "blocked"} {
		moving := statusMarkTester(value, false)
		before := encoded(t, moving)
		moving.Advance(300 * time.Millisecond)
		moving.Frame()
		if bytes.Equal(before, encoded(t, moving)) {
			t.Errorf("%s mark does not move", value)
		}
		still := statusMarkTester(value, true)
		before = encoded(t, still)
		still.Advance(300 * time.Millisecond)
		still.Frame()
		if !bytes.Equal(before, encoded(t, still)) {
			t.Errorf("%s mark moves despite reduced motion", value)
		}
	}
	for _, value := range []string{"done", "idle", "unknown"} {
		tester := statusMarkTester(value, false)
		before := encoded(t, tester)
		tester.Advance(300 * time.Millisecond)
		tester.Frame()
		if !bytes.Equal(before, encoded(t, tester)) {
			t.Errorf("settled %s mark moves", value)
		}
	}
}

func TestStatusMarkRenderArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export status marks")
	}
	for _, dark := range []bool{false, true} {
		tester := native.NewTester(func(c *native.Context) {
			p := design.Colors(dark)
			native.Row(c).Fill().Padding(16).Gap(24).AlignItems(native.Center).Background(p.Content).Children(func() {
				for _, value := range []string{"working", "blocked", "done", "idle", "unknown"} {
					native.Column(c).Gap(8).AlignItems(native.Center).Children(func() {
						statusMark(c, value, value, 10, p)
						statusMark(c, value+"-large", value, 32, p)
						native.Text(c, value).FontSize(11).TextColor(p.Secondary)
					})
				}
			})
		}, 420, 120)
		tester.SetScale(2)
		tester.SetDark(dark)
		tester.Frame()
		tester.Advance(400 * time.Millisecond)
		tester.Frame()
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("status-marks-dark-%t.png", dark)))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tester.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

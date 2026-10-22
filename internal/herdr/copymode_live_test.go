package herdr

import (
	"context"
	"os"
	"testing"
	"time"
)

// A scratch check of the copy-mode calls the window makes.
func TestLiveCopyModeCalls(t *testing.T) {
	if os.Getenv("HERDR_UI_LIVE") == "" {
		t.Skip("set HERDR_UI_LIVE=1")
	}
	paneID := os.Getenv("HERDR_UI_PANE")
	if paneID == "" {
		t.Skip("set HERDR_UI_PANE")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{CellWidthPx: 8, CellHeightPx: 17, Cols: 120, Rows: 40, SurfaceActive: true})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer endpoint.Close()
	select {
	case <-endpoint.Snapshots():
	case <-time.After(15 * time.Second):
		t.Fatal("no snapshot")
	}
	if err := endpoint.Focus(true); err != nil {
		t.Fatalf("focus: %v", err)
	}
	defer endpoint.Focus(false)

	// Some text to find.
	for _, line := range []string{"echo herdr-ui-copy", "echo second line"} {
		if err := endpoint.SendText(paneID, line); err != nil {
			t.Fatalf("send text: %v", err)
		}
		if err := endpoint.SendKey(paneID, PaneKey{Code: KeyEnter}); err != nil {
			t.Fatalf("send key: %v", err)
		}
		time.Sleep(600 * time.Millisecond)
	}
	time.Sleep(2 * time.Second)

	// The revision of the pane's content, which the daemon asks for.
	var set map[string]any
	if err := endpoint.Request(ctx, "client_shell.surface.set", map[string]any{"active": true}, &set); err != nil {
		t.Fatalf("surface.set: %v", err)
	}
	var revision uint64
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) && revision == 0 {
		select {
		case frame := <-endpoint.Surfaces():
			for _, pane := range frame.Panes {
				if pane.PaneID == paneID {
					revision = pane.ContentRevision
				}
			}
		case <-time.After(time.Second):
		}
	}
	t.Logf("the revision of the pane is %d", revision)
	if revision == 0 {
		t.Fatal("the surface holds no revision")
	}

	// The search of the copy mode.
	var search struct {
		Type    string `json:"type"`
		Matches []struct {
			Start struct {
				Row int `json:"row"`
				Col int `json:"col"`
			} `json:"start"`
			End struct {
				Row int `json:"row"`
				Col int `json:"col"`
			} `json:"end"`
		} `json:"matches"`
		Total   int `json:"total"`
		Current int `json:"current"`
	}
	for attempt := 0; attempt < 6; attempt++ {
		err = endpoint.Request(ctx, "pane.copy_search", map[string]any{
			"pane_id":          paneID,
			"query":            "herdr-ui-copy",
			"direction":        "forward",
			"cursor":           map[string]any{"row": 0, "col": 0},
			"content_revision": revision,
		}, &search)
		if err == nil {
			break
		}
		t.Logf("attempt %d: %v", attempt, err)
		time.Sleep(300 * time.Millisecond)
		select {
		case frame := <-endpoint.Surfaces():
			for _, pane := range frame.Panes {
				if pane.PaneID == paneID {
					revision = pane.ContentRevision
				}
			}
		default:
		}
	}
	if err != nil {
		t.Fatalf("pane.copy_search: %v", err)
	}
	t.Logf("search: %+v", search)
	if search.Total == 0 {
		t.Fatal("the search found nothing")
	}

	// The motion of the copy mode.
	var motion struct {
		Type   string `json:"type"`
		Cursor struct {
			Row int `json:"row"`
			Col int `json:"col"`
		} `json:"cursor"`
	}
	if err := endpoint.Request(ctx, "pane.copy_motion", map[string]any{
		"pane_id": paneID,
		"cursor":  map[string]any{"row": search.Matches[0].Start.Row, "col": search.Matches[0].Start.Col},
		"motion":  "next_word_start",
	}, &motion); err != nil {
		t.Fatalf("pane.copy_motion: %v", err)
	}
	t.Logf("motion: %+v", motion)

	// The reading of the selection.
	var read struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := endpoint.Request(ctx, "pane.selection.read", map[string]any{
		"pane_id": paneID,
		"anchor":  map[string]any{"row": search.Matches[0].Start.Row, "col": search.Matches[0].Start.Col},
		"cursor":  map[string]any{"row": search.Matches[0].End.Row, "col": search.Matches[0].End.Col},
	}, &read); err != nil {
		t.Fatalf("pane.selection.read: %v", err)
	}
	t.Logf("selection: %q", read.Text)
	if read.Text == "" {
		t.Fatal("the selection read nothing")
	}
}

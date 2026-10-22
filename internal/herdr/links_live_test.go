package herdr

import (
	"context"
	"os"
	"testing"
	"time"
)

// A scratch check of the link calls the window makes.
func TestLiveLinkCalls(t *testing.T) {
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

	// A web address in the output.
	if err := endpoint.SendText(paneID, "echo https://example.com/herdr-ui"); err != nil {
		t.Fatalf("send text: %v", err)
	}
	if err := endpoint.SendKey(paneID, PaneKey{Code: KeyEnter}); err != nil {
		t.Fatalf("send key: %v", err)
	}
	time.Sleep(3 * time.Second)

	// The revision of the pane, which the daemon asks for.
	var set map[string]any
	if err := endpoint.Request(ctx, "client_shell.surface.set", map[string]any{"active": true}, &set); err != nil {
		t.Fatalf("surface.set: %v", err)
	}
	var revision uint64
	var frame *SurfaceFrame
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case frame = <-endpoint.Surfaces():
			for _, pane := range frame.Panes {
				if pane.PaneID == paneID {
					revision = pane.ContentRevision
				}
			}
			t.Logf("a frame with %d panes, revision %d", len(frame.Panes), revision)
			for _, pane := range frame.Panes {
				t.Logf("  pane %s revision %d", pane.PaneID, pane.ContentRevision)
			}
		case <-time.After(time.Second):
		}
		if revision > 0 {
			break
		}
	}
	t.Logf("the revision of the pane is %d", revision)

	// Where the address stands in the pane.
	row, column := -1, -1
	if frame != nil {
		width := int(frame.Grid.Width)
		for r := 0; r < int(frame.Grid.Height); r++ {
			for c := 0; c < width; c++ {
				index := r*width + c
				if index >= len(frame.Grid.Cells) {
					break
				}
				if frame.Grid.Cells[index].Symbol == "h" {
					row, column = r, c
					break
				}
			}
			if row >= 0 {
				break
			}
		}
	}
	t.Logf("the address stands at row %d column %d", row, column)
	if row < 0 {
		t.Skip("the address is not on the surface")
	}

	// The reading of the link.
	var resolved struct {
		Type    string       `json:"type"`
		Regions []linkRegion `json:"regions"`
	}
	for attempt := 0; attempt < 6; attempt++ {
		err = endpoint.Request(ctx, "pane.link.resolve", map[string]any{
			"pane_id":          paneID,
			"viewport_row":     row,
			"col":              column,
			"content_revision": revision,
		}, &resolved)
		if err == nil {
			break
		}
		t.Logf("attempt %d: %v", attempt, err)
		time.Sleep(400 * time.Millisecond)
		select {
		case frame = <-endpoint.Surfaces():
			for _, pane := range frame.Panes {
				if pane.PaneID == paneID {
					revision = pane.ContentRevision
				}
			}
		default:
		}
	}
	if err != nil {
		t.Fatalf("pane.link.resolve: %v", err)
	}
	t.Logf("resolved: %+v", resolved)

	// The opening of the link.
	var activated struct {
		Type    string  `json:"type"`
		Handled bool    `json:"handled"`
		URL     *string `json:"url"`
	}
	for attempt := 0; attempt < 6; attempt++ {
		err = endpoint.Request(ctx, "pane.link.activate", map[string]any{
			"pane_id":          paneID,
			"viewport_row":     row,
			"col":              column,
			"content_revision": revision,
		}, &activated)
		if err == nil {
			break
		}
		t.Logf("attempt %d: %v", attempt, err)
		time.Sleep(400 * time.Millisecond)
		select {
		case frame = <-endpoint.Surfaces():
			for _, pane := range frame.Panes {
				if pane.PaneID == paneID {
					revision = pane.ContentRevision
				}
			}
		default:
		}
	}
	if err != nil {
		t.Fatalf("pane.link.activate: %v", err)
	}
	t.Logf("activated: %+v", activated)
}

type linkRegion struct {
	Row      int `json:"row"`
	StartCol int `json:"start_col"`
	EndCol   int `json:"end_col"`
}

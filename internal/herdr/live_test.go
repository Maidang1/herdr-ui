package herdr

import (
	"context"
	"os"
	"testing"
	"time"
)

// The live test talks to the daemon the machine runs. It is skipped unless
// HERDR_UI_LIVE is set, so the ordinary test run stays hermetic.
func TestLiveRequests(t *testing.T) {
	if os.Getenv("HERDR_UI_LIVE") == "" {
		t.Skip("set HERDR_UI_LIVE=1 to talk to the local daemon")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{
		CellWidthPx: 8, CellHeightPx: 17, Cols: 120, Rows: 40, SurfaceActive: true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer endpoint.Close()

	var snapshot *Snapshot
	select {
	case snapshot = <-endpoint.Snapshots():
	case <-ctx.Done():
		t.Fatal("no snapshot arrived")
	}

	// Asking the daemon to show the surfaces is what a client does once it
	// is ready.
	var result map[string]any
	if err := endpoint.Request(ctx, "client_shell.surface.set", map[string]any{"active": true}, &result); err != nil {
		t.Fatalf("client_shell.surface.set: %v", err)
	}
	if result["type"] != "client_shell_surface_set" {
		t.Fatalf("the daemon answered %v", result)
	}

	// Focusing the tab that is already focused changes nothing, and shows
	// that the daemon serves the call.
	if tab, ok := snapshot.FocusedTab(); ok {
		var focus map[string]any
		if err := endpoint.Request(ctx, "tab.focus", map[string]any{"tab_id": tab.TabID}, &focus); err != nil {
			t.Fatalf("tab.focus: %v", err)
		}
		t.Logf("the daemon answered %v", focus)
	}
}

func TestLiveHandshakeAndSnapshot(t *testing.T) {
	if os.Getenv("HERDR_UI_LIVE") == "" {
		t.Skip("set HERDR_UI_LIVE=1 to talk to the local daemon")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	endpoint, err := Connect(ctx, EndpointOptions{
		CellWidthPx:   8,
		CellHeightPx:  17,
		Cols:          120,
		Rows:          40,
		SurfaceActive: true,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer endpoint.Close()

	welcome := endpoint.Welcome()
	if welcome == nil {
		t.Fatal("no welcome")
	}
	t.Logf("welcome: server=%s generation=%d methods=%d capabilities=%v",
		welcome.ServerVersion, welcome.Generation, len(welcome.Methods), welcome.Capabilities)
	if welcome.Generation != EndpointProtocolGeneration {
		t.Fatalf("generation %d, want %d", welcome.Generation, EndpointProtocolGeneration)
	}

	select {
	case snapshot := <-endpoint.Snapshots():
		t.Logf("snapshot: boot=%s workspaces=%d tabs=%d panes=%d agents=%d",
			snapshot.BootID, len(snapshot.Workspaces), len(snapshot.Tabs),
			len(snapshot.Panes), len(snapshot.Agents))
		if len(snapshot.Workspaces) == 0 {
			t.Fatal("the snapshot holds no workspace")
		}
	case <-ctx.Done():
		t.Fatal("no snapshot arrived")
	}

	select {
	case frame := <-endpoint.Surfaces():
		t.Logf("surface: panes=%d cells=%d grid=%dx%d",
			len(frame.Panes), len(frame.Grid.Cells), frame.Grid.Width, frame.Grid.Height)
		if len(frame.Grid.Cells) == 0 {
			t.Fatal("the surface holds no cell")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no surface arrived")
	}
}

// Command herdr-probe connects to the herdr daemon and says what it finds.
//
// It is how the wire is checked against the daemon that is running: the
// handshake, the snapshot of the session, and the surface of the pane in
// front.
//
//	go run ./cmd/herdr-probe
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"herdr-ui/internal/client"
	"herdr-ui/internal/protocol"
)

func main() {
	socket := flag.String("socket", "", "the socket of the daemon (the daemon's own by default)")
	flag.Parse()

	path := *socket
	if path == "" {
		path = client.SocketPath()
	}
	fmt.Println("socket:", path)

	hello := protocol.DefaultHello(120, 40, 8, 16)
	c, err := client.Connect(path, hello)
	if err != nil {
		fmt.Fprintln(os.Stderr, "herdr-probe:", err)
		os.Exit(1)
	}
	defer c.Close()

	welcome := c.Welcome()
	fmt.Printf("daemon %s, generation %d\n", welcome.ServerVersion, welcome.Generation)
	fmt.Printf("methods: %d, capabilities: %v\n", len(welcome.Methods), welcome.Capabilities)

	deadline := time.After(15 * time.Second)
	var snapshot *protocol.Snapshot
	var surface *protocol.SurfaceFrame
	for snapshot == nil || surface == nil {
		select {
		case event, ok := <-c.Events:
			if !ok {
				fmt.Fprintln(os.Stderr, "herdr-probe: the connection ended:", c.Err())
				os.Exit(1)
			}
			switch {
			case event.Snapshot != nil:
				snapshot = event.Snapshot
				printSnapshot(snapshot)
			case event.Surface != nil:
				surface = event.Surface
				printSurface(surface)
			case event.Notification != nil:
				fmt.Printf("notification: %s %s\n", event.Notification.Title, event.Notification.Body)
			case event.Error != nil:
				fmt.Printf("error: %s\n", event.Error.Message)
			}
		case <-deadline:
			fmt.Fprintln(os.Stderr, "herdr-probe: the daemon said nothing in time")
			os.Exit(1)
		}
	}

	// The surface must be asked for: the client says how big its viewport
	// is, and the daemon sends the surface of the pane in front.
	if err := c.Resize(120, 40, 8, 16); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-probe:", err)
	}
}

// printSnapshot says what the session holds.
func printSnapshot(snapshot *protocol.Snapshot) {
	fmt.Printf("\nsnapshot %d: %d workspaces, %d tabs, %d panes, %d agents, %d commands\n",
		snapshot.Revision,
		len(snapshot.Workspaces), len(snapshot.Tabs), len(snapshot.Panes),
		len(snapshot.Agents), len(snapshot.Commands))
	fmt.Println("focused:", snapshot.FocusedWorkspaceID, snapshot.FocusedTabID, snapshot.FocusedPaneID)
	for _, workspace := range snapshot.Workspaces {
		mark := " "
		if workspace.Focused {
			mark = "*"
		}
		fmt.Printf("  %s workspace %s label=%q branch=%q status=%s\n",
			mark, workspace.WorkspaceID, workspace.Label, workspace.Branch, workspace.AgentStatus)
	}
	for _, tab := range snapshot.Tabs {
		fmt.Printf("    tab %s number=%d label=%q focused=%v\n", tab.TabID, tab.Number, tab.Label, tab.Focused)
	}
	for _, pane := range snapshot.Panes {
		fmt.Printf("    pane %s cwd=%q focused=%v\n", pane.PaneID, pane.Cwd, pane.Focused)
	}
	for _, agent := range snapshot.Agents {
		fmt.Printf("    agent %s name=%q status=%s title=%q\n",
			agent.PaneID, agent.DisplayAgent, agent.AgentStatus, agent.Title)
	}
}

// printSurface says what the pane in front holds.
func printSurface(surface *protocol.SurfaceFrame) {
	fmt.Printf("\nsurface %dx%d, %d cells, revision %d\n",
		surface.Width, surface.Height, len(surface.Cells), surface.SurfaceRevision)
	for y := 0; y < int(surface.Height) && y < 12; y++ {
		text := surface.Text(y)
		if text == "" {
			continue
		}
		fmt.Printf("  %2d| %s\n", y, text)
	}
	if len(surface.Cells) > 0 {
		fmt.Println("the cells of the pane in front:")
		for _, pane := range surface.Panes {
			if !pane.Focused {
				continue
			}
			for _, row := range []int{0, 3, 5} {
				index := (int(pane.InnerRect.Y)+row)*int(surface.Width) + int(pane.InnerRect.X)
				if index < 0 || index >= len(surface.Cells) {
					continue
				}
				cell := surface.Cells[index]
				fmt.Printf("  row %d: symbol=%q fg=%08x bg=%08x mod=%04x\n",
					row, cell.Symbol, cell.Fg, cell.Bg, cell.Modifier)
			}
		}
	}
	if surface.Cursor != nil {
		fmt.Printf("cursor at %d,%d shown=%v\n", surface.Cursor.X, surface.Cursor.Y, surface.Cursor.Visible)
	}
	for _, pane := range surface.Panes {
		fmt.Printf("pane %s at %d,%d %dx%d focused=%v\n",
			pane.PaneID, pane.Rect.X, pane.Rect.Y, pane.Rect.Width, pane.Rect.Height, pane.Focused)
	}
}

// min returns the smaller of two numbers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// dumpJSON says a thing as JSON, for the eyes that want it.
func dumpJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return
	}
	fmt.Println(string(data))
}

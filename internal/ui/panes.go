package ui

import (
	"fmt"
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The daemon composes the panes of a tab into one grid of cells and leaves
// a strip above each pane for its title. This file paints those titles and
// finds the pane under the pointer.

// paneHeaderHeight is how many rows of a pane the title takes.
const paneHeaderHeight = 1

// drawPaneHeaders paints the title of every pane of the frame.
//
// The daemon leaves the strip above each pane's content empty; the title
// goes there, the way a terminal multiplexer names its panes.
func (a *App) drawPaneHeaders(p *ui.Painter, r ui.Rect, frame *herdr.SurfaceFrame, snapshot *herdr.Snapshot, cellWidth float32) {
	if frame == nil || snapshot == nil {
		return
	}
	for _, pane := range frame.Panes {
		if pane.InnerRect.Y <= pane.Rect.Y {
			continue
		}
		x := r.X + float32(pane.Rect.X)*cellWidth
		y := r.Y + float32(pane.Rect.Y)*a.rowHeightPx()
		width := float32(pane.Rect.Width) * cellWidth

		title := a.paneTitle(pane.PaneID, snapshot)
		if title == "" {
			continue
		}
		if pane.Focused {
			p.Fill(ui.Rect{X: x, Y: y, W: width, H: float32(a.rowHeightPx())}, ui.RGBA(120, 170, 255, 0.12), 0)
		}
		p.Text(x+4, y+a.fontSizePx(), title, a.fontSizePx(), ui.RGB(150, 158, 170))
	}
}

// paneTitle names a pane after its agent, its title and its folder.
func (a *App) paneTitle(paneID string, snapshot *herdr.Snapshot) string {
	var parts []string
	agent, hasAgent := snapshot.AgentOf(paneID)
	if hasAgent {
		if agent.Agent != nil && *agent.Agent != "" {
			parts = append(parts, *agent.Agent)
		}
		if agent.TerminalTitle != nil && *agent.TerminalTitle != "" {
			parts = append(parts, *agent.TerminalTitle)
		}
		if agent.AgentStatus != "" && agent.AgentStatus != "idle" {
			parts = append(parts, agent.AgentStatus)
		}
	}
	if len(parts) == 0 {
		if pane, ok := snapshot.Pane(paneID); ok {
			if pane.Label != nil && *pane.Label != "" {
				parts = append(parts, *pane.Label)
			}
			if pane.CWD != nil && *pane.CWD != "" {
				parts = append(parts, *pane.CWD)
			}
		}
	}
	return strings.Join(parts, " · ")
}

// paneAt says which pane of the frame lies under a point.
//
// The content of a pane is what the user aims at, so the inner rectangle
// of the pane is what counts.
func paneAt(frame *herdr.SurfaceFrame, x, y, cellWidth, rowHeight float32) (string, bool) {
	if frame == nil {
		return "", false
	}
	for _, pane := range frame.Panes {
		inner := pane.InnerRect
		left := float32(inner.X) * cellWidth
		top := float32(inner.Y) * rowHeight
		right := left + float32(inner.Width)*cellWidth
		bottom := top + float32(inner.Height)*rowHeight
		if x >= left && x < right && y >= top && y < bottom {
			return pane.PaneID, true
		}
	}
	return "", false
}

// focusPane asks the daemon to focus a pane and remembers it here, so that
// what the user types goes to the pane they aimed at.
func (a *App) focusPane(paneID string) {
	if paneID == "" {
		return
	}
	a.mu.Lock()
	a.pane = paneID
	a.mu.Unlock()
	a.request("pane.focus", map[string]any{"pane_id": paneID})
	a.invalidate()
}

// selectedPaneID returns the pane the user last aimed at.
func (a *App) selectedPaneID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pane
}

// targetPane returns the pane that takes what the user types: the one they
// aimed at, or the one the daemon focuses.
func (a *App) targetPane(snapshot *herdr.Snapshot) (herdr.Pane, bool) {
	if snapshot == nil {
		return herdr.Pane{}, false
	}
	if selected := a.selectedPaneID(); selected != "" {
		if pane, ok := snapshot.Pane(selected); ok {
			return pane, true
		}
	}
	return snapshot.FocusedPane()
}

// agentRows returns the agents of the session, in the order the daemon
// gives them.
func agentRows(snapshot *herdr.Snapshot) []herdr.Agent {
	if snapshot == nil {
		return nil
	}
	agents := append([]herdr.Agent(nil), snapshot.Agents...)
	if len(snapshot.AgentOrder) == 0 {
		return agents
	}
	byPane := map[string]herdr.Agent{}
	for _, agent := range agents {
		byPane[agent.PaneID] = agent
	}
	ordered := make([]herdr.Agent, 0, len(agents))
	for _, paneID := range snapshot.AgentOrder {
		if agent, ok := byPane[paneID]; ok {
			ordered = append(ordered, agent)
			delete(byPane, paneID)
		}
	}
	for _, agent := range agents {
		if _, ok := byPane[agent.PaneID]; ok {
			ordered = append(ordered, agent)
		}
	}
	return ordered
}

// agentLabel names an agent for the sidebar.
func agentLabel(agent herdr.Agent) string {
	name := ""
	if agent.DisplayAgent != nil && *agent.DisplayAgent != "" {
		name = *agent.DisplayAgent
	} else if agent.Agent != nil {
		name = *agent.Agent
	}
	if name == "" {
		name = "agent"
	}
	if agent.AgentStatus != "" && agent.AgentStatus != "idle" {
		name = fmt.Sprintf("%s (%s)", name, agent.AgentStatus)
	}
	return name
}

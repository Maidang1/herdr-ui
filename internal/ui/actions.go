package ui

import (
	"fmt"
	"time"

	"herdr-ui/internal/herdr"
)

// The daemon offers a few things about the session that are not in the
// snapshot: the worktrees of a repository, the coding agents that are
// installed, and the actions a pane can take. This file asks for them and
// keeps what comes back.

// WorktreeInfo is one worktree of the repository of the session.
type WorktreeInfo struct {
	Path            string `json:"path"`
	Label           string `json:"label"`
	Branch          string `json:"branch"`
	OpenWorkspaceID string `json:"open_workspace_id"`
	IsLinked        bool   `json:"is_linked_worktree"`
}

// IntegrationInfo is one coding agent the daemon knows.
type IntegrationInfo struct {
	Target    string `json:"target"`
	Label     string `json:"label"`
	Command   string `json:"command"`
	State     string `json:"state"`
	Available bool   `json:"available"`
}

// refreshPanels asks the daemon for the worktrees and the agents.
func (a *App) refreshPanels() {
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()

		var worktrees struct {
			Worktrees []WorktreeInfo `json:"worktrees"`
		}
		if err := a.endpoint.Request(ctx, "worktree.list", map[string]any{}, &worktrees); err == nil {
			a.mu.Lock()
			a.worktrees = worktrees.Worktrees
			a.mu.Unlock()
			a.invalidate()
		}

		var integrations struct {
			Integrations []IntegrationInfo `json:"integrations"`
		}
		if err := a.endpoint.Request(ctx, "integration.list", map[string]any{}, &integrations); err == nil {
			a.mu.Lock()
			a.integrations = integrations.Integrations
			a.mu.Unlock()
			a.invalidate()
		}
	}()
}

// worktreeRows returns the worktrees the daemon knows.
func (a *App) worktreeRows() []WorktreeInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]WorktreeInfo(nil), a.worktrees...)
}

// integrationRows returns the coding agents the daemon knows.
func (a *App) integrationRows() []IntegrationInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]IntegrationInfo(nil), a.integrations...)
}

// openWorktree asks the daemon to open a worktree in a workspace.
func (a *App) openWorktree(path string) {
	if path == "" {
		return
	}
	a.request("worktree.open", map[string]any{"path": path, "focus": true})
	a.setStatus("asked for %s", path)
}

// splitPane asks the daemon to split a pane.
func (a *App) splitPane(paneID, direction string) {
	if paneID == "" {
		a.setStatus("no pane is selected")
		return
	}
	a.request("pane.split", map[string]any{
		"target_pane_id": paneID,
		"direction":      direction,
		"focus":          true,
	})
}

// zoomPane asks the daemon to zoom a pane, or to unzoom it.
func (a *App) zoomPane(paneID string) {
	if paneID == "" {
		a.setStatus("no pane is selected")
		return
	}
	a.request("pane.zoom", map[string]any{"pane_id": paneID, "mode": "toggle"})
}

// scrollPane moves the view of a pane up or down its scrollback.
func (a *App) scrollPane(paneID string, delta int) {
	if paneID == "" {
		a.setStatus("no pane is selected")
		return
	}
	offset := a.scrollOffset(paneID) + delta
	if offset < 0 {
		offset = 0
	}
	a.request("pane.scroll", map[string]any{"pane_id": paneID, "offset_from_bottom": offset})
}

// scrollOffset says how far the view of a pane sits from the end of its
// output.
func (a *App) scrollOffset(paneID string) int {
	a.mu.Lock()
	frame := a.surface
	a.mu.Unlock()
	if frame == nil {
		return 0
	}
	for _, pane := range frame.Panes {
		if pane.PaneID == paneID && pane.Scroll != nil {
			return int(pane.Scroll.OffsetFromBottom)
		}
	}
	return 0
}

// closePane asks the daemon to close a pane.
func (a *App) closePane(paneID string) {
	if paneID == "" {
		a.setStatus("no pane is selected")
		return
	}
	a.request("pane.close", map[string]any{"pane_id": paneID})
}

// activateSurface tells the daemon that this client shows the terminals.
//
// The daemon serves the methods that act on a surface only to a client that
// holds one.
func (a *App) activateSurface() {
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := a.endpoint.Request(ctx, "client_shell.surface.set", map[string]any{"active": true}, nil); err != nil {
			logf("herdr-ui: could not activate the surface: %v", err)
		}
	}()
}

// surfaceOf returns the frame the daemon last painted.
func (a *App) surfaceOf() *herdr.SurfaceFrame {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.surface
}

// paneLabel names a pane for the logs.
func paneLabel(paneID string) string {
	return fmt.Sprintf("pane %s", paneID)
}

// refreshAfter waits a moment and then asks for the panels again.
func (a *App) refreshAfter(d time.Duration) {
	go func() {
		time.Sleep(d)
		a.refreshPanels()
	}()
}

// Package sidebarinfo collects local read-only workspace metadata outside render.
package sidebarinfo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/bytedance/herdr-ui/internal/session"
)

type PaneSource struct{ ID, TerminalID string }
type WorkspaceSource struct {
	ID, Path string
	Panes    []PaneSource
}
type Source struct {
	TargetID   uint64
	Workspaces []WorkspaceSource
}
type Info struct {
	Path, Branch, Remote string
	RepoRoot             string
	Ahead, Behind        int
	GitKnown, PortsKnown bool
	Ports                []int
}
type Entry struct {
	Fingerprint string
	Info        Info
}
type Publication struct {
	TargetID uint64
	Entries  map[string]Entry
}

// BuildSource uses server ordering, never this window's selected pane.
func BuildSource(p session.PublishedSnapshot) Source {
	source := Source{TargetID: p.TargetID}
	if p.Snapshot == nil {
		return source
	}
	for _, w := range p.Snapshot.Workspaces() {
		item := WorkspaceSource{ID: w.ID, Path: w.CheckoutPath}
		for _, tab := range p.Snapshot.Tabs(w.ID) {
			for _, pane := range p.Snapshot.Panes(tab.ID) {
				if item.Path == "" && pane.CWD != "" {
					item.Path = pane.CWD
				}
				item.Panes = append(item.Panes, PaneSource{ID: pane.ID, TerminalID: pane.TerminalID})
			}
		}
		source.Workspaces = append(source.Workspaces, item)
	}
	return source
}

func Fingerprint(source WorkspaceSource) string {
	data, _ := json.Marshal(source)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func cloneSource(source Source) Source {
	source.Workspaces = append([]WorkspaceSource(nil), source.Workspaces...)
	for i := range source.Workspaces {
		source.Workspaces[i].Panes = append([]PaneSource(nil), source.Workspaces[i].Panes...)
	}
	return source
}
func clonePublication(p Publication) Publication {
	entries := make(map[string]Entry, len(p.Entries))
	for id, entry := range p.Entries {
		entry.Info.Ports = append([]int(nil), entry.Info.Ports...)
		entries[id] = entry
	}
	p.Entries = entries
	return p
}

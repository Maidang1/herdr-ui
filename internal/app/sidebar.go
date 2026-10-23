package app

import "github.com/bytedance/herdr-ui/internal/sidebarinfo"

func (a *App) sidebarSources() map[string]string {
	p := a.publication
	p.Snapshot = a.snapshot
	sources := map[string]string{}
	for _, source := range sidebarinfo.BuildSource(p).Workspaces {
		sources[source.ID] = sidebarinfo.Fingerprint(source)
	}
	return sources
}
func (a *App) pruneSidebar() {
	sources := a.sidebarSources()
	for id, entry := range a.sidebar {
		if sources[id] != entry.Fingerprint {
			delete(a.sidebar, id)
		}
	}
}

// UpdateSidebar installs only metadata whose workspace path and terminal set
// still match this window's authoritative snapshot. It performs no I/O.
func (a *App) UpdateSidebar(p sidebarinfo.Publication) {
	if p.TargetID != a.publication.TargetID || a.snapshot == nil {
		return
	}
	sources := a.sidebarSources()
	if a.sidebar == nil {
		a.sidebar = map[string]sidebarinfo.Entry{}
	}
	changed := false
	for id, entry := range p.Entries {
		if sources[id] != entry.Fingerprint {
			continue
		}
		entry.Info.Ports = append([]int(nil), entry.Info.Ports...)
		a.sidebar[id] = entry
		changed = true
	}
	if changed {
		a.version++
	}
}

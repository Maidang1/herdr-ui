package sidebarinfo

import (
	"github.com/Maidang1/herdr-ui/internal/session"
	"testing"
)

func TestBuildSourceUsesAuthoritativeOrderAndFingerprintOwnsIdentity(t *testing.T) {
	snapshot, err := session.DecodeSnapshot([]byte(`{"type":"session_snapshot","snapshot":{"version":"test","protocol":22,"workspaces":[{"workspace_id":"w"}],"tabs":[{"workspace_id":"w","tab_id":"a"},{"workspace_id":"w","tab_id":"b"}],"panes":[{"workspace_id":"w","tab_id":"a","pane_id":"p1","terminal_id":"t1","cwd":"/repo"},{"workspace_id":"w","tab_id":"b","pane_id":"p2","terminal_id":"t2","cwd":"/other","focused":true}],"layouts":[],"agents":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	source := BuildSource(session.PublishedSnapshot{TargetID: 3, Snapshot: snapshot})
	if source.TargetID != 3 || source.Workspaces[0].Path != "/repo" || source.Workspaces[0].Panes[1].ID != "p2" {
		t.Fatal(source)
	}
	original := Fingerprint(source.Workspaces[0])
	copy := cloneSource(source)
	copy.Workspaces[0].Panes[0].TerminalID = "replacement"
	if Fingerprint(copy.Workspaces[0]) == original || Fingerprint(source.Workspaces[0]) != original {
		t.Fatal("fingerprint or clone lost pane identity")
	}
}
func TestPublicationCloneOwnsMapAndPorts(t *testing.T) {
	p := Publication{Entries: map[string]Entry{"w": {Info: Info{Ports: []int{3000}}}}}
	copy := clonePublication(p)
	entry := copy.Entries["w"]
	entry.Info.Ports[0] = 9999
	delete(copy.Entries, "w")
	if p.Entries["w"].Info.Ports[0] != 3000 {
		t.Fatal("published ports alias")
	}
}

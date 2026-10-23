package layout

import (
	"encoding/json"
	"testing"
)

func TestExportPaths(t *testing.T) {
	var root Node
	if err := json.Unmarshal([]byte(`{"type":"split","direction":"right","ratio":0.5,"first":{"type":"pane","pane_id":"a"},"second":{"type":"split","direction":"down","ratio":0.6,"first":{"type":"pane","pane_id":"b"},"second":{"type":"pane","pane_id":"c"}}}`), &root); err != nil {
		t.Fatal(err)
	}
	n, err := ResolvePath(&root, []bool{true, false})
	if err != nil || n.PaneID != "b" {
		t.Fatalf("node=%v error=%v", n, err)
	}
	if _, err := ResolvePath(&root, []bool{false, true}); err == nil {
		t.Fatal("walked through leaf")
	}
	paths, err := SplitPaths(&root)
	if err != nil || len(paths) != 2 || len(paths[0]) != 0 || len(paths[1]) != 1 || !paths[1][0] {
		t.Fatalf("paths=%v error=%v", paths, err)
	}
	root.Second.Second.PaneID = "a"
	if _, err := SplitPaths(&root); err == nil {
		t.Fatal("duplicate pane accepted")
	}
}

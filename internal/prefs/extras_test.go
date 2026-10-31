package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStartupAndLayoutsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Settings(); got.StartupMode != "auto" || got.AgentView != "all" || got.ShareReminders {
		t.Fatalf("defaults %+v", got)
	}
	if _, err := s.Update(func(v *Settings) { v.StartupMode, v.AgentView, v.ShareReminders = "plain", "working", true }); err != nil {
		t.Fatal(err)
	}
	root := json.RawMessage(`{"type":"pane","cwd":"/src"}`)
	if err := s.SaveLayoutTemplate(LayoutTemplate{Name: "Review", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLayoutTemplate(LayoutTemplate{Name: "Review", Root: root}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveLayoutTemplate(LayoutTemplate{Name: "Build", Root: root}); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Settings(); got.StartupMode != "plain" || got.AgentView != "working" || !got.ShareReminders {
		t.Fatalf("reopened %+v", got)
	}
	if layouts := reopened.LayoutTemplates(); len(layouts) != 2 || layouts[0].Name != "Review" || layouts[1].Name != "Build" {
		t.Fatalf("layouts %+v", layouts)
	}
	if err := reopened.DeleteLayoutTemplate("Review"); err != nil || len(reopened.LayoutTemplates()) != 1 {
		t.Fatalf("delete %v %+v", err, reopened.LayoutTemplates())
	}
	os.WriteFile(path, []byte(`{"startup_mode":"sometimes"}`), 0o644)
	if invalid, _ := Open(path); invalid.Settings().StartupMode != "auto" {
		t.Fatal("kept an invalid startup mode")
	}
}

func TestMemoryStoreWritesNothing(t *testing.T) {
	dir := t.TempDir()
	wd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(wd)
	s := Memory()
	if _, err := s.Update(func(v *Settings) { v.StartupMode = "plain" }); err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 || s.Settings().StartupMode != "plain" {
		t.Fatalf("wrote %v", entries)
	}
}

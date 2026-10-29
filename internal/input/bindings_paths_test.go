package input

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Maidang1/herdr-ui/internal/herdr"
)

func TestLoadBindingsUsesDaemonConfigPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HERDR_CONFIG_PATH", "")
	t.Setenv("XDG_CONFIG_HOME", dir)
	home, _ := os.UserHomeDir()
	path := herdr.ConfigPath(home, os.Getenv)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[keys]\nprefix='ctrl+a'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	b, err := LoadBindings("")
	if err != nil {
		t.Fatal(err)
	}
	if b.Source != path || b.Prefix != "ctrl+a" {
		t.Fatalf("bindings loaded from different path: %+v", b)
	}
	override := filepath.Join(dir, "override.toml")
	if err := os.WriteFile(override, []byte("[keys]\nprefix='ctrl+q'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_CONFIG_PATH", override)
	b, err = LoadBindings("")
	if err != nil {
		t.Fatal(err)
	}
	if b.Source != override || b.Prefix != "ctrl+q" {
		t.Fatalf("override ignored: %+v", b)
	}
	b, err = LoadBindings(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Source != path || b.Prefix != "ctrl+a" {
		t.Fatal("explicit file lost precedence")
	}
}

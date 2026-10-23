package input

import (
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeKey(t *testing.T) {
	for _, tt := range []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{ui.Shift, ui.KeySlash, "?"}, {ui.Shift, ui.KeyEqual, "+"}, {ui.Shift, ui.KeyH, "shift+h"},
		{ui.Ctrl | ui.Alt, ui.KeyB, "ctrl+alt+b"}, {ui.Super, ui.KeyV, "cmd+v"}, {ui.Shift, ui.KeyTab, "shift+tab"},
	} {
		got, ok := NativeKey(tt.mods, tt.key)
		if !ok || got != tt.want {
			t.Fatalf("%v/%v = %q,%v want %q", tt.mods, tt.key, got, ok, tt.want)
		}
	}
	if _, ok := NativeKey(0, ui.KeyUnknown); ok {
		t.Fatal("unknown key accepted")
	}
}
func TestCompositionAndResize(t *testing.T) {
	b, _ := Resolve(nil, "defaults")
	m := Machine{Bindings: b}
	m.Key("ctrl+b")
	if r := m.Handle("v", true); r.Consumed || m.Awaiting || m.Resize {
		t.Fatal(r, m)
	}
	m.Key("ctrl+b")
	if r := m.Handle("r", false); r.Action != "resize_mode" || !m.Resize {
		t.Fatal(r, m)
	}
	for _, k := range []string{"h", "left", "shift+left", "ctrl+h"} {
		if r := m.Key(k); r.Action != "resize_left" || !r.Consumed || !m.Resize {
			t.Fatal(r, m)
		}
	}
	if r := m.Key("a"); !r.Consumed || !m.Resize {
		t.Fatal(r, m)
	}
	if r := m.Key("q"); !r.Consumed || !m.Resize {
		t.Fatal(r, m)
	}
	if r := m.Key("shift+esc"); !r.Consumed || m.Resize {
		t.Fatal(r, m)
	}
	m.Resize = true
	m.Handle("ctrl+b", true)
	if m.Resize || m.Awaiting {
		t.Fatal(m)
	}
	m.Resize = true
	m.Reset()
	if m.Resize {
		t.Fatal(m)
	}
}

func TestLoadBindingsConfigPrecedenceAndErrors(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	t.Setenv("HERDR_CONFIG_PATH", "")
	b, err := LoadBindings("")
	if err != nil || b.Prefix != "ctrl+b" {
		t.Fatal(b, err)
	}
	path := filepath.Join(root, "custom.toml")
	if err := os.WriteFile(path, []byte("[keys]\nprefix='ctrl+a'"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_CONFIG_PATH", path)
	b, err = LoadBindings("")
	if err != nil || b.Prefix != "ctrl+a" || b.Source != path {
		t.Fatal(b, err)
	}
	invalid := filepath.Join(root, "invalid.toml")
	os.WriteFile(invalid, []byte("[keys]\nprefix = ["), 0600)
	if _, err = LoadBindings(invalid); err == nil {
		t.Fatal("invalid explicit config silently fell back")
	}
}

func TestConfiguredCtrlBActionPrecedesTransportEscape(t *testing.T) {
	b, err := Resolve([]byte("[keys]\nprefix='ctrl+a'\nsplit_vertical='ctrl+b'\n"), "test")
	if err != nil || len(b.Diagnostics) != 0 {
		t.Fatal(b, err)
	}
	m := Machine{Bindings: b}
	if r := m.Key("ctrl+b"); r.Action != "split_vertical" || len(r.Send) != 0 {
		t.Fatal(r)
	}
	delete(b.Actions, "ctrl+b")
	if r := m.Key("ctrl+b"); string(r.Send) != "\x02\x02" {
		t.Fatal(r)
	}
}

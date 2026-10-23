package input

import (
	"github.com/egoist/mygo/ui"
	"testing"
)

func TestIndexedTargetsSurviveNormalization(t *testing.T) {
	b, err := Resolve([]byte("[keys]\nswitch_workspace='prefix+shift+1..9'\nfocus_agent=['alt+2','alt+9']"), "indexed")
	if err != nil {
		t.Fatal(err)
	}
	m := Machine{Bindings: b}
	m.Key(b.Prefix)
	if r := m.Key("@"); r.Action != "switch_workspace" || r.Index != 1 {
		t.Fatal(r)
	}
	if r := m.Key("alt+9"); r.Action != "focus_agent" || r.Index != 8 {
		t.Fatal(r)
	}
	m.Key(b.Prefix)
	if r := m.Key("5"); r.Action != "switch_tab" || r.Index != 4 {
		t.Fatal(r)
	}
	invalid, err := Resolve([]byte("[keys]\nswitch_tab='prefix+?'"), "invalid")
	if err != nil || len(invalid.Diagnostics) == 0 || invalid.Actions["prefix+?"] != "help" {
		t.Fatal(invalid, err)
	}

}

func TestNavigationAndLegacyOverlay(t *testing.T) {
	b, err := Resolve([]byte("[keys]\nnavigate_workspace_up='j'\nnavigate_pane_down='ctrl+j'\n[keys.indexed]\ntabs='alt'\nworkspaces='ctrl+shift'"), "overlay")
	if err != nil {
		t.Fatal(err)
	}
	if b.Navigation["j"] != "navigate_workspace_up" || b.Navigation["ctrl+j"] != "navigate_pane_down" || b.Actions["j"] != "" {
		t.Fatal(b)
	}
	if b.Actions["prefix+1"] != "" || b.Actions["alt+1"] != "switch_tab" || b.Indices["ctrl+!"] != 0 {
		t.Fatal(b)
	}
	for _, label := range []string{"prefix+j", "esc", "alt+esc", "enter", "1", "left", "tab", "ctrl+b"} {
		invalid, err := Resolve([]byte("[keys]\nnavigate_workspace_up='"+label+"'"), "reserved")
		if err != nil || len(invalid.Diagnostics) == 0 {
			t.Fatalf("reserved navigation %s lacks diagnostic: %v %v", label, invalid.Diagnostics, err)
		}

	}
}

func TestCustomCommandPayload(t *testing.T) {
	b, err := Resolve([]byte("[[keys.command]]\nkey='prefix+y'\ncommand='echo hello'\ndescription='hello'\n[[keys.command]]\nkey='prefix+f'\ncommand='lazygit'\ntype='popup'\nwidth=90\nheight='80%'"), "commands")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Commands) != 2 || b.Commands[0].Type != "shell" || b.Commands[0].Description != "hello" || b.Commands[1].Width != int64(90) || b.Commands[1].Height != "80%" {
		t.Fatal(b.Commands)
	}
	m := Machine{Bindings: b}
	m.Key(b.Prefix)
	if r := m.Key("f"); r.Action != "command[1]" || r.Index != 1 {
		t.Fatal(r)
	}
}

func TestAlternatePrefixLiterals(t *testing.T) {
	for key, want := range map[string]string{"f12": "\x1b[24~", "esc": "\x1b", "n": "n", "alt+x": "\x1bx", "ctrl+a": "\x01", "ctrl+b": "\x02\x02", "ctrl+alt+b": "\x1b\x02\x02", "shift+tab": "\x1b[Z", "ctrl+left": "\x1b[1;5D", "plus": "+"} {
		b, err := Resolve([]byte("[keys]\nprefix='"+key+"'"), "prefix")
		if err != nil {
			t.Fatal(key, err)
		}
		m := Machine{Bindings: b}
		m.Key(b.Prefix)
		if r := m.Key(b.Prefix); string(r.Send) != want {
			t.Fatalf("%s = %q want %q", key, r.Send, want)
		}
	}
}

func TestAdditionalDefaultsAndNativeKeys(t *testing.T) {
	b, err := Resolve(nil, "defaults")
	if err != nil {
		t.Fatal(err)
	}
	if b.Actions["ctrl+v"] != "remote_image_paste" || b.Navigation["up"] != "navigate_workspace_up" {
		t.Fatal(b)
	}
	b, err = Resolve([]byte("[keys]\nfullscreen='prefix+f'"), "alias")
	if err != nil || b.Actions["prefix+f"] != "zoom" {
		t.Fatal(b, err)
	}
	for k, want := range map[ui.Key]string{ui.KeyHome: "home", ui.KeyEnd: "end", ui.KeyDelete: "delete", ui.KeyInsert: "insert", ui.KeyPageUp: "pageup", ui.KeyPageDown: "pagedown"} {
		if got, ok := NativeKey(0, k); !ok || got != want {
			t.Fatal(k, got, ok)
		}
	}
}

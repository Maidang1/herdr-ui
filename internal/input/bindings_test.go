package input

import "testing"

func TestBindings(t *testing.T) {
	b, err := Resolve(nil, "defaults")
	if err != nil {
		t.Fatal(err)
	}
	if b.Prefix != "ctrl+b" || b.Actions["prefix+-"] != "split_horizontal" {
		t.Fatal(b)
	}
	b, err = Resolve([]byte("[keys]\nprefix='control+a'\nsplit_vertical=['prefix+option+v', 'command+v']\nsplit_horizontal=''\n"), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if b.Prefix != "ctrl+a" || b.Actions["prefix+alt+v"] != "split_vertical" || b.Actions["cmd+v"] != "split_vertical" || b.Actions["prefix+-"] != "" {
		t.Fatal(b)
	}
	for _, config := range []string{
		"[keys]\nprefix='nonsense'", "[keys]\nsplit_vertical='prefix+ctrl+b'",
		"[keys]\nsplit_vertical='prefix+x'\nsplit_horizontal='prefix+x'", "[keys]\nsplit_vertical='prefix+hyper+v'",
		"[keys]\nsplit_vertical='v'",
		"[keys]\ncommand=[{key='prefix+g',command=''}]",
	} {
		if _, err := Resolve([]byte(config), "fixture"); err == nil {
			t.Fatalf("unsupported/conflicting config accepted: %s", config)
		}
	}
}

func TestCustomCommandsReserveBindings(t *testing.T) {
	b, err := Resolve([]byte("[[keys.command]]\nkey=['prefix+shift+h','command+g']\ncommand='echo test'\ntype='shell'\n"), "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if b.Actions["prefix+shift+h"] != "command[0]" || b.Actions["cmd+g"] != "command[0]" {
		t.Fatal(b)
	}
	if _, err := Resolve([]byte("[keys]\nsplit_vertical='prefix+g'\n[[keys.command]]\nkey='prefix+g'\ncommand='echo test'"), "fixture"); err == nil {
		t.Fatal("conflicting command accepted")
	}
}

func TestPrefixTransport(t *testing.T) {
	b, err := Resolve(nil, "default")
	if err != nil {
		t.Fatal(err)
	}
	m := Machine{Bindings: b}
	if r := m.Key("ctrl+b"); !r.Consumed || len(r.Send) != 0 {
		t.Fatal(r)
	}
	if r := m.Key("v"); r.Action != "split_vertical" || !r.Consumed || len(r.Send) != 0 {
		t.Fatal(r)
	}
	m.Key("ctrl+b")
	if r := m.Key("ctrl+b"); string(r.Send) != "\x02\x02" {
		t.Fatalf("literal = %v", r)
	}
	m.Key("ctrl+b")
	m.Reset()
	if r := m.Key("q"); r.Consumed {
		t.Fatal("stale prefix consumed q")
	}
	b.Prefix = "ctrl+a"
	m.Bindings = b
	if r := m.Key("ctrl+b"); string(r.Send) != "\x02\x02" {
		t.Fatal("normal Ctrl-B not escaped")
	}
	m.Key("ctrl+a")
	if r := m.Key("ctrl+a"); string(r.Send) != "\x01" {
		t.Fatal(r)
	}
}

func TestAliasesAndNormalization(t *testing.T) {
	for a, w := range map[string]string{"control+option+V": "ctrl+alt+shift+v", "prefix+minus": "prefix+-", "shift+/": "?", "escape": "esc", "shift+tab": "shift+tab", "command+plus": "cmd++"} {
		got, err := Normalize(a)
		if err != nil || got != w {
			t.Fatalf("%q = %q %v, want %q", a, got, err, w)
		}
	}
}

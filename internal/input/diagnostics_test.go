package input

import (
	"strings"
	"testing"
)

func TestBindingDiagnosticsKeepValidSiblings(t *testing.T) {
	b, err := Resolve([]byte(`[keys]
help = ["prefix+y", "prefix+hyper+h", "prefix+f13", 42]
new_tab = "prefix+c"
unknown_future_action = "alt+k"
[[keys.command]]
key = "prefix+f"
command = ""
[[keys.command]]
key = "prefix+t"
command = "echo valid"
`), "mixed")
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Diagnostics) != 5 {
		t.Fatal(b.Diagnostics)
	}
	if b.Actions["prefix+y"] != "help" || b.Actions["prefix+c"] != "new_tab" || b.Actions["prefix+t"] != "command[1]" || len(b.Commands) != 2 || b.Commands[1].Command != "echo valid" {
		t.Fatal(b)
	}
	m := Machine{Bindings: b}
	m.Key(b.Prefix)
	if r := m.Key("t"); r.Index != 1 || r.Action != "command[1]" {
		t.Fatal(r)
	}
}
func TestConflictPrecedenceMatchesHerdr(t *testing.T) {
	b, err := Resolve([]byte(`[keys]
switch_tab = "ctrl+1"
resize_pane_left = "alt+2"
last_pane = "prefix+f"
cycle_pane_next = "prefix+f"
[keys.indexed]
tabs = "alt"
[[keys.command]]
key = ["alt+2", "alt+3", "prefix+z"]
command = "echo custom"
`), "precedence")
	if err != nil {
		t.Fatal(err)
	}
	if b.Actions["alt+2"] != "resize_pane_left" || b.Actions["alt+3"] != "switch_tab" || b.Actions["ctrl+1"] != "switch_tab" || b.Actions["prefix+z"] != "command[0]" || b.Actions["prefix+f"] != "last_pane" {
		t.Fatal(b)
	}
	if len(b.Diagnostics) != 4 {
		t.Fatal(b.Diagnostics)
	}
}
func TestInvalidLegacyOverrideSuppressesDefault(t *testing.T) {
	b, err := Resolve([]byte("[keys.indexed]\ntabs='bogus'"), "legacy")
	if err != nil || len(b.Diagnostics) != 1 || b.Actions["prefix+1"] != "" || b.Actions["prefix+?"] != "help" {
		t.Fatal(b, err)
	}
}
func TestUnsupportedNativePrefixesDiagnoseAndFallback(t *testing.T) {
	for _, key := range []string{"f13", "f24", "hyper+x", "ö", "prefix+x", "nonsense"} {
		b, err := Resolve([]byte("[keys]\nprefix='"+key+"'"), "prefix")
		if err != nil || b.Prefix != "ctrl+b" || len(b.Diagnostics) != 1 {
			t.Fatal(key, b, err)
		}
		if b.Actions["prefix+c"] != "new_tab" {
			t.Fatal(b)
		}
	}
	if _, err := Resolve([]byte("[keys]\nhelp=["), "syntax"); err == nil {
		t.Fatal("TOML syntax error not fatal")
	}
}
func TestLiteralModifiedPrefixEncoding(t *testing.T) {
	for key, want := range map[string]string{
		"cmd+a": "\x1b[97;9u", "ctrl+cmd+a": "\x1b[97;13u", "ctrl+shift+b": "\x1b[98;6u",
		"alt+u": "\x1bu", "ctrl+2": "\x00", "ctrl+3": "\x1b", "ctrl+7": "\x1f", "ctrl+8": "\x7f",
		"ctrl+backspace": "\x08", "alt+ctrl+backspace": "\x1b\x08", "shift+enter": "\x1b[13;2u",
		"ctrl+tab": "\x1b[9;5u", "cmd+plus": "\x1b[43;9u",
	} {
		if got := LiteralKey(key); string(got) != want {
			t.Fatalf("%s=%q want %q", key, got, want)
		}
	}
	for _, invalid := range []string{"", "hyper+x", "prefix+a", "nonsense"} {
		if LiteralKey(invalid) != nil {
			t.Fatal("invalid literal", invalid)
		}
	}
}
func TestIndexedModifierOrderAndCommandSize(t *testing.T) {
	b, err := Resolve([]byte(`[keys]
switch_workspace = "prefix+1+ctrl"
[[keys.command]]
key = "prefix+t"
command = "echo zero"
type = "popup"
width = 0
height = "100%"
`), "compatibility")
	if err != nil || len(b.Diagnostics) != 0 || b.Actions["prefix+ctrl+1"] != "switch_workspace" || b.Commands[0].Width != int64(0) {
		t.Fatal(b, err)
	}
	if n, e := Normalize("Control+BACKTAB"); e != nil || n != "ctrl+shift+tab" {
		t.Fatal(n, e)
	}
}
func TestPrefixEscapeCannotAdvertiseUnreachableAction(t *testing.T) {
	b, err := Resolve([]byte("[keys]\nhelp='prefix+alt+esc'"), "escape")
	if err != nil || len(b.Diagnostics) != 1 || !strings.Contains(b.Diagnostics[0], "Escape cancels") {
		t.Fatal(b, err)
	}
	m := Machine{Bindings: b}
	m.Key(b.Prefix)
	if r := m.Key("alt+esc"); !r.Consumed || r.Action != "" || m.Awaiting {
		t.Fatal(r)
	}
}

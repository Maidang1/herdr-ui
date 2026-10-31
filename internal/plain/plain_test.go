package plain

import (
	"testing"

	"github.com/Maidang1/herdr-ui/internal/herdr"
	native "github.com/egoist/mygo/ui"
)

func TestShortcutKeepsControlForTheShell(t *testing.T) {
	cases := []struct {
		goos string
		mods native.Modifiers
		key  native.Key
		want string
	}{
		{"darwin", native.Super, native.KeyT, "new_tab"},
		{"darwin", native.Super, native.KeyW, "close_tab"},
		{"darwin", native.Super | native.Shift, native.KeyBracketRight, "next_tab"},
		{"darwin", native.Super, native.Key3, "select_tab_3"},
		{"linux", native.Ctrl | native.Shift, native.KeyT, "new_tab"},
		{"linux", native.Ctrl, native.KeyTab, "next_tab"},
		{"linux", native.Ctrl | native.Shift, native.KeyTab, "previous_tab"},
		{"windows", native.Ctrl | native.Shift, native.Key9, "select_tab_9"},
	}
	for _, c := range cases {
		if got, ok := Shortcut(c.goos, c.mods, c.key); !ok || got != c.want {
			t.Fatalf("%s %v %v: %q %v", c.goos, c.mods, c.key, got, ok)
		}
	}
	for _, key := range []native.Key{native.KeyT, native.KeyW, native.KeyC} {
		if got, ok := Shortcut("linux", native.Ctrl, key); ok {
			t.Fatalf("Control+%v taken as %s", key, got)
		}
	}
}

func TestDefaultShell(t *testing.T) {
	env := map[string]string{"SHELL": "/usr/bin/fish"}
	get := func(k string) string { return env[k] }
	if got := DefaultShell("linux", get); got[0] != "/usr/bin/fish" || got[1] != "-l" {
		t.Fatalf("%v", got)
	}
	if got := DefaultShell("darwin", func(string) string { return "" }); got[0] != "/bin/zsh" {
		t.Fatalf("%v", got)
	}
	if got := DefaultShell("windows", func(k string) string {
		if k == "COMSPEC" {
			return `C:\Windows\System32\cmd.exe`
		}
		return ""
	}); got[0] != `C:\Windows\System32\cmd.exe` {
		t.Fatalf("%v", got)
	}
}

func TestTabCommands(t *testing.T) {
	closed := false
	w := &Window{opts: Options{Close: func() { closed = true }}, tabs: []*tab{{id: 1}, {id: 2}, {id: 3}}}
	w.Command("next_tab")
	w.Command("next_tab")
	w.Command("next_tab")
	if w.selected != 0 {
		t.Fatalf("wrap %d", w.selected)
	}
	w.Command("previous_tab")
	if w.selected != 2 {
		t.Fatalf("previous %d", w.selected)
	}
	w.Command("select_tab_9")
	if w.selected != 2 {
		t.Fatalf("last %d", w.selected)
	}
	w.Command("select_tab_1")
	w.Command("close_tab")
	if len(w.tabs) != 2 || w.tabs[0].id != 2 || w.selected != 0 {
		t.Fatalf("close %+v %d", w.tabs, w.selected)
	}
	w.Command("close_tab")
	w.Command("close_tab")
	if len(w.tabs) != 0 || !closed {
		t.Fatal("last tab did not close the window")
	}
	if w.Command("select_tab_0") || w.Command("unknown") {
		t.Fatal("took an unknown command")
	}
}

func TestHerdrChipExplainsAndConnects(t *testing.T) {
	install := herdr.Installation{}
	connected, opened := 0, ""
	w := &Window{opts: Options{
		Installation: func() herdr.Installation { return install },
		ConnectHerdr: func() error { connected++; return nil },
		OpenURL:      func(url string) { opened = url },
	}}
	tester := native.NewTester(w.View, 900, 600)
	tester.Frame()
	if !tester.HasText("No open shells") || !tester.HasText("Herdr not installed") {
		t.Fatalf("texts %v", tester.Texts())
	}
	if err := tester.Click("Herdr not installed"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.Click("Installation Guide"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if opened != herdr.InstallURL {
		t.Fatalf("opened %q", opened)
	}
	install = herdr.Installation{Path: "/bin/herdr", Version: "0.9.5"}
	w.herdrOpen = true
	tester.Frame()
	if err := tester.Click("Open Herdr Workspaces"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if connected != 1 || w.herdrOpen {
		t.Fatal("did not connect to Herdr")
	}
}

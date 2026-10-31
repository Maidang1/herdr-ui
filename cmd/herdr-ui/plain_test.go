package main

import (
	"testing"

	"github.com/Maidang1/herdr-ui/internal/herdr"
)

func TestStartupPlain(t *testing.T) {
	installed := herdr.Installation{Path: "/usr/local/bin/herdr", Version: "herdr 0.9.5"}
	missing := herdr.Installation{}
	broken := herdr.Installation{Path: "/usr/local/bin/herdr", Error: "exit 1"}
	cases := []struct {
		mode         string
		plain, herdr bool
		install      herdr.Installation
		want         bool
	}{
		{"auto", false, false, installed, false},
		{"auto", false, false, missing, true},
		// A Herdr that does not answer still opens its window, which
		// explains the failure and offers Retry.
		{"auto", false, false, broken, false},
		{"plain", false, false, installed, true},
		{"herdr", false, false, missing, false},
		{"plain", false, true, installed, false},
		{"herdr", true, false, installed, true},
	}
	for _, c := range cases {
		if got := startupPlain(c.mode, c.plain, c.herdr, c.install); got != c.want {
			t.Fatalf("%+v: %v", c, got)
		}
	}
}

func TestPlainAccelerator(t *testing.T) {
	want := "Ctrl+Shift+T"
	if isDarwin {
		want = "Cmd+T"
	}
	if got := plainAccelerator("T"); got != want {
		t.Fatal(got)
	}
	if !isDarwin && plainAccelerator("Shift+]") != "Ctrl+Shift+]" {
		t.Fatal(plainAccelerator("Shift+]"))
	}
}

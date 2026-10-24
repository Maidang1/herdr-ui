package shellenv

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, tt := range []struct {
		name, out, want string
		ok              bool
	}{
		{name: "plain", out: marker + "/a:/b\n", want: "/a:/b", ok: true},
		{name: "noisy startup files", out: "Welcome\n" + marker + "x\n" + "motd " + marker + "/a:/b\nbye\n", want: "/a:/b", ok: true},
		{name: "no marker", out: "/a:/b\n"},
		{name: "empty", out: marker + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parse([]byte(tt.out))
			if got != tt.want || ok != tt.ok {
				t.Fatalf("parse = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	got := Merge("/home/u/.local/bin:/usr/bin", "/usr/bin:/bin::", "/opt/homebrew/bin:/home/u/.local/bin")
	if want := "/home/u/.local/bin:/usr/bin:/bin:/opt/homebrew/bin"; got != want {
		t.Fatalf("Merge = %q, want %q", got, want)
	}
	if got := Merge("", "/usr/bin"); got != "/usr/bin" {
		t.Fatalf("Merge without a shell PATH = %q", got)
	}
}

func TestFromTerminal(t *testing.T) {
	if FromTerminal(func(string) string { return "" }) {
		t.Fatal("launchd environment reported as a terminal")
	}
	if !FromTerminal(func(k string) string { return map[string]string{"TERM": "xterm-256color"}[k] }) {
		t.Fatal("terminal environment not reported")
	}
}

func TestLoginPath(t *testing.T) {
	dir := t.TempDir()
	shell := filepath.Join(dir, "sh")
	// A shell whose startup files print around the command's output.
	script := "#!/bin/sh\necho 'startup noise'\nPATH=/from/rc:$PATH\nexport PATH\nshift 3\neval \"$1\"\necho 'more noise'\n"
	if err := os.WriteFile(shell, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := LoginPath(context.Background(), shell)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "/from/rc:") {
		t.Fatalf("LoginPath = %q", got)
	}
}

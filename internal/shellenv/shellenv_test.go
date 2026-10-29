package shellenv

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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
	first, second, third, want := "/home/u/.local/bin:/usr/bin", "/usr/bin:/bin::", "/opt/homebrew/bin:/home/u/.local/bin", "/home/u/.local/bin:/usr/bin:/bin:/opt/homebrew/bin"
	if runtime.GOOS == "windows" {
		first, second, third, want = `C:\Tools;C:\Windows`, `C:\Windows;D:\bin;;`, `C:\Cargo;C:\Tools`, `C:\Tools;C:\Windows;D:\bin;C:\Cargo`
	}
	got := Merge(first, second, third)
	if got != want {
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
	if runtime.GOOS == "windows" {
		t.Setenv("PATH", `C:\Tools;C:\Windows`)
		got, err := LoginPath(context.Background(), `C:\missing-shell.exe`)
		if err != nil || got != os.Getenv("PATH") {
			t.Fatalf("Windows LoginPath = %q, %v", got, err)
		}
		return
	}
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

func TestWindowsApplyPreservesPathWithoutUnixShell(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows environment behavior")
	}
	t.Setenv("PATH", `C:\Tools;C:\Windows\System32`)
	t.Setenv("SHELL", `C:\missing-shell.exe`)
	before := os.Getenv("PATH")
	if err := Apply(context.Background(), "missing-herdr-ui-test-binary"); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("PATH"); got != before {
		t.Fatalf("PATH changed: %q", got)
	}
}

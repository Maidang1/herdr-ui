// Package shellenv gives an app opened from Finder or the Dock the PATH of
// the user's shell. launchd starts such apps with /usr/bin:/bin:/usr/sbin:/sbin
// only, so tools installed under ~/.local/bin, Homebrew or ~/.cargo/bin, such
// as herdr, are not found.
package shellenv

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// marker precedes the PATH in the shell's output, which an interactive
// shell's startup files may fill with other text.
const marker = "__HERDR_UI_PATH__"

// FromTerminal reports whether the process was started from a terminal,
// whose PATH is already the shell's. launchd sets no TERM.
func FromTerminal(getenv func(string) string) bool { return getenv("TERM") != "" }

// LoginPath runs shell as an interactive login shell, which reads the files
// users set PATH in (.zprofile, .zshrc, config.fish…), and returns its PATH.
func LoginPath(ctx context.Context, shell string) (string, error) {
	if runtime.GOOS == "windows" {
		return os.Getenv("PATH"), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", "printf '%s' "+marker+"; /usr/bin/printenv PATH")
	// Startup files that start background programs keep the output open.
	cmd.WaitDelay = 500 * time.Millisecond
	out, err := cmd.Output()
	if path, ok := parse(out); ok {
		return path, nil
	}
	if err == nil {
		err = errors.New("no PATH in its output")
	}
	return "", err
}

func parse(out []byte) (string, bool) {
	i := bytes.LastIndex(out, []byte(marker))
	if i < 0 {
		return "", false
	}
	path, _, _ := strings.Cut(string(out[i+len(marker):]), "\n")
	path = strings.TrimSpace(path)
	return path, path != ""
}

// Merge returns the entries of first, then those of the others that first
// lacks, without empty entries.
func Merge(first string, others ...string) string {
	var dirs []string
	seen := map[string]bool{}
	for _, list := range append([]string{first}, others...) {
		for _, dir := range filepath.SplitList(list) {
			if dir != "" && !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}
	return strings.Join(dirs, string(os.PathListSeparator))
}

// CommonDirs lists where command-line tools are usually installed.
func CommonDirs(home string) string {
	if runtime.GOOS == "windows" {
		return strings.Join([]string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".cargo", "bin")}, string(os.PathListSeparator))
	}
	return strings.Join([]string{
		filepath.Join(home, ".local", "bin"),
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		filepath.Join(home, ".cargo", "bin"),
	}, string(os.PathListSeparator))
}

// Apply makes binary findable by a process that was not started from a
// terminal. It appends the common directories to PATH, which costs nothing
// and finds most installs; only when binary is still missing does it ask the
// user's shell, whose interactive startup can take seconds, and put the
// shell's PATH first. It returns the error of the shell, if any.
func Apply(ctx context.Context, binary string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if FromTerminal(os.Getenv) {
		return nil
	}
	home, _ := os.UserHomeDir()
	current := Merge(os.Getenv("PATH"), CommonDirs(home))
	if err := os.Setenv("PATH", current); err != nil {
		return err
	}
	if _, err := exec.LookPath(binary); err == nil || strings.ContainsRune(binary, os.PathSeparator) {
		return nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh"
	}
	login, err := LoginPath(ctx, shell)
	if err != nil {
		return fmt.Errorf("shellenv: %s: %w", shell, err)
	}
	return os.Setenv("PATH", Merge(login, current))
}

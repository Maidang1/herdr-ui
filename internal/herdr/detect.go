package herdr

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// InstallURL is where Herdr's installation instructions live.
const InstallURL = "https://github.com/herdrdev/herdr#installation"

// Installation is what Hergo found of the Herdr executable.
type Installation struct {
	// Path is the executable's absolute path; empty when none was found.
	Path string
	// Version is the first line `herdr --version` printed.
	Version string
	// Error says why the executable could not be used, when it was found
	// but did not answer.
	Error string
}

func (i Installation) Installed() bool { return i.Path != "" && i.Error == "" }

// ErrNotInstalled is the reason a missing executable gives.
var ErrNotInstalled = errors.New("Herdr is not installed")

// candidateDirs are where Herdr's installers place the executable, beyond
// PATH: an app opened from the Dock or a desktop entry may not have them.
func candidateDirs(home, goos string) []string {
	dirs := []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".cargo", "bin"), filepath.Join(home, "bin")}
	switch goos {
	case "darwin":
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	case "linux":
		dirs = append(dirs, "/usr/local/bin", "/usr/bin", "/home/linuxbrew/.linuxbrew/bin")
	case "windows":
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			dirs = append(dirs, filepath.Join(local, "Programs", "herdr"), filepath.Join(local, "herdr"))
		}
	}
	return dirs
}

// LocateBinary resolves binary as exec.LookPath does, then in the usual
// installation directories. A path with a separator is taken as given.
func LocateBinary(binary string) (string, error) {
	if binary == "" {
		binary = "herdr"
	}
	if path, err := exec.LookPath(binary); err == nil {
		if abs, err := filepath.Abs(path); err == nil {
			return abs, nil
		}
		return path, nil
	}
	if strings.ContainsRune(binary, filepath.Separator) || strings.ContainsRune(binary, '/') {
		return "", ErrNotInstalled
	}
	home, _ := os.UserHomeDir()
	name := binary
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	for _, dir := range candidateDirs(home, runtime.GOOS) {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0o111 != 0) {
			return path, nil
		}
	}
	return "", ErrNotInstalled
}

// Detect finds Herdr and asks its version, within a few seconds.
func Detect(ctx context.Context, binary string) Installation {
	path, err := LocateBinary(binary)
	if err != nil {
		return Installation{}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return Installation{Path: path, Error: "herdr --version failed: " + err.Error()}
	}
	version := strings.TrimSpace(string(out))
	if i := strings.IndexByte(version, '\n'); i >= 0 {
		version = strings.TrimSpace(version[:i])
	}
	return Installation{Path: path, Version: version}
}

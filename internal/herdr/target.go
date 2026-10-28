// Package herdr contains the local Herdr wire and process boundary.
package herdr

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type Target struct{ Socket string }

var sessionName = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// ResolveTarget mirrors Herdr's platform paths. Config file overrides do not
// change config_dir, and a socket override wins over an inherited session.
func ResolveTarget(explicit, home string, getenv func(string) string) (Target, error) {
	socket := explicit
	if socket == "" {
		socket = getenv("HERDR_SOCKET_PATH")
	}
	if socket != "" {
		abs, err := filepath.Abs(socket)
		return Target{Socket: abs}, err
	}
	base := ConfigDir(home, getenv)
	name := getenv("HERDR_SESSION")
	if name != "" && name != "default" {
		if !sessionName.MatchString(name) || name == "." || name == ".." {
			return Target{}, fmt.Errorf("invalid HERDR_SESSION %q", name)
		}
		base = filepath.Join(base, "sessions", name)
	}
	socket, err := filepath.Abs(filepath.Join(base, "herdr.sock"))
	return Target{Socket: socket}, err
}

// Attach uses argv, never a shell. Drop inherited caller identity so a
// child targeting another session cannot accidentally address its parent pane.
func (t Target) Attach(binary, terminalID string, inherited []string) ([]string, []string, error) {
	if binary == "" || terminalID == "" || strings.HasPrefix(terminalID, "-") || t.Socket == "" {
		return nil, nil, fmt.Errorf("attach requires executable, terminal ID and socket")
	}
	env := make([]string, 0, len(inherited)+1)
	for _, v := range inherited {
		k, _, _ := strings.Cut(v, "=")
		switch k {
		case "HERDR_SOCKET_PATH", "HERDR_SESSION", "HERDR_PANE_ID", "HERDR_TAB_ID", "HERDR_WORKSPACE_ID":
			continue
		}
		env = append(env, v)
	}
	env = append(env, "HERDR_SOCKET_PATH="+t.Socket)
	return []string{binary, "terminal", "attach", terminalID}, env, nil
}

// ConfigDir returns the daemon's configuration directory, independent of a
// HERDR_CONFIG_PATH file override and of the active session.
func ConfigDir(home string, getenv func(string) string) string {
	return configDir(runtime.GOOS, home, getenv)
}

func configDir(platform, home string, getenv func(string) string) string {
	base := getenv("XDG_CONFIG_HOME")
	if base == "" && platform == "windows" {
		base = getenv("APPDATA")
		if base == "" && getenv("USERPROFILE") != "" {
			base = filepath.Join(getenv("USERPROFILE"), "AppData", "Roaming")
		}
	}
	if base == "" {
		if home == "" {
			home = getenv("HOME")
		}
		if home != "" {
			base = filepath.Join(home, ".config")
		} else {
			base = os.TempDir()
		}
	}
	return filepath.Join(base, "herdr")
}

// ConfigPath honors Herdr's configuration file override.
func ConfigPath(home string, getenv func(string) string) string {
	if path := getenv("HERDR_CONFIG_PATH"); path != "" {
		return path
	}
	return filepath.Join(ConfigDir(home, getenv), "config.toml")
}

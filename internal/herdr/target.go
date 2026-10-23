// Package herdr contains the local Herdr wire and process boundary.
package herdr

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

type Target struct{ Socket string }

var sessionName = regexp.MustCompile(`^[a-zA-Z0-9._-]{1,64}$`)

// ResolveTarget mirrors Herdr's macOS paths. Config file overrides do not
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
	base := getenv("XDG_CONFIG_HOME")
	if base == "" {
		if home == "" {
			return Target{}, fmt.Errorf("home directory unavailable")
		}
		base = filepath.Join(home, ".config")
	}
	base = filepath.Join(base, "herdr")
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

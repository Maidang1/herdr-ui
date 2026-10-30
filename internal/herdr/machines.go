package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type LocalSession struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Running   bool   `json:"running"`
	Directory string `json:"session_dir"`
	Socket    string `json:"socket_path"`
}

// CLI rows verified against upstream src/cli/machine.rs (Herdr 0.9.5).
type Machine struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Target   string `json:"target"`
	Session  string `json:"session"`
	Enabled  bool   `json:"enabled"`
	Selected bool   `json:"selected"`
}
type MachineStatus struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Error  string `json:"error"`
}
type MachineCatalog struct {
	Machines    []Machine
	Status      []MachineStatus
	StatusError string
	Sessions    []LocalSession
}
type MachineCommand struct{ Action, ID, Value, Session string }

func CatalogEnvironment(inherited []string) []string {
	env := make([]string, 0, len(inherited))
	for _, entry := range inherited {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "HERDR_SOCKET_PATH", "HERDR_SESSION", "HERDR_PANE_ID", "HERDR_TAB_ID", "HERDR_WORKSPACE_ID":
			continue
		}
		env = append(env, entry)
	}
	return env
}
func ReadMachineCatalog(ctx context.Context, binary string) (MachineCatalog, error) {
	var result MachineCatalog
	read := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = CatalogEnvironment(os.Environ())
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s: %w: %s", strings.Join(args, " "), err, stderr.String())
		}
		if !json.Valid(stdout.Bytes()) {
			return nil, fmt.Errorf("%s returned invalid JSON", strings.Join(args, " "))
		}
		return stdout.Bytes(), nil
	}
	machines, err := read("machine", "list", "--json")
	if err != nil {
		return result, err
	}
	var entries []Machine
	if err = json.Unmarshal(machines, &entries); err != nil {
		return result, err
	}
	for _, entry := range entries {
		if entry.ID == "" || entry.Target == "" {
			return result, fmt.Errorf("machine catalog is missing identity fields")
		}
	}
	result.Machines = entries
	sessions, err := read("session", "list", "--json")
	if err != nil {
		return result, err
	}
	var list struct {
		Sessions []LocalSession `json:"sessions"`
	}
	if err = json.Unmarshal(sessions, &list); err != nil {
		return result, err
	}
	result.Sessions = list.Sessions

	return result, nil
}
func MachineArguments(command MachineCommand) ([]string, error) {
	id := strings.TrimSpace(command.ID)
	value := strings.TrimSpace(command.Value)
	safe := func(s string) bool {
		return s != "" && !strings.HasPrefix(s, "-") && !strings.ContainsAny(s, "\x00\r\n")
	}
	switch command.Action {
	case "add":
		if !safe(value) {
			break
		}
		args := []string{"machine", "add", value}
		if command.Session != "" {
			if !safe(command.Session) {
				break
			}
			args = append(args, "--remote-session", command.Session)
		}
		return args, nil
	case "rename":
		if !safe(id) || !safe(value) {
			break
		}
		return []string{"machine", "rename", id, "--label", value}, nil
	case "status":
		args := []string{"machine", "status"}
		if id != "" {
			if !safe(id) {
				break
			}
			args = append(args, id)
		}
		return args, nil
	case "enable", "disable", "remove", "reconnect":
		if !safe(id) {
			break
		}
		return []string{"machine", command.Action, id}, nil
	case "open":
		if !safe(value) {
			break
		}
		args := []string{"--remote", value}
		if command.Session != "" {
			if !safe(command.Session) {
				break
			}
			args = append(args, "--session", command.Session)
		}
		return args, nil
	case "session":
		if !sessionName.MatchString(value) || value == "." || value == ".." {
			break
		}
		return []string{"session", "attach", value}, nil
	}
	return nil, fmt.Errorf("enter a valid machine ID, SSH target or session name")
}

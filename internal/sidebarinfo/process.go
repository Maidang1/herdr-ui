package sidebarinfo

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/herdr"
)

type process struct {
	pid, parent int
	birth       string
}
type shellRoot struct {
	pid     int
	parent  int
	birth   string
	expires time.Time
	known   bool
}
type processCall func(context.Context, herdr.Target, string) (int, error)

func readShellPID(ctx context.Context, target herdr.Target, paneID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var result struct {
		Process struct {
			PaneID   string  `json:"pane_id"`
			ShellPID *uint32 `json:"shell_pid"`
		} `json:"process_info"`
	}
	if err := herdr.Call(ctx, target, "pane.process_info", map[string]string{"pane_id": paneID}, &result); err != nil {
		return 0, err
	}
	if result.Process.PaneID != paneID {
		return 0, fmt.Errorf("process info pane mismatch")
	}
	if result.Process.ShellPID == nil {
		return 0, nil
	}
	return int(*result.Process.ShellPID), nil
}

func parseProcesses(data []byte) (map[int]process, error) {
	processes := make(map[int]process)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 7 {
			return nil, fmt.Errorf("invalid process metadata row")
		}
		pid, e1 := strconv.Atoi(fields[0])
		parent, e2 := strconv.Atoi(fields[1])
		if e1 != nil || e2 != nil || pid <= 0 || parent < 0 {
			return nil, fmt.Errorf("invalid process identity")
		}
		if _, duplicate := processes[pid]; duplicate {
			return nil, fmt.Errorf("duplicate process identity")
		}
		processes[pid] = process{pid: pid, parent: parent, birth: strings.Join(fields[2:], " ")}
	}
	return processes, nil
}

func parseListeners(data []byte) (map[int][]int, error) {
	listeners := make(map[int][]int)
	pid := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'p':
			value, err := strconv.Atoi(line[1:])
			if err != nil || value <= 0 {
				return nil, fmt.Errorf("invalid listening process")
			}
			pid = value
		case 'n':
			if pid == 0 {
				return nil, fmt.Errorf("listener has no process identity")
			}
			// -nP ensures no service-name lookup. IPv6 is [address]:port.
			at := strings.LastIndexByte(line, ':')
			if at < 0 {
				return nil, fmt.Errorf("listener missing port")
			}
			port, err := strconv.Atoi(line[at+1:])
			if err != nil || port < 1 || port > 65535 {
				return nil, fmt.Errorf("invalid listener port")
			}
			listeners[pid] = append(listeners[pid], port)
		}
	}
	return listeners, nil
}

func descendsFrom(pid, root int, processes map[int]process) bool {
	seen := map[int]bool{}
	for pid > 0 && !seen[pid] {
		if pid == root {
			return true
		}
		seen[pid] = true
		current, ok := processes[pid]
		if !ok {
			return false
		}
		pid = current.parent
	}
	return false
}

package ui

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// The meters of the session.
//
// The Rust client shows the load of the machine and the ports the
// workspaces listen on at the bottom of the window. The window here asks the
// machine the same questions, and keeps the answers fresh on its own.

// load is what the machine is busy with.
type load struct {
	// CPU is how much of the processor is in use, as a percentage.
	CPU float64
	// Memory is how much of the memory is in use, as a percentage.
	Memory float64
}

// portReading is one port the machine listens on.
type portReading struct {
	// Port is the number of the port.
	Port int
	// Dir is the folder the process of the port works in.
	Dir string
}

// watchMeters keeps the load and the ports of the machine fresh.
func (a *App) watchMeters() {
	go func() {
		for {
			a.pollLoad()
			a.pollPorts()
			time.Sleep(4 * time.Second)
		}
	}()
}

// pollLoad asks the machine how busy it is.
func (a *App) pollLoad() {
	found := load{}
	if output, err := exec.Command("top", "-l", "1", "-n", "0").Output(); err == nil {
		found.CPU = parseCPU(string(output))
	}
	found.Memory = parseMemory()
	a.mu.Lock()
	a.load = found
	a.mu.Unlock()
	a.invalidate()
}

// parseCPU reads the use of the processor out of what `top` says.
func parseCPU(output string) float64 {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "CPU usage") {
			continue
		}
		var total float64
		for _, part := range strings.Split(line, ",") {
			fields := strings.Fields(part)
			if len(fields) < 2 {
				continue
			}
			// The line names what the share is of: the work of the user
			// and the work of the kernel.
			if fields[len(fields)-1] != "user" && fields[len(fields)-1] != "sys" {
				continue
			}
			number := strings.TrimSuffix(fields[len(fields)-2], "%")
			value, err := strconv.ParseFloat(number, 64)
			if err != nil {
				continue
			}
			total += value
		}
		return total
	}
	return 0
}

// parseMemory reads the use of the memory out of the machine.
//
// The pages of `vm_stat` are counted, and the size of the memory is asked
// of the kernel.
func parseMemory() float64 {
	output, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}
	sizeOutput, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	total, err := strconv.ParseFloat(strings.TrimSpace(string(sizeOutput)), 64)
	if err != nil || total <= 0 {
		return 0
	}
	pageSize := float64(4096)
	used := float64(0)
	for _, line := range strings.Split(string(output), "\n") {
		colon := strings.Index(line, ":")
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		number := strings.TrimSpace(line[colon+1:])
		number = strings.TrimSuffix(number, ".")
		value, err := strconv.ParseFloat(number, 64)
		if err != nil {
			continue
		}
		switch name {
		case "Pages active", "Pages wired down", "Pages occupied by compressor":
			used += value * pageSize
		}
	}
	if used <= 0 {
		return 0
	}
	return used / total * 100
}

// pollPorts asks the machine which ports it listens on.
func (a *App) pollPorts() {
	readings := readListeningPorts()
	a.mu.Lock()
	a.ports = readings
	a.mu.Unlock()
	a.invalidate()
}

// readListeningPorts asks the machine for the ports it listens on.
//
// Every port comes with the folder its process works in, so that the window
// can tell which workspace listens on it.
func readListeningPorts() []portReading {
	output, err := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN", "-F", "pn").Output()
	if err != nil {
		return nil
	}
	var readings []portReading
	seen := map[int]bool{}
	pid := 0
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 2 {
			continue
		}
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(line[1:])
		case 'n':
			if pid == 0 {
				continue
			}
			port, ok := portOf(line[1:])
			if !ok || seen[port] {
				continue
			}
			seen[port] = true
			readings = append(readings, portReading{Port: port, Dir: directoryOfProcess(pid)})
		}
	}
	return readings
}

// portOf reads the port out of the name of a socket, as `127.0.0.1:8080`.
func portOf(name string) (int, bool) {
	colon := strings.LastIndex(name, ":")
	if colon < 0 {
		return 0, false
	}
	port, err := strconv.Atoi(name[colon+1:])
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

// directoryOfProcess asks the machine where a process works.
func directoryOfProcess(pid int) string {
	output, err := exec.Command("lsof", "-a", "-p", strconv.Itoa(pid), "-d", "cwd", "-F", "n").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimSpace(line[1:])
		}
	}
	return ""
}

// portsOfWorkspace returns the ports the workspace listens on.
//
// A port belongs to the workspace whose folder the process of the port works
// in, or works in a folder under it.
func (a *App) portsOfWorkspace(workspaceID string) []string {
	dirs := a.workspaceDirs(workspaceID)
	if len(dirs) == 0 {
		return nil
	}
	a.mu.Lock()
	readings := append([]portReading(nil), a.ports...)
	a.mu.Unlock()

	var ports []string
	for _, reading := range readings {
		if reading.Dir == "" {
			continue
		}
		for _, dir := range dirs {
			if reading.Dir == dir || strings.HasPrefix(reading.Dir, strings.TrimSuffix(dir, "/")+"/") {
				ports = append(ports, ":"+strconv.Itoa(reading.Port))
				break
			}
		}
	}
	return ports
}

// portsOfSession returns the ports the session listens on.
func (a *App) portsOfSession() []string {
	a.mu.Lock()
	readings := append([]portReading(nil), a.ports...)
	a.mu.Unlock()
	var ports []string
	for _, reading := range readings {
		ports = append(ports, ":"+strconv.Itoa(reading.Port))
	}
	return ports
}

// workspaceDirs returns the folders the workspace works in.
func (a *App) workspaceDirs(workspaceID string) []string {
	a.mu.Lock()
	snapshot := a.snapshot
	a.mu.Unlock()
	if snapshot == nil {
		return nil
	}
	var dirs []string
	for _, pane := range snapshot.Panes {
		if pane.WorkspaceID != workspaceID || pane.CWD == nil {
			continue
		}
		dirs = append(dirs, *pane.CWD)
	}
	return dirs
}

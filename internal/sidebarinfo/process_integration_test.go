package sidebarinfo

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/herdr-ui/internal/herdr"
)

func TestSidebarListenerHelper(t *testing.T) {
	if os.Getenv("HERDR_UI_SIDEBAR_LISTENER_HELPER") != "1" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(2)
	}
	defer listener.Close()
	fmt.Fprintln(os.Stdout, listener.Addr().(*net.TCPAddr).Port)
	io.Copy(io.Discard, os.Stdin)
	os.Exit(0)
}
func TestRealLocalTCPListenerIsScopedToPaneShellOwner(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("local lsof not installed")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestSidebarListenerHelper$")
	cmd.Env = append(os.Environ(), "HERDR_UI_SIDEBAR_LISTENER_HELPER=1")
	output, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { input.Close(); cmd.Process.Kill(); cmd.Wait() }()
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatal(err)
	}
	unrelated, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer unrelated.Close()
	c := &Collector{roots: map[string]shellRoot{}, run: runCommand, processInfo: func(context.Context, herdr.Target, string) (int, error) { return cmd.Process.Pid, nil }}
	ports, known := c.collectPorts(ctx, Source{TargetID: 1, Workspaces: []WorkspaceSource{{ID: "w", Panes: []PaneSource{{ID: "helper", TerminalID: "term"}}}}})
	if !known["w"] {
		t.Fatal("local process/socket collection unavailable")
	}
	if !slices.Contains(ports["w"], port) || slices.Contains(ports["w"], unrelated.Addr().(*net.TCPAddr).Port) {
		t.Fatal("real listener ownership mismatch", ports, port)
	}
}

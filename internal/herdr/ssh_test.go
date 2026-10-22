package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestTheBridgeToARemoteHostSpeaksTheProtocol says the window can reach a
// daemon on another machine: the ssh process stands in for the host, and the
// protocol travels over it.
func TestTheBridgeToARemoteHostSpeaksTheProtocol(t *testing.T) {
	folder := t.TempDir()
	binary := filepath.Join(folder, "fakessh")
	build := exec.Command("go", "build", "-o", binary, "./testdata/fakessh")
	build.Env = append(os.Environ(), "GOFLAGS=-buildvcs=false")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the fake ssh cannot be built: %v\n%s", err, out)
	}

	// The welcome the remote daemon would send.
	welcome, _ := json.Marshal(map[string]any{
		"generation":     1,
		"server_version": "9.9.9",
		"methods":        []string{"pane.focus"},
		"capabilities":   []string{},
	})
	var frame bytes.Buffer
	if err := writeFrame(&frame, encodeEndpointControl("endpoint.welcome.v1", string(welcome))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(folder, "welcome.bin")
	if err := os.WriteFile(path, frame.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	// The status the remote host offers.
	offer, _ := json.Marshal(map[string]any{
		"endpoint_protocol_generation": 1,
		"endpoint_capabilities": []string{
			"surface_interest", "presentation_effects_fence", "health_check",
		},
		"remote_bridge_idle_timeout": false,
	})

	t.Setenv("FAKESSH_OFFER", string(offer))
	t.Setenv("FAKESSH_WELCOME", path)

	previous := sshCommand
	sshCommand = func() string { return binary }
	defer func() { sshCommand = previous }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	endpoint, err := ConnectSSH(ctx, EndpointOptions{
		CellWidthPx: 8, CellHeightPx: 17, Cols: 80, Rows: 24,
	}, "user@host", "default")
	if err != nil {
		t.Fatalf("the bridge to the remote host: %v", err)
	}
	defer endpoint.Close()

	greeting := endpoint.Welcome()
	if greeting == nil {
		t.Fatal("the remote host did not answer")
	}
	if greeting.ServerVersion != "9.9.9" {
		t.Fatalf("the remote host came out as %+v", greeting)
	}
}

func TestTheTargetOfTheBridgeIsChecked(t *testing.T) {
	for _, target := range []string{"user@host", "host", "ssh://user@host"} {
		if err := ValidateSSHTarget(target); err != nil {
			t.Errorf("the target %q was refused: %v", target, err)
		}
	}
	for _, target := range []string{"", "-oProxyCommand=evil", "user:name@host", "host\nname"} {
		if err := ValidateSSHTarget(target); err == nil {
			t.Errorf("the target %q was taken", target)
		}
	}
}

func TestTheNameOfASessionIsChecked(t *testing.T) {
	for _, name := range []string{"default", "work-1", "a.b_c"} {
		if !ValidSessionName(name) {
			t.Errorf("the session %q was refused", name)
		}
	}
	for _, name := range []string{"", "..", "a/b", "a b"} {
		if ValidSessionName(name) {
			t.Errorf("the session %q was taken", name)
		}
	}
}

func TestTheStatusOfTheRemoteHostIsRead(t *testing.T) {
	idle, ok := sshStatusOf(`{"endpoint_protocol_generation":1,"endpoint_capabilities":["surface_interest","presentation_effects_fence","health_check"],"remote_bridge_idle_timeout":true}`)
	if !ok {
		t.Fatal("a host that speaks the protocol was refused")
	}
	if !idle {
		t.Fatal("the idle timeout was not read")
	}
	if _, ok := sshStatusOf(`{"endpoint_protocol_generation":2,"endpoint_capabilities":["surface_interest","presentation_effects_fence","health_check"]}`); ok {
		t.Fatal("a host of another generation was taken")
	}
	if _, ok := sshStatusOf(`{"endpoint_protocol_generation":1,"endpoint_capabilities":["surface_interest"]}`); ok {
		t.Fatal("a host without the capabilities was taken")
	}
}

func TestTheScriptOfTheBridgeAsksTheHostForItsHerdr(t *testing.T) {
	script := sshBridgeScript("work-1")
	for _, want := range []string{
		"remote-client-bridge",
		"--session 'work-1'",
		"status client --json",
		"herdr-remote-output-ready:1",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("the script does not say %q:\n%s", want, script)
		}
	}
}

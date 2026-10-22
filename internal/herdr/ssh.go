package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
	"unicode"
)

// The window can talk to a daemon on another machine, over SSH.
//
// The ssh binary does the encryption and the authentication; the remote
// herdr bridges its client socket to the connection, and the window speaks
// the same protocol over it.
//
// The remote host is asked for its herdr, and its answer is read before the
// bridge begins: the host offers the copies of herdr it knows, and the
// window takes the first that speaks this protocol.

// sshPolicy is the connection policy of the bridge.
//
// The window never asks for a password: a host that needs one is a host the
// user has to set up first, and the window says so rather than waiting.
var sshPolicy = []string{
	"-T",
	"-C",
	"-o", "BatchMode=yes",
	"-o", "NumberOfPasswordPrompts=0",
	"-o", "StrictHostKeyChecking=yes",
	"-o", "ConnectTimeout=10",
	"-o", "ConnectionAttempts=1",
	"-o", "ServerAliveInterval=15",
	"-o", "ServerAliveCountMax=4",
	"-o", "ForwardX11=no",
	"-o", "ControlMaster=no",
}

// sshCandidates is the shell code that finds the herdr on the remote host.
//
// The names are kept in shell variables, and never run as commands.
const sshCandidates = `candidate=$(command -v herdr 2>/dev/null || :)
case "$candidate" in /*/mise/shims/herdr) candidate=;; /*) ;; *) candidate=;; esac
for path in "$candidate" "$HOME/.local/bin/herdr" /opt/homebrew/bin/herdr /usr/local/bin/herdr /home/linuxbrew/.linuxbrew/bin/herdr "$HOME/.nix-profile/bin/herdr" "/etc/profiles/per-user/$USER/bin/herdr" /nix/var/nix/profiles/default/bin/herdr /run/current-system/sw/bin/herdr; do`

// sshReady is the line the remote host writes when it has offered its
// status and waits for the answer.
const sshReady = "herdr-remote-output-ready:1"

// sshBridgeScript is what runs on the remote host.
//
// It offers the copies of herdr it finds, one at a time, and takes the one
// the window answers for.
func sshBridgeScript(session string) string {
	return fmt.Sprintf(`%s
    if [ -n "$path" ] && [ -x "$path" ]; then
        status=$("%s" status client --json </dev/null) || continue
        printf '%%s\n' "$status"
        printf '\n%%s\n' '%s'
        IFS= read -r choice || exit 1
        case "$choice" in
            accept) exec "%s" --session %s remote-client-bridge;;
            accept-idle) exec "%s" --session %s remote-client-bridge --idle-timeout-v1;;
        esac
    fi
done
exit 127`, sshCandidates, "$path", sshReady, "$path", shellQuote(session), "$path", shellQuote(session))
}

// shellQuote puts a word into a shell command safely.
func shellQuote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// ValidateSSHTarget says the target of an SSH connection may be used.
//
// The target is given to ssh as one word, so it may not begin with a dash
// and may not hold a control character; the part before the `@` is the
// user, which may not hold a colon.
func ValidateSSHTarget(target string) error {
	authority := strings.TrimPrefix(target, "ssh://")
	if target == "" ||
		strings.HasPrefix(target, "-") ||
		len(target) > 1024 ||
		strings.ContainsFunc(target, unicode.IsControl) {
		return fmt.Errorf("herdr: %q is not an SSH target", target)
	}
	if index := strings.LastIndex(authority, "@"); index >= 0 {
		if strings.Contains(authority[:index], ":") {
			return fmt.Errorf("herdr: %q is not an SSH target", target)
		}
	}
	return nil
}

// ValidSessionName says the name of a session may be used.
//
// The name is a folder of the daemon's configuration, so it may not hold a
// path of its own.
func ValidSessionName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == '.':
		default:
			return false
		}
	}
	return name != "." && name != ".."
}

// ConnectSSH opens a connection to a daemon on another machine, over SSH.
func ConnectSSH(ctx context.Context, opts EndpointOptions, target, session string) (*Endpoint, error) {
	if err := ValidateSSHTarget(target); err != nil {
		return nil, err
	}
	if session == "" {
		session = "default"
	}
	if !ValidSessionName(session) {
		return nil, fmt.Errorf("herdr: %q is not the name of a session", session)
	}
	dial := func() (stream, error) {
		return startSSHBridge(target, session)
	}
	conn, err := dial()
	if err != nil {
		return nil, err
	}
	return connectStream(ctx, opts, conn, dial, target)
}

// startSSHBridge opens the bridge to the remote host.
//
// The bridge is the ssh process and the herdr it starts on the other side;
// the window reads the offer of the host, answers it, and speaks the
// protocol over the rest of the connection.
func startSSHBridge(target, session string) (stream, error) {
	args := append([]string{}, sshPolicy...)
	args = append(args, "-o", "ClearAllForwardings=yes", "--", target,
		"/bin/sh -c "+shellQuote(sshBridgeScript(session)))
	command := exec.Command(sshCommand(), args...)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("herdr: the bridge to %s cannot be opened: %w", target, err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("herdr: the bridge to %s cannot be opened: %w", target, err)
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("herdr: the bridge to %s cannot be opened: %w", target, err)
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("herdr: the bridge to %s cannot be opened: %w", target, err)
	}
	// The errors of ssh are read and dropped: they are the window's own
	// diagnosis, not the remote host's.
	go io.Copy(io.Discard, stderr)

	bridge := &pipeStream{
		reader: bufio.NewReader(stdout),
		writer: stdin,
		closer: func() {
			if command.Process != nil {
				_ = command.Process.Kill()
			}
			_ = command.Wait()
		},
	}
	if err := sshHandshake(bridge); err != nil {
		bridge.Close()
		return nil, err
	}
	return bridge, nil
}

// sshCommand is the program that opens the bridge.
//
// It is a variable so that a test can put another program in its place.
var sshCommand = func() string { return "ssh" }

// sshHandshake answers the offer of the remote host.
//
// The host writes the status of every copy of herdr it knows, one after
// another, and waits for an answer before each: the window takes the first
// copy that speaks this protocol, and skips the others.
func sshHandshake(bridge *pipeStream) error {
	reader, ok := bridge.reader.(*bufio.Reader)
	if !ok {
		return fmt.Errorf("herdr: the bridge of the remote host cannot be read")
	}
	var offered []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("herdr: the remote host closed the bridge: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line != sshReady {
			// The status of the copy the host is offering.
			if strings.TrimSpace(line) != "" {
				offered = append(offered, line)
			}
			continue
		}
		// The host waits for an answer: the first copy that speaks
		// this protocol is taken, the others are skipped.
		answer := "skip\n"
		for _, status := range offered {
			if idle, ok := sshStatusOf(status); ok {
				if idle {
					answer = "accept-idle\n"
				} else {
					answer = "accept\n"
				}
				break
			}
		}
		if _, err := bridge.Write([]byte(answer)); err != nil {
			return fmt.Errorf("herdr: the remote host did not take the answer: %w", err)
		}
		if answer != "skip\n" {
			return nil
		}
		offered = nil
	}
}

// pipeStream is the bridge of a remote host: the standard input of the ssh
// process, and its standard output.
type pipeStream struct {
	reader io.Reader
	writer io.WriteCloser
	closer func()
}

func (p *pipeStream) Read(b []byte) (int, error) { return p.reader.Read(b) }

func (p *pipeStream) Write(b []byte) (int, error) { return p.writer.Write(b) }

// SetWriteDeadline does nothing: the bridge of a remote host has no
// deadline of its own, and ssh keeps it open.
func (p *pipeStream) SetWriteDeadline(time.Time) error { return nil }

func (p *pipeStream) Close() error {
	if p.closer != nil {
		p.closer()
	}
	return nil
}

// sshStatus is what the remote host says about one copy of herdr.
type sshStatus struct {
	Generation   uint64   `json:"endpoint_protocol_generation"`
	Capabilities []string `json:"endpoint_capabilities"`
	IdleTimeout  bool     `json:"remote_bridge_idle_timeout"`
}

// sshStatusOf reads the status of a copy of herdr.
//
// It says whether the copy speaks this protocol, and whether its bridge
// times out when it is idle.
func sshStatusOf(line string) (idle bool, ok bool) {
	var status sshStatus
	if err := json.Unmarshal([]byte(line), &status); err != nil {
		return false, false
	}
	if status.Generation != EndpointProtocolGeneration {
		return false, false
	}
	for _, required := range []string{"surface_interest", "presentation_effects_fence", "health_check"} {
		found := false
		for _, capability := range status.Capabilities {
			if capability == required {
				found = true
				break
			}
		}
		if !found {
			return false, false
		}
	}
	return status.IdleTimeout, true
}

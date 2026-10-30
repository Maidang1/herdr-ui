package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/localipc"
)

func TestRestoreConfigPreservesUnknownAndComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := "# keep me\n[keys]\nprefix = 'ctrl+a'\n[session] # session\nresume_agents_on_restore = true # keep comment\ncustom = 42\n[experimental]\npane_history = false\nother = 'x'\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	want := RestoreSettings{PaneHistory: true}
	if err := SaveRestoreSettings(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRestoreSettings(path)
	if err != nil || got != want {
		t.Fatalf("%+v %v", got, err)
	}
	data, _ := os.ReadFile(path)
	for _, keep := range []string{"# keep me", "prefix = 'ctrl+a'", "# keep comment", "custom = 42", "other = 'x'"} {
		if !strings.Contains(string(data), keep) {
			t.Fatalf("lost %q", keep)
		}
	}
	defaults, err := ReadRestoreSettings(filepath.Join(t.TempDir(), "missing"))
	if err != nil || !defaults.ResumeAgents || defaults.PaneHistory {
		t.Fatalf("defaults %+v %v", defaults, err)
	}
}
func TestRestoreConfigPreservesMultilineString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	banner := "banner = \"\"\"\n[session]\nresume_agents_on_restore = false\n\"\"\"\n"
	original := banner + "[session]\nresume_agents_on_restore = true\n[experimental]\npane_history = false\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveRestoreSettings(path, RestoreSettings{ResumeAgents: true, PaneHistory: true}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), banner) {
		t.Fatalf("changed multiline string: %s (%v)", data, err)
	}
}

func TestRestoreConfigRejectsInvalidWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	text := "[session\nbroken"
	os.WriteFile(path, []byte(text), 0600)
	if SaveRestoreSettings(path, RestoreSettings{}) == nil {
		t.Fatal("accepted invalid TOML")
	}
	data, _ := os.ReadFile(path)
	if string(data) != text {
		t.Fatal("changed invalid config")
	}
}
func TestWatchLaneCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	listener, err := localipc.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	waiting := make(chan struct{})
	ended := make(chan struct{})
	requests := make(chan string, 4)
	go func() {
		for i := 0; i < 2; i++ {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go func() {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				line, e := reader.ReadBytes('\n')
				if e != nil {
					return
				}
				var request struct {
					ID, Method string
					Params     struct {
						Source string                       `json:"source"`
						Lines  int                          `json:"lines"`
						Match  struct{ Type, Value string } `json:"match"`
					}
				}
				if json.Unmarshal(line, &request) != nil {
					return
				}
				requests <- request.Method
				if request.Method == "pane.wait_for_output" {
					if request.Params.Source != "recent" || request.Params.Lines != 80 || request.Params.Match.Type != "regex" {
						requests <- "bad params"
					}
					close(waiting)
					reader.ReadByte()
					close(ended)
					return
				}
				json.NewEncoder(conn).Encode(map[string]any{"id": request.ID, "result": map[string]any{"type": "pong"}})
			}()
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, e := WaitForOutput(ctx, Target{path}, "pane", OutputWatch{Text: "done.*", Regex: true, Timeout: time.Minute})
		result <- e
	}()
	select {
	case <-waiting:
	case <-time.After(time.Second):
		t.Fatal("wait not connected")
	}
	var pong struct{ Type string }
	if err = Call(context.Background(), Target{path}, "ping", nil, &pong); err != nil || pong.Type != "pong" {
		t.Fatalf("ordinary request blocked: %v", err)
	}
	cancel()
	select {
	case err = <-result:
		if err == nil {
			t.Fatal("missing cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("wait not cancelled")
	}
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("wait connection not closed")
	}
	close(requests)
	for method := range requests {
		if method == "bad params" {
			t.Fatal(method)
		}
	}
}
func TestRenameAgentRejectsReplacedTerminal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s")
	l, err := localipc.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		conn, e := l.Accept()
		if e != nil {
			return
		}
		defer conn.Close()
		bufio.NewReader(conn).ReadBytes('\n')
		json.NewEncoder(conn).Encode(map[string]any{"id": "m0-1", "result": map[string]any{"type": "agent_info", "agent": map[string]any{"pane_id": "pane", "terminal_id": "replacement", "agent": "codex"}}})
	}()
	err = RenameAgentTerminal(context.Background(), Target{path}, "pane", "original", nil, func() bool { return true })
	if err == nil {
		t.Fatal("renamed replacement terminal")
	}
}
func TestMachineArgumentsAndEnvironment(t *testing.T) {
	args, err := MachineArguments(MachineCommand{Action: "open", Value: "me@host", Session: "work"})
	if err != nil || strings.Join(args, "|") != "--remote|me@host|--session|work" {
		t.Fatalf("%v %v", args, err)
	}
	if _, err = MachineArguments(MachineCommand{Action: "add", Value: "--yes"}); err == nil {
		t.Fatal("option accepted as target")
	}
	env := CatalogEnvironment([]string{"HERDR_SOCKET_PATH=x", "HERDR_SESSION=other", "HERDR_PANE_ID=p", "PATH=bin", "HERDR_CONFIG_PATH=config"})
	if strings.Join(env, "|") != "PATH=bin|HERDR_CONFIG_PATH=config" {
		t.Fatal(env)
	}
}

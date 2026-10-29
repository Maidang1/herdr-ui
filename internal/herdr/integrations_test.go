package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"github.com/Maidang1/herdr-ui/internal/localipc"
	"os"
	"path/filepath"
	"testing"
)

func integrationServer(t *testing.T, reply string) (Target, <-chan struct {
	Method string
	Params map[string]string
}) {
	t.Helper()
	// macOS Unix socket paths have a small length limit; t.TempDir includes
	// the full test name and can exceed it before the socket name is added.
	dir, err := os.MkdirTemp("", "herdr-integration-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "i.sock")
	listener, err := localipc.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	requests := make(chan struct {
		Method string
		Params map[string]string
	}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var request struct {
			Method string
			Params map[string]string
		}
		if json.NewDecoder(bufio.NewReader(conn)).Decode(&request) != nil {
			return
		}
		requests <- request
		conn.Write([]byte(reply + "\n"))
	}()
	return Target{Socket: path}, requests
}

func TestListIntegrationsPreservesAvailabilityAndInstallation(t *testing.T) {
	target, requests := integrationServer(t, `{"id":"m0-1","result":{"type":"integration_list","integrations":[{"target":"codex","label":"Codex","command":"codex","available":false,"state":"current"}]}}`)
	entries, err := ListIntegrations(context.Background(), target)
	if err != nil || len(entries) != 1 {
		t.Fatal(entries, err)
	}
	if entries[0].Available || entries[0].State != "current" || entries[0].Target != "codex" {
		t.Fatal(entries)
	}
	if request := <-requests; request.Method != "integration.list" || len(request.Params) != 0 {
		t.Fatal(request)
	}
}

func TestIntegrationMutationUsesServerInstaller(t *testing.T) {
	for _, action := range []string{"install", "uninstall"} {
		t.Run(action, func(t *testing.T) {
			target, requests := integrationServer(t, `{"id":"m0-1","result":{"type":"integration_`+action+`","target":"antigravity_cli","details":{"messages":["Configuration updated"]}}}`)
			messages, err := ChangeIntegration(context.Background(), target, action, "antigravity_cli")
			if err != nil || len(messages) != 1 || messages[0] != "Configuration updated" {
				t.Fatal(messages, err)
			}
			request := <-requests
			if request.Method != "integration."+action || request.Params["target"] != "antigravity_cli" {
				t.Fatal(request)
			}
		})
	}
}

func TestIntegrationFailuresAreNotTreatedAsSuccess(t *testing.T) {
	for _, reply := range []string{
		`{"id":"m0-1","error":{"code":"permission_denied","message":"Cannot update agent config"}}`,
		`{"id":"m0-1","result":{"type":"integration_list"}}`,
		`{"id":"m0-1","result":{"type":"different_response","integrations":[]}}`,
	} {
		target, _ := integrationServer(t, reply)
		if _, err := ListIntegrations(context.Background(), target); err == nil {
			t.Fatal("invalid response accepted", reply)
		}
	}
	target, _ := integrationServer(t, `{"id":"m0-1","result":{"type":"integration_install","target":"claude","details":{"messages":[]}}}`)
	if _, err := ChangeIntegration(context.Background(), target, "install", "codex"); err == nil {
		t.Fatal("wrong mutation target accepted")
	}
}

func TestIntegrationMutationRequiresCompleteResult(t *testing.T) {
	for _, details := range []string{"", `,"details":null`, `,"details":{}`, `,"details":{"messages":null}`} {
		target, _ := integrationServer(t, `{"id":"m0-1","result":{"type":"integration_install","target":"codex"`+details+`}}`)
		if _, err := ChangeIntegration(context.Background(), target, "install", "codex"); err == nil {
			t.Fatal("incomplete mutation response accepted", details)
		}
	}
}

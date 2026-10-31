package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Maidang1/herdr-ui/internal/localipc"
)

// fakeServer answers each request with reply(method, params), recording
// what it was asked.
func fakeServer(t *testing.T, reply func(method string, params json.RawMessage) any) (Target, <-chan string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "s")
	listener, err := localipc.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	methods := make(chan string, 16)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				line, err := bufio.NewReader(conn).ReadBytes('\n')
				if err != nil {
					return
				}
				var request struct {
					ID     string          `json:"id"`
					Method string          `json:"method"`
					Params json.RawMessage `json:"params"`
				}
				if json.Unmarshal(line, &request) != nil {
					return
				}
				methods <- request.Method + " " + string(request.Params)
				json.NewEncoder(conn).Encode(map[string]any{"id": request.ID, "result": reply(request.Method, request.Params)})
			}()
		}
	}()
	return Target{path}, methods
}

func TestAgentPromptAndWait(t *testing.T) {
	target, methods := fakeServer(t, func(method string, _ json.RawMessage) any {
		agent := map[string]any{"pane_id": "p1", "terminal_id": "t1", "agent": "claude", "agent_status": "done"}
		if method == "agent.prompt" {
			return map[string]any{"type": "agent_prompted", "agent": agent}
		}
		return map[string]any{"type": "agent_info", "agent": agent}
	})
	if _, err := AgentPrompt(context.Background(), target, "p1", "  "); err == nil {
		t.Fatal("accepted an empty prompt")
	}
	if _, err := AgentPrompt(context.Background(), target, "p1", "run the tests"); err != nil {
		t.Fatal(err)
	}
	if got := <-methods; !strings.HasPrefix(got, "agent.prompt ") || !strings.Contains(got, `"text":"run the tests"`) {
		t.Fatalf("prompt request %s", got)
	}
	details, err := AgentWait(context.Background(), target, "p1", []string{"done", "blocked"}, time.Minute)
	if err != nil || details.Status != "done" {
		t.Fatalf("%+v %v", details, err)
	}
	if got := <-methods; !strings.Contains(got, `"until":["done","blocked"]`) || !strings.Contains(got, `"timeout_ms":60000`) {
		t.Fatalf("wait request %s", got)
	}
	if _, err := AgentWait(context.Background(), target, "p1", []string{"sleeping"}, time.Minute); err == nil {
		t.Fatal("accepted an unknown state")
	}
}

func TestAgentViewPresetsBecomeHerdrFilters(t *testing.T) {
	cases := map[string]string{
		"attention": `{"field":"status","op":"in","values":["blocked","done"]}`,
		"working":   `{"field":"status","op":"eq","value":"working"}`,
		"workspace": `{"field":"workspace_id","op":"eq","value":{"context":"current_workspace_id"}}`,
	}
	for id, want := range cases {
		data, _ := json.Marshal(AgentViewPresetByID(id).agentViewFilter())
		if string(data) != want {
			t.Fatalf("%s: %s", id, data)
		}
	}
	if AgentViewPresetByID("all").agentViewFilter() != nil || AgentViewPresetByID("unknown").ID != "all" {
		t.Fatal("all is not unfiltered")
	}
	target, methods := fakeServer(t, func(string, json.RawMessage) any { return map[string]any{"type": "agent_view", "active": true} })
	if err := SetAgentView(context.Background(), target, AgentViewPresetByID("working")); err != nil {
		t.Fatal(err)
	}
	if got := <-methods; !strings.HasPrefix(got, "agent.view.set ") || !strings.Contains(got, `"source":"hergo"`) {
		t.Fatalf("view request %s", got)
	}
	if err := SetAgentView(context.Background(), target, AgentViewPresetByID("all")); err != nil {
		t.Fatal(err)
	}
	if got := <-methods; !strings.HasPrefix(got, "agent.view.clear ") {
		t.Fatalf("clear request %s", got)
	}
}

func TestLayoutTemplateDropsIdentityAndEnvironment(t *testing.T) {
	root := `{"type":"split","direction":"right","ratio":0.6,"first":{"type":"pane","pane_id":"p1","cwd":"/src","label":"agent","env":{"TOKEN":"secret"}},"second":{"type":"pane","pane_id":"p2","command":["npm","test"]}}`
	target, methods := fakeServer(t, func(method string, _ json.RawMessage) any {
		if method == "layout.export" {
			return map[string]any{"type": "layout_export", "layout": map[string]any{"root": json.RawMessage(root)}}
		}
		return map[string]any{"type": "layout_applied"}
	})
	template, err := ExportLayoutTemplate(context.Background(), target, "tab1", "Review")
	if err != nil {
		t.Fatal(err)
	}
	<-methods
	text := string(template.Root)
	if strings.Contains(text, "pane_id") || strings.Contains(text, "secret") || !strings.Contains(text, `"cwd":"/src"`) || !strings.Contains(text, `"ratio":0.6`) {
		t.Fatalf("template %s", text)
	}
	if err := ApplyLayoutTemplate(context.Background(), target, "w1", template); err != nil {
		t.Fatal(err)
	}
	if got := <-methods; !strings.HasPrefix(got, "layout.apply ") || !strings.Contains(got, `"tab_label":"Review"`) || !strings.Contains(got, `"workspace_id":"w1"`) {
		t.Fatalf("apply request %s", got)
	}
	if _, err := templateNode(json.RawMessage(`{"type":"tabs"}`)); err == nil {
		t.Fatal("accepted an unknown node")
	}
}

func TestShowNotificationValidates(t *testing.T) {
	target, methods := fakeServer(t, func(string, json.RawMessage) any { return map[string]any{"type": "ok"} })
	if ShowNotification(context.Background(), target, "", "", "") == nil || ShowNotification(context.Background(), target, "x", "", "loud") == nil {
		t.Fatal("accepted an invalid notification")
	}
	if err := ShowNotification(context.Background(), target, "Done", "tests passed", "done"); err != nil {
		t.Fatal(err)
	}
	if got := <-methods; !strings.Contains(got, `"sound":"done"`) || !strings.Contains(got, `"body":"tests passed"`) {
		t.Fatalf("notification %s", got)
	}
}

func TestPluginUnlinkReportsNotLinked(t *testing.T) {
	target, _ := fakeServer(t, func(string, json.RawMessage) any {
		return map[string]any{"type": "plugin_unlinked", "plugin_id": "x", "removed": false}
	})
	if err := UnlinkPlugin(context.Background(), target, "x"); err == nil || !strings.Contains(err.Error(), "not a linked plugin") {
		t.Fatalf("%v", err)
	}
	if _, err := LinkPlugin(context.Background(), target, " \n"); err == nil {
		t.Fatal("accepted an empty path")
	}
}

func TestLocateBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bits are Unix")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "herdr-test-binary")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\necho 'herdr 9.9.9'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	found := Detect(context.Background(), "herdr-test-binary")
	if !found.Installed() || found.Path != exe || found.Version != "herdr 9.9.9" {
		t.Fatalf("%+v", found)
	}
	if missing := Detect(context.Background(), "herdr-missing-binary-name"); missing.Installed() || missing.Path != "" {
		t.Fatalf("%+v", missing)
	}
	broken := filepath.Join(dir, "herdr-broken")
	os.WriteFile(broken, []byte("#!/bin/sh\nexit 3\n"), 0o755)
	if got := Detect(context.Background(), broken); got.Installed() || got.Path != broken || got.Error == "" {
		t.Fatalf("%+v", got)
	}
}

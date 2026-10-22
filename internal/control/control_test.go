package control

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeHandler is a window that only remembers what it was asked to do.
type fakeHandler struct {
	opened   []string
	closed   []string
	notes    []string
	zooms    []string
	commands []string
	found    []string
}

func (f *fakeHandler) ControlStatus() Response {
	return Response{BootID: "boot-1", Workspaces: []WorkspaceInfo{{ID: "w1", Label: "dsh"}}}
}

func (f *fakeHandler) ControlOpenPage(target string) (PageInfo, error) {
	f.opened = append(f.opened, target)
	return PageInfo{ID: "page-1", Title: "example", URL: target, Zoom: 1}, nil
}

func (f *fakeHandler) ControlPages() []PageInfo {
	return []PageInfo{{ID: "page-1", Title: "example", URL: "https://example.com", Zoom: 1}}
}

func (f *fakeHandler) ControlPage(id string) (PageInfo, bool) {
	return PageInfo{ID: id}, true
}

func (f *fakeHandler) ControlClosePage(id string) error {
	f.closed = append(f.closed, id)
	return nil
}

func (f *fakeHandler) ControlReloadPage(id string) error        { return nil }
func (f *fakeHandler) ControlNavigatePage(id, url string) error { return nil }
func (f *fakeHandler) ControlPageBack(id string) error          { return nil }
func (f *fakeHandler) ControlPageForward(id string) error       { return nil }
func (f *fakeHandler) ControlPageText(id string) (string, error) {
	return "the text of the page", nil
}

func (f *fakeHandler) ControlPageZoom(id, change string) (float64, error) {
	f.zooms = append(f.zooms, change)
	return 1.25, nil
}

func (f *fakeHandler) ControlSendNotes(text string) error {
	f.notes = append(f.notes, text)
	return nil
}

func (f *fakeHandler) ControlCommands() []CommandInfo {
	return []CommandInfo{{ID: "cmd-1", Description: "hunk: review changes", Binding: "prefix+shift+h"}}
}

func (f *fakeHandler) ControlInvokeCommand(id string) error {
	f.commands = append(f.commands, id)
	return nil
}

func (f *fakeHandler) ControlAgents() []AgentInfo {
	return []AgentInfo{{PaneID: "w1:p1", Agent: "grok", Status: "idle"}}
}

func (f *fakeHandler) ControlFind(query, direction string) (int, int, error) {
	f.found = append(f.found, query)
	return 3, 0, nil
}

func (f *fakeHandler) ControlFindNext(step int) (int, int, error) { return 3, 1, nil }

func (f *fakeHandler) ControlFindClose() {}

func (f *fakeHandler) ControlFocusWorkspace(id string) error { return nil }
func (f *fakeHandler) ControlFocusTab(id string) error       { return nil }
func (f *fakeHandler) ControlCloseTab(id string) error       { return nil }

func startServer(t *testing.T) (*Server, *fakeHandler, string) {
	t.Helper()
	// A unix socket path is short on macOS, so the test keeps it under
	// the temporary directory rather than in the test's own folder.
	socket := filepath.Join("/tmp", fmt.Sprintf("herdr-ui-test-%d.sock", os.Getpid()))
	_ = os.Remove(socket)
	handler := &fakeHandler{}
	server, err := Listen(socket, handler)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	t.Cleanup(func() { server.Close() })
	return server, handler, socket
}

func TestTheServerAnswersOnItsSocket(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "status"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "status" {
		t.Fatalf("the answer came back as %q", response.Status)
	}
	if response.BootID != "boot-1" {
		t.Fatalf("the boot id came back as %q", response.BootID)
	}
	if response.Protocol != ProtocolVersion {
		t.Fatalf("the protocol came back as %d", response.Protocol)
	}
}

func TestTheServerOpensAPage(t *testing.T) {
	_, handler, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{
		Method: "browser.open",
		Page:   &PageTarget{URL: "https://example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "opened" {
		t.Fatalf("the answer came back as %q", response.Status)
	}
	if response.Page == nil || response.Page.ID != "page-1" {
		t.Fatalf("the page came back as %+v", response.Page)
	}
	if len(handler.opened) != 1 || handler.opened[0] != "https://example.com" {
		t.Fatalf("the handler saw %+v", handler.opened)
	}
}

func TestTheServerReadsTheTextOfAPage(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{
		Method: "browser.text",
		Page:   &PageTarget{ID: "page-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Text != "the text of the page" {
		t.Fatalf("the text came back as %q", response.Text)
	}
}

func TestTheServerSendsNotes(t *testing.T) {
	_, handler, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "notes.send", Text: "look here"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "notes_sent" {
		t.Fatalf("the answer came back as %q", response.Status)
	}
	if len(handler.notes) != 1 || handler.notes[0] != "look here" {
		t.Fatalf("the handler saw %+v", handler.notes)
	}
}

func TestTheServerRefusesWhatItDoesNotServe(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Call(ctx, Request{Method: "nonsense"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "error" || response.Code != "unsupported_method" {
		t.Fatalf("the answer came back as %+v", response)
	}
}

func TestTheServerRefusesARequestWithoutAMethod(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Call(ctx, Request{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "error" {
		t.Fatalf("the answer came back as %+v", response)
	}
}

func TestTheServerListsTheCommands(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "command", Op: "list"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Commands) != 1 || response.Commands[0].ID != "cmd-1" {
		t.Fatalf("the commands came back as %+v", response.Commands)
	}
}

func TestTheServerRunsACommand(t *testing.T) {
	_, handler, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "command", Op: "invoke", Target: "cmd-1"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "ok" {
		t.Fatalf("the answer came back as %q", response.Status)
	}
	if len(handler.commands) != 1 || handler.commands[0] != "cmd-1" {
		t.Fatalf("the handler saw %+v", handler.commands)
	}
}

func TestTheServerListsTheAgents(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Agents) != 1 || response.Agents[0].Agent != "grok" {
		t.Fatalf("the agents came back as %+v", response.Agents)
	}
}

func TestTheServerSearchesTheOutputOfAPane(t *testing.T) {
	_, handler, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.CallAndCheck(ctx, Request{Method: "find", Op: "search", Text: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != "found" {
		t.Fatalf("the answer came back as %q", response.Status)
	}
	if len(handler.found) != 1 || handler.found[0] != "error" {
		t.Fatalf("the handler saw %+v", handler.found)
	}
}

func TestTheSocketIsForTheUserAlone(t *testing.T) {
	_, _, socket := startServer(t)
	info, err := os.Stat(socket)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("the socket has the mode %o, want 600", mode)
	}
}

func TestTheAnswerIsOneLineOfJSON(t *testing.T) {
	_, _, socket := startServer(t)
	client := NewClient(socket)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := client.Call(ctx, Request{Method: "status"})
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "\n") {
		t.Fatal("the answer holds a line break")
	}
}

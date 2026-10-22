// Package control is the door into a running Herdr UI.
//
// The window listens on a unix socket and answers one JSON request per
// connection. The request names a method and, for the methods that act on
// something, says what to act on. The answer is one JSON object whose
// status says whether the request was carried out.
package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the version of the control protocol this server speaks.
const ProtocolVersion = 6

// MaxMessageSize bounds one request, so a broken client cannot make the
// server read without end.
const MaxMessageSize = 256 * 1024

// Caller says who asks and on what.
type Caller struct {
	PaneID      string `json:"pane_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Daemon      string `json:"daemon_socket,omitempty"`
	Window      int    `json:"window,omitempty"`
}

// PageTarget says what page to act on.
type PageTarget struct {
	ID   string `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	File string `json:"file,omitempty"`
}

// Request is one request for the window.
type Request struct {
	Method string      `json:"method"`
	Caller *Caller     `json:"caller,omitempty"`
	Op     string      `json:"op,omitempty"`
	Page   *PageTarget `json:"page,omitempty"`
	Target string      `json:"target,omitempty"`
	URL    string      `json:"url,omitempty"`
	Change string      `json:"change,omitempty"`
	Text   string      `json:"text,omitempty"`
	Panes  string      `json:"panes,omitempty"`
	Wait   int         `json:"wait_seconds,omitempty"`
}

// Response is the answer to one request.
type Response struct {
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`

	Version      string            `json:"version,omitempty"`
	Protocol     int               `json:"protocol,omitempty"`
	PID          int               `json:"pid,omitempty"`
	BootID       string            `json:"boot_id,omitempty"`
	Workspaces   []WorkspaceInfo   `json:"workspaces,omitempty"`
	Tabs         []TabInfo         `json:"tabs,omitempty"`
	Panes        []PaneInfo        `json:"panes,omitempty"`
	Pages        []PageInfo        `json:"pages,omitempty"`
	Commands     []CommandInfo     `json:"commands,omitempty"`
	Worktrees    []WorktreeInfo    `json:"worktrees,omitempty"`
	Integrations []IntegrationInfo `json:"integrations,omitempty"`
	Agents       []AgentInfo       `json:"agents,omitempty"`
	Page         *PageInfo         `json:"page,omitempty"`
	Zoom         float64           `json:"zoom,omitempty"`
	Text         string            `json:"text,omitempty"`
	Notes        []Note            `json:"notes,omitempty"`
}

// WorkspaceInfo describes one workspace.
type WorkspaceInfo struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	TabCount    int    `json:"tab_count"`
	AgentStatus string `json:"agent_status,omitempty"`
	Focused     bool   `json:"focused"`
}

// TabInfo describes one tab.
type TabInfo struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	PaneCount   int    `json:"pane_count"`
	AgentStatus string `json:"agent_status,omitempty"`
	Focused     bool   `json:"focused"`
}

// PaneInfo describes one pane.
type PaneInfo struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	TabID       string `json:"tab_id"`
	CWD         string `json:"cwd,omitempty"`
	Title       string `json:"title,omitempty"`
	Agent       string `json:"agent,omitempty"`
	AgentStatus string `json:"agent_status,omitempty"`
	Focused     bool   `json:"focused"`
}

// PageInfo describes one browser page.
type PageInfo struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	URL   string  `json:"url"`
	File  string  `json:"file,omitempty"`
	Zoom  float64 `json:"zoom"`
}

// CommandInfo describes one command the daemon offers.
type CommandInfo struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
	Binding     string `json:"binding,omitempty"`
}

// AgentInfo describes one agent that works in a pane.
type AgentInfo struct {
	PaneID      string `json:"pane_id"`
	WorkspaceID string `json:"workspace_id"`
	TabID       string `json:"tab_id"`
	Agent       string `json:"agent,omitempty"`
	Title       string `json:"title,omitempty"`
	Status      string `json:"agent_status,omitempty"`
	Focused     bool   `json:"focused"`
}

// WorktreeInfo describes one worktree of the repository of the session.
type WorktreeInfo struct {
	Path            string `json:"path"`
	Label           string `json:"label"`
	Branch          string `json:"branch,omitempty"`
	OpenWorkspaceID string `json:"open_workspace_id,omitempty"`
}

// IntegrationInfo describes one coding agent the daemon knows.
type IntegrationInfo struct {
	Target    string `json:"target"`
	Label     string `json:"label"`
	Command   string `json:"command,omitempty"`
	State     string `json:"state,omitempty"`
	Available bool   `json:"available"`
}

// Note is a note the user left for an agent.
type Note struct {
	Text   string `json:"text"`
	PaneID string `json:"pane_id,omitempty"`
}

// Handler is what the window offers the control server.
type Handler interface {
	// ControlStatus says what the window is showing.
	ControlStatus() Response
	// ControlOpenPage opens a page for a URL or a local file.
	ControlOpenPage(target string) (PageInfo, error)
	// ControlPages lists the pages.
	ControlPages() []PageInfo
	// ControlPage returns one page.
	ControlPage(id string) (PageInfo, bool)
	// ControlClosePage closes a page.
	ControlClosePage(id string) error
	// ControlReloadPage reloads a page.
	ControlReloadPage(id string) error
	// ControlNavigatePage loads another address in a page.
	ControlNavigatePage(id, url string) error
	// ControlPageBack goes back in a page.
	ControlPageBack(id string) error
	// ControlPageForward goes forward in a page.
	ControlPageForward(id string) error
	// ControlPageText reads the text of a page.
	ControlPageText(id string) (string, error)
	// ControlPageZoom changes the zoom of a page.
	ControlPageZoom(id, change string) (float64, error)
	// ControlSendNotes sends notes to the focused pane.
	ControlSendNotes(text string) error
	// ControlFocusWorkspace focuses a workspace.
	ControlFocusWorkspace(id string) error
	// ControlFocusTab focuses a tab.
	ControlFocusTab(id string) error
	// ControlCloseTab closes a tab.
	ControlCloseTab(id string) error
	// ControlCommands lists the commands the daemon offers.
	ControlCommands() []CommandInfo
	// ControlInvokeCommand carries out a command.
	ControlInvokeCommand(id string) error
	// ControlAgents lists the agents of the session.
	ControlAgents() []AgentInfo
	// ControlFind searches the output of the focused pane.
	ControlFind(query, direction string) (int, int, error)
	// ControlFindNext moves to the next match, or to the one before it.
	ControlFindNext(step int) (int, int, error)
	// ControlFindClose closes the find bar.
	ControlFindClose()
}

// Server answers control requests on a unix socket.
type Server struct {
	socket   string
	handler  Handler
	listener net.Listener

	mu     sync.Mutex
	closed bool
}

// Listen starts a control server on the socket.
//
// The socket is created with the permissions only the user may reach, and a
// socket that is already there is replaced.
func Listen(socket string, handler Handler) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(socket), 0o700); err != nil {
		return nil, err
	}
	if err := os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		listener.Close()
		return nil, err
	}
	return &Server{socket: socket, handler: handler, listener: listener}, nil
}

// Socket returns the path the server listens on.
func (s *Server) Socket() string { return s.socket }

// Serve answers requests until the server is closed.
func (s *Server) Serve() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return
			}
			continue
		}
		go s.handle(conn)
	}
}

// Close stops the server and removes its socket.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	err := s.listener.Close()
	_ = os.Remove(s.socket)
	return err
}

// handle answers one request.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return
	}
	if len(line) > MaxMessageSize {
		writeResponse(conn, Response{Status: "error", Code: "invalid_request", Message: "the request is too large"})
		return
	}

	var request Request
	if err := json.Unmarshal(line, &request); err != nil {
		writeResponse(conn, Response{Status: "error", Code: "invalid_request", Message: err.Error()})
		return
	}

	response := s.dispatch(request)
	writeResponse(conn, response)
}

// dispatch carries out one request.
func (s *Server) dispatch(request Request) Response {
	if request.Method == "" {
		return Response{Status: "error", Code: "invalid_request", Message: "the request names no method"}
	}
	switch request.Method {
	case "status":
		status := s.handler.ControlStatus()
		status.Status = "status"
		status.Protocol = ProtocolVersion
		return status
	case "browser.open":
		return s.openPage(request)
	case "browser.list":
		return Response{Status: "status", Pages: s.handler.ControlPages()}
	case "browser.reload":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		if err := s.handler.ControlReloadPage(id); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "reloaded"}
	case "browser.close":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		if err := s.handler.ControlClosePage(id); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	case "browser.navigate":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		url := request.URL
		if url == "" && request.Page != nil {
			url = request.Page.URL
		}
		if url == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no address"}
		}
		if err := s.handler.ControlNavigatePage(id, url); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	case "browser.back":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		if err := s.handler.ControlPageBack(id); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	case "browser.forward":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		if err := s.handler.ControlPageForward(id); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	case "browser.text":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		text, err := s.handler.ControlPageText(id)
		if err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok", Text: text}
	case "browser.zoom":
		id, err := s.pageID(request)
		if err != nil {
			return errorResponse(err)
		}
		change := request.Change
		if change == "" {
			change = request.Target
		}
		zoom, err := s.handler.ControlPageZoom(id, change)
		if err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok", Zoom: zoom * 100}
	case "browser.feedback":
		text := request.Text
		if text == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request carries no notes"}
		}
		if err := s.handler.ControlSendNotes(text); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "feedback", Notes: []Note{{Text: text}}}
	case "notes.send":
		text := request.Text
		if text == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request carries no notes"}
		}
		if err := s.handler.ControlSendNotes(text); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "notes_sent", Notes: []Note{{Text: text}}}
	case "tab":
		return s.tabOp(request)
	case "window":
		return s.windowOp(request)
	case "command":
		return s.commandOp(request)
	case "agent":
		return Response{Status: "agent", Agents: s.handler.ControlAgents()}
	case "find":
		return s.findOp(request)
	case "app":
		status := s.handler.ControlStatus()
		status.Status = "app"
		status.Protocol = ProtocolVersion
		return status
	default:
		return Response{Status: "error", Code: "unsupported_method", Message: "this window does not serve " + request.Method}
	}
}

// openPage opens a page for a URL or a local file.
func (s *Server) openPage(request Request) Response {
	target := request.URL
	if target == "" && request.Page != nil {
		target = request.Page.URL
		if target == "" {
			target = request.Page.File
		}
	}
	if target == "" {
		target = request.Target
	}
	if target == "" {
		return Response{Status: "error", Code: "invalid_request", Message: "the request names no page"}
	}
	page, err := s.handler.ControlOpenPage(target)
	if err != nil {
		return errorResponse(err)
	}
	return Response{Status: "opened", Page: &page}
}

// pageID says which page a request is about.
func (s *Server) pageID(request Request) (string, error) {
	if request.Page != nil && request.Page.ID != "" {
		return request.Page.ID, nil
	}
	if request.Target != "" {
		return request.Target, nil
	}
	if request.Caller != nil && request.Caller.PaneID != "" {
		return request.Caller.PaneID, nil
	}
	return "", errors.New("the request names no page")
}

// tabOp carries out a request about the tabs.
func (s *Server) tabOp(request Request) Response {
	status := s.handler.ControlStatus()
	switch request.Op {
	case "", "list":
		return Response{Status: "tab", Tabs: status.Tabs, Workspaces: status.Workspaces}
	case "focus":
		if request.Target == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no tab"}
		}
		if err := s.handler.ControlFocusTab(request.Target); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	case "close":
		if request.Target == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no tab"}
		}
		if err := s.handler.ControlCloseTab(request.Target); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	default:
		return Response{Status: "error", Code: "unsupported_op", Message: "this window does not serve tab " + request.Op}
	}
}

// windowOp carries out a request about the windows.
func (s *Server) windowOp(request Request) Response {
	status := s.handler.ControlStatus()
	switch request.Op {
	case "", "list":
		return Response{Status: "window", Workspaces: status.Workspaces, Tabs: status.Tabs, Pages: status.Pages}
	case "focus":
		if request.Target == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no workspace"}
		}
		if err := s.handler.ControlFocusWorkspace(request.Target); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	default:
		return Response{Status: "error", Code: "unsupported_op", Message: "this window does not serve window " + request.Op}
	}
}

// commandOp carries out a request about the commands.
func (s *Server) commandOp(request Request) Response {
	switch request.Op {
	case "", "list":
		return Response{Status: "command", Commands: s.handler.ControlCommands()}
	case "invoke":
		if request.Target == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no command"}
		}
		if err := s.handler.ControlInvokeCommand(request.Target); err != nil {
			return errorResponse(err)
		}
		return Response{Status: "ok"}
	default:
		return Response{Status: "error", Code: "unsupported_op", Message: "this window does not serve command " + request.Op}
	}
}

// findOp carries out a request about the find bar.
func (s *Server) findOp(request Request) Response {
	switch request.Op {
	case "", "search":
		query := request.Text
		if query == "" {
			query = request.Target
		}
		if query == "" {
			return Response{Status: "error", Code: "invalid_request", Message: "the request names no text to find"}
		}
		total, current, err := s.handler.ControlFind(query, "forward")
		if err != nil {
			return errorResponse(err)
		}
		return Response{Status: "found", Zoom: float64(current), Text: query, Notes: []Note{{Text: fmt.Sprintf("%d of %d", current+1, total)}}}
	case "next", "previous":
		step := 1
		if request.Op == "previous" {
			step = -1
		}
		total, current, err := s.handler.ControlFindNext(step)
		if err != nil {
			return errorResponse(err)
		}
		return Response{Status: "found", Zoom: float64(current), Notes: []Note{{Text: fmt.Sprintf("%d of %d", current+1, total)}}}
	case "close":
		s.handler.ControlFindClose()
		return Response{Status: "ok"}
	default:
		return Response{Status: "error", Code: "unsupported_op", Message: "this window does not serve find " + request.Op}
	}
}

func errorResponse(err error) Response {
	return Response{Status: "error", Code: "failed", Message: err.Error()}
}

func writeResponse(conn net.Conn, response Response) {
	body, err := json.Marshal(response)
	if err != nil {
		return
	}
	_, _ = conn.Write(append(body, '\n'))
}

// Client asks a running window to do something.
type Client struct {
	socket string
}

// NewClient returns a client for the window listening on the socket.
func NewClient(socket string) *Client { return &Client{socket: socket} }

// Call sends one request and returns the answer.
func (c *Client) Call(ctx context.Context, request Request) (Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return Response{}, err
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "unix", c.socket)
	if err != nil {
		return Response{}, fmt.Errorf("herdr-ui: no window is listening on %s", c.socket)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	if _, err := conn.Write(append(body, '\n')); err != nil {
		return Response{}, err
	}
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil && len(line) == 0 {
		return Response{}, err
	}
	var response Response
	if err := json.Unmarshal(line, &response); err != nil {
		return Response{}, err
	}
	return response, nil
}

// CallAndCheck sends one request and turns an error answer into an error.
func (c *Client) CallAndCheck(ctx context.Context, request Request) (Response, error) {
	response, err := c.Call(ctx, request)
	if err != nil {
		return response, err
	}
	if response.Status == "error" {
		return response, fmt.Errorf("herdr-ui: %s: %s", response.Code, response.Message)
	}
	return response, nil
}

// DefaultSocketPath returns where the control server listens by default.
func DefaultSocketPath() (string, error) {
	if path := os.Getenv("HERDR_UI_CONTROL_SOCKET"); path != "" {
		return path, nil
	}
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "herdr", "herdr-ui", "control.sock"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "herdr", "herdr-ui", "control.sock"), nil
}

// Describe renders an answer for the terminal.
func Describe(response Response) string {
	var out strings.Builder
	out.WriteString(response.Status)
	if response.Message != "" {
		out.WriteString(": ")
		out.WriteString(response.Message)
	}
	if len(response.Pages) > 0 {
		out.WriteString("\n")
		for _, page := range response.Pages {
			out.WriteString(fmt.Sprintf("  %s  %s  %s\n", page.ID, page.Title, page.URL))
		}
	}
	if len(response.Tabs) > 0 {
		out.WriteString("\n")
		for _, tab := range response.Tabs {
			out.WriteString(fmt.Sprintf("  %s  %s\n", tab.ID, tab.Label))
		}
	}
	if len(response.Workspaces) > 0 {
		out.WriteString("\n")
		for _, workspace := range response.Workspaces {
			out.WriteString(fmt.Sprintf("  %s  %s\n", workspace.ID, workspace.Label))
		}
	}
	if len(response.Commands) > 0 {
		out.WriteString("\n")
		for _, command := range response.Commands {
			out.WriteString(fmt.Sprintf("  %s  %s  %s\n", command.ID, command.Description, command.Binding))
		}
	}
	if len(response.Worktrees) > 0 {
		out.WriteString("\n")
		for _, worktree := range response.Worktrees {
			out.WriteString(fmt.Sprintf("  %s  %s  %s\n", worktree.Path, worktree.Label, worktree.Branch))
		}
	}
	if len(response.Integrations) > 0 {
		out.WriteString("\n")
		for _, integration := range response.Integrations {
			out.WriteString(fmt.Sprintf("  %s  %s  %s\n", integration.Target, integration.Label, integration.State))
		}
	}
	if len(response.Agents) > 0 {
		out.WriteString("\n")
		for _, agent := range response.Agents {
			out.WriteString(fmt.Sprintf("  %s  %s  %s\n", agent.PaneID, agent.Agent, agent.Status))
		}
	}
	if response.Message != "" && response.Status != "error" {
		out.WriteString(": ")
		out.WriteString(response.Message)
	}
	if response.Text != "" {
		out.WriteString("\n")
		out.WriteString(response.Text)
	}
	return out.String()
}

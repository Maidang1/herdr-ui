package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Maidang1/herdr-ui/internal/localipc"
)

// callLong owns an independent connection for a request the server answers
// only when something happens, as agent.wait does. The ordinary five second
// request budget would end it early; the caller bounds it with timeout.
func callLong(ctx context.Context, target Target, method string, params any, timeout time.Duration, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
	defer cancel()
	conn, err := localipc.DialContext(ctx, target.Socket)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	id := fmt.Sprintf("long-%d", connectionIDs.Add(1))
	if err := sendRequest(conn, id, method, params); err != nil {
		return err
	}
	line, err := readLine(bufio.NewReader(conn))
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return decodeResponse(line, id, out)
}

// AgentStatuses are the lifecycle states agent.wait and agent.prompt wait for.
var AgentStatuses = []string{"idle", "working", "blocked", "done"}

// AgentPrompt sends text to the agent in a pane as a submitted prompt.
// Herdr types it and presses Enter; nothing is retried.
func AgentPrompt(ctx context.Context, target Target, pane, text string) (AgentDetails, error) {
	var response struct {
		Type  string       `json:"type"`
		Agent AgentDetails `json:"agent"`
	}
	if strings.TrimSpace(text) == "" {
		return response.Agent, fmt.Errorf("enter a prompt")
	}
	if err := Call(ctx, target, "agent.prompt", struct {
		Target string `json:"target"`
		Text   string `json:"text"`
	}{pane, text}, &response); err != nil {
		return response.Agent, err
	}
	if response.Type != "agent_prompted" || response.Agent.PaneID != pane {
		return response.Agent, fmt.Errorf("unexpected prompt response; check the agent before repeating")
	}
	return response.Agent, nil
}

// AgentWait returns once the agent in pane reaches one of until, or the
// timeout ends it.
func AgentWait(ctx context.Context, target Target, pane string, until []string, timeout time.Duration) (AgentDetails, error) {
	var response struct {
		Type  string       `json:"type"`
		Agent AgentDetails `json:"agent"`
	}
	if len(until) == 0 || timeout <= 0 || timeout > 24*time.Hour {
		return response.Agent, fmt.Errorf("choose at least one state and a timeout up to 24 hours")
	}
	for _, state := range until {
		if !validAgentStatus(state) {
			return response.Agent, fmt.Errorf("unknown agent state %q", state)
		}
	}
	err := callLong(ctx, target, "agent.wait", struct {
		Target  string   `json:"target"`
		Until   []string `json:"until"`
		Timeout int64    `json:"timeout_ms"`
	}{pane, until, timeout.Milliseconds()}, timeout, &response)
	if err != nil {
		return response.Agent, err
	}
	if response.Agent.PaneID != "" && response.Agent.PaneID != pane {
		return response.Agent, fmt.Errorf("unexpected agent identity")
	}
	return response.Agent, nil
}

func validAgentStatus(state string) bool {
	for _, known := range AgentStatuses {
		if known == state {
			return true
		}
	}
	return false
}

// AgentStart launches an agent of kind in an existing pane, which Herdr
// types into the pane's shell.
func AgentStart(ctx context.Context, target Target, pane, kind, name string, args []string) (AgentDetails, error) {
	var response struct {
		Type  string       `json:"type"`
		Agent AgentDetails `json:"agent"`
	}
	kind = strings.TrimSpace(kind)
	if kind == "" || strings.ContainsAny(kind, " \t\r\n\x00") {
		return response.Agent, fmt.Errorf("enter an agent kind such as claude or codex")
	}
	if strings.TrimSpace(name) == "" {
		name = kind
	}
	params := struct {
		Name    string   `json:"name"`
		Kind    string   `json:"kind"`
		Pane    string   `json:"pane_id"`
		Args    []string `json:"args,omitempty"`
		Timeout int64    `json:"timeout_ms"`
	}{name, kind, pane, args, 30000}
	if err := callLong(ctx, target, "agent.start", params, 30*time.Second, &response); err != nil {
		return response.Agent, err
	}
	if response.Type != "agent_started" {
		return response.Agent, fmt.Errorf("unexpected start response; check the pane before repeating")
	}
	return response.Agent, nil
}

// AgentViewPreset is a named agent filter Hergo applies to its own sidebar
// and shares with Herdr's clients through agent.view.set.
type AgentViewPreset struct {
	ID, Label string
	// Statuses keeps agents in these states; empty keeps every state.
	Statuses []string
	// CurrentWorkspace keeps the agents of the selected workspace.
	CurrentWorkspace bool
}

var AgentViewPresets = []AgentViewPreset{
	{ID: "all", Label: "All agents"},
	{ID: "attention", Label: "Needs attention", Statuses: []string{"blocked", "done"}},
	{ID: "working", Label: "Working", Statuses: []string{"working"}},
	{ID: "workspace", Label: "This workspace", CurrentWorkspace: true},
}

func AgentViewPresetByID(id string) AgentViewPreset {
	for _, preset := range AgentViewPresets {
		if preset.ID == id {
			return preset
		}
	}
	return AgentViewPresets[0]
}

// agentViewFilter is the preset as Herdr's AgentViewFilter, or nil for all.
func (p AgentViewPreset) agentViewFilter() any {
	var filters []any
	if len(p.Statuses) == 1 {
		filters = append(filters, map[string]any{"op": "eq", "field": "status", "value": p.Statuses[0]})
	} else if len(p.Statuses) > 1 {
		values := make([]any, len(p.Statuses))
		for i, s := range p.Statuses {
			values[i] = s
		}
		filters = append(filters, map[string]any{"op": "in", "field": "status", "values": values})
	}
	if p.CurrentWorkspace {
		filters = append(filters, map[string]any{"op": "eq", "field": "workspace_id", "value": map[string]string{"context": "current_workspace_id"}})
	}
	switch len(filters) {
	case 0:
		return nil
	case 1:
		return filters[0]
	}
	return map[string]any{"op": "all", "filters": filters}
}

// SetAgentView shares a preset with Herdr; "all" clears the view.
func SetAgentView(ctx context.Context, target Target, preset AgentViewPreset) error {
	var response struct {
		Type string `json:"type"`
	}
	if preset.ID == "all" {
		return Call(ctx, target, "agent.view.clear", struct {
			Source string `json:"source"`
		}{"hergo"}, &response)
	}
	params := map[string]any{"source": "hergo", "label": preset.Label, "sort": []map[string]string{{"field": "attention", "order": "desc"}, {"field": "workspace_order"}}}
	if filter := preset.agentViewFilter(); filter != nil {
		params["filter"] = filter
	}
	if err := Call(ctx, target, "agent.view.set", params, &response); err != nil {
		return err
	}
	if response.Type != "agent_view" && response.Type != "ok" {
		return fmt.Errorf("unexpected agent view response")
	}
	return nil
}

// MoveWorkspace places a workspace at insert_index of the sidebar order.
func MoveWorkspace(ctx context.Context, target Target, workspace string, index int) error {
	if index < 0 {
		return fmt.Errorf("invalid workspace position")
	}
	return Call(ctx, target, "workspace.move", struct {
		Workspace string `json:"workspace_id"`
		Index     int    `json:"insert_index"`
	}{workspace, index}, nil)
}

// NotificationSound is how loud Herdr's toast is: none, done or request.
func ShowNotification(ctx context.Context, target Target, title, body, sound string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("a notification needs a title")
	}
	switch sound {
	case "", "none", "done", "request":
	default:
		return fmt.Errorf("unknown notification sound %q", sound)
	}
	if sound == "" {
		sound = "none"
	}
	params := map[string]any{"title": title, "sound": sound}
	if body != "" {
		params["body"] = body
	}
	return Call(ctx, target, "notification.show", params, nil)
}

// AgentManifest is how Herdr recognizes one agent, and where that came from.
type AgentManifest struct {
	Agent         string  `json:"agent"`
	Source        string  `json:"source"`
	SourceKind    string  `json:"source_kind"`
	ActiveVersion *string `json:"active_version"`
	RemoteVersion *string `json:"cached_remote_version"`
	Shadowing     bool    `json:"local_override_shadowing_remote"`
	Warning       *string `json:"warning"`
	UpdateError   *string `json:"remote_update_error"`
}

func ReadAgentManifests(ctx context.Context, target Target) ([]AgentManifest, error) {
	return agentManifests(ctx, target, "server.agent_manifests")
}

func ReloadAgentManifests(ctx context.Context, target Target) ([]AgentManifest, error) {
	return agentManifests(ctx, target, "server.reload_agent_manifests")
}

func agentManifests(ctx context.Context, target Target, method string) ([]AgentManifest, error) {
	var raw json.RawMessage
	if err := Call(ctx, target, method, nil, &raw); err != nil {
		return nil, err
	}
	var response struct {
		Type      string          `json:"type"`
		Manifests []AgentManifest `json:"manifests"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("Herdr returned invalid agent manifests")
	}
	return response.Manifests, nil
}

// LinkPlugin links a local plugin directory into Herdr, for plugin
// development; UnlinkPlugin removes the link.
func LinkPlugin(ctx context.Context, target Target, path string) (Plugin, error) {
	var response struct {
		Type   string `json:"type"`
		Plugin Plugin `json:"plugin"`
	}
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsAny(path, "\x00\r\n") {
		return response.Plugin, fmt.Errorf("enter the plugin's directory")
	}
	if err := Call(ctx, target, "plugin.link", struct {
		Path    string `json:"path"`
		Enabled bool   `json:"enabled"`
	}{path, true}, &response); err != nil {
		return response.Plugin, err
	}
	if response.Type != "plugin_linked" {
		return response.Plugin, fmt.Errorf("unexpected plugin link response")
	}
	return response.Plugin, nil
}

func UnlinkPlugin(ctx context.Context, target Target, id string) error {
	var response struct {
		Type    string `json:"type"`
		Removed bool   `json:"removed"`
	}
	if err := Call(ctx, target, "plugin.unlink", struct {
		ID string `json:"plugin_id"`
	}{id}, &response); err != nil {
		return err
	}
	if response.Type != "plugin_unlinked" {
		return fmt.Errorf("unexpected plugin unlink response")
	}
	if !response.Removed {
		return fmt.Errorf("%s is not a linked plugin", id)
	}
	return nil
}

// LiveHandoff asks the running server to hand its sessions to the Herdr
// executable at exe, as after an upgrade, without closing terminals.
func LiveHandoff(ctx context.Context, target Target, exe string) error {
	params := map[string]any{}
	if exe != "" {
		params["import_exe"] = exe
	}
	return callLong(ctx, target, "server.live_handoff", params, 30*time.Second, nil)
}

// LayoutTemplate is a tab's split tree as layout.export describes it,
// without pane identities, for layout.apply to rebuild in a new tab.
type LayoutTemplate struct {
	Name string          `json:"name"`
	Root json.RawMessage `json:"root"`
}

// ExportLayoutTemplate reads a tab's layout and keeps its splits, ratios,
// labels and directories. Pane IDs and environments stay behind: the first
// would name panes that are not the new tab's, the second may hold secrets.
func ExportLayoutTemplate(ctx context.Context, target Target, tab, name string) (LayoutTemplate, error) {
	var response struct {
		Type   string `json:"type"`
		Layout struct {
			Root json.RawMessage `json:"root"`
		} `json:"layout"`
	}
	if err := Call(ctx, target, "layout.export", struct {
		Tab string `json:"tab_id"`
	}{tab}, &response); err != nil {
		return LayoutTemplate{}, err
	}
	if response.Type != "layout_export" || len(response.Layout.Root) == 0 {
		return LayoutTemplate{}, fmt.Errorf("Herdr returned an invalid layout")
	}
	root, err := templateNode(response.Layout.Root)
	if err != nil {
		return LayoutTemplate{}, err
	}
	return LayoutTemplate{Name: name, Root: root}, nil
}

func templateNode(raw json.RawMessage) (json.RawMessage, error) {
	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("invalid layout node")
	}
	var kind string
	json.Unmarshal(node["type"], &kind)
	switch kind {
	case "pane":
		delete(node, "pane_id")
		delete(node, "env")
	case "split":
		for _, side := range []string{"first", "second"} {
			child, err := templateNode(node[side])
			if err != nil {
				return nil, err
			}
			node[side] = child
		}
	default:
		return nil, fmt.Errorf("unknown layout node %q", kind)
	}
	return json.Marshal(node)
}

// ApplyLayoutTemplate builds the template in a new tab of workspace.
func ApplyLayoutTemplate(ctx context.Context, target Target, workspace string, template LayoutTemplate) error {
	if len(template.Root) == 0 {
		return fmt.Errorf("the layout is empty")
	}
	params := map[string]any{"root": template.Root, "focus": true}
	if workspace != "" {
		params["workspace_id"] = workspace
	}
	if template.Name != "" {
		params["tab_label"] = template.Name
	}
	return Call(ctx, target, "layout.apply", params, nil)
}

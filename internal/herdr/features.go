package herdr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Maidang1/herdr-ui/internal/localipc"
)

// AgentDetails follows protocol 22's AgentInfo, independently of pane labels.
type AgentDetails struct {
	PaneID           string        `json:"pane_id"`
	TerminalID       string        `json:"terminal_id"`
	Name             string        `json:"name"`
	Agent            string        `json:"agent"`
	Status           string        `json:"agent_status"`
	CWD              string        `json:"cwd"`
	ForegroundCWD    string        `json:"foreground_cwd"`
	Title            string        `json:"terminal_title_stripped"`
	InteractiveReady bool          `json:"interactive_ready"`
	LaunchPending    bool          `json:"launch_pending"`
	DetectionSkipped bool          `json:"screen_detection_skipped"`
	Session          *AgentSession `json:"agent_session"`
}
type AgentSession struct{ Agent, Kind, Source, Value string }
type ProcessDetails struct {
	PaneID          string              `json:"pane_id"`
	ShellPID        *uint32             `json:"shell_pid"`
	TTY             string              `json:"tty"`
	ForegroundGroup *uint32             `json:"foreground_process_group_id"`
	Processes       []ForegroundProcess `json:"foreground_processes"`
}
type ForegroundProcess struct {
	PID     uint32   `json:"pid"`
	Name    string   `json:"name"`
	Command string   `json:"cmdline"`
	CWD     string   `json:"cwd"`
	Argv    []string `json:"argv"`
}
type PaneDetails struct {
	Agent                    AgentDetails
	Process                  ProcessDetails
	Explain                  json.RawMessage
	AgentError, ExplainError string
}

func ReadPaneDetails(ctx context.Context, target Target, pane string) (PaneDetails, error) {
	var d PaneDetails
	var process struct {
		Type    string         `json:"type"`
		Process ProcessDetails `json:"process_info"`
	}
	if err := Call(ctx, target, "pane.process_info", struct {
		Pane string `json:"pane_id"`
	}{pane}, &process); err != nil {
		return d, err
	}
	if process.Type != "pane_process_info" || process.Process.PaneID != pane {
		return d, fmt.Errorf("unexpected process identity")
	}
	d.Process = process.Process
	var agent struct {
		Type  string       `json:"type"`
		Agent AgentDetails `json:"agent"`
	}
	if err := Call(ctx, target, "agent.get", struct {
		Target string `json:"target"`
	}{pane}, &agent); err != nil {
		d.AgentError = err.Error()
	} else {
		if agent.Type != "agent_info" || agent.Agent.PaneID != pane {
			return d, fmt.Errorf("unexpected agent identity")
		}
		d.Agent = agent.Agent
	}
	var explain struct {
		Type    string          `json:"type"`
		Explain json.RawMessage `json:"explain"`
	}
	if err := Call(ctx, target, "agent.explain", struct {
		Target string `json:"target"`
	}{pane}, &explain); err != nil {
		d.ExplainError = err.Error()
	} else {
		if explain.Type != "agent_explain" {
			d.ExplainError = "Unexpected diagnostic response"
		} else {
			d.Explain = explain.Explain
		}
	}
	return d, nil
}
func renameAgent(ctx context.Context, target Target, pane string, name *string) error {
	var response struct {
		Type string `json:"type"`
	}
	err := Call(ctx, target, "agent.rename", struct {
		Target string  `json:"target"`
		Name   *string `json:"name"`
	}{pane, name}, &response)
	if err == nil && response.Type != "ok" && response.Type != "agent_info" {
		return fmt.Errorf("unexpected rename response; check the agent name before repeating")
	}
	return err
}

// RenameAgentTerminal verifies live agent identity immediately before a single
// mutation. The API has no conditional write, so Herdr remains authoritative.
func RenameAgentTerminal(ctx context.Context, target Target, pane, terminal string, name *string, current func() bool) error {
	var response struct {
		Type  string       `json:"type"`
		Agent AgentDetails `json:"agent"`
	}
	if err := Call(ctx, target, "agent.get", struct {
		Target string `json:"target"`
	}{pane}, &response); err != nil {
		return err
	}
	if response.Type != "agent_info" || response.Agent.PaneID != pane || response.Agent.TerminalID != terminal || response.Agent.Agent == "" || !current() {
		return fmt.Errorf("agent identity changed; reopen details")
	}
	return renameAgent(ctx, target, pane, name)
}

type Plugin struct {
	ID          string       `json:"plugin_id"`
	Name        string       `json:"name"`
	Version     string       `json:"version"`
	Description string       `json:"description"`
	Enabled     bool         `json:"enabled"`
	Warnings    []string     `json:"warnings"`
	Panes       []PluginPane `json:"panes"`
}
type PluginPane struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Placement string `json:"placement"`
}
type PluginAction struct {
	PluginID    string   `json:"plugin_id"`
	ID          string   `json:"action_id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Contexts    []string `json:"contexts"`
}
type PluginLog struct {
	ID       string   `json:"log_id"`
	PluginID string   `json:"plugin_id"`
	ActionID string   `json:"action_id"`
	Status   string   `json:"status"`
	Command  []string `json:"command"`
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	Error    string   `json:"error"`
	ExitCode *int     `json:"exit_code"`
}
type PluginCatalog struct {
	Plugins []Plugin
	Actions []PluginAction
	Logs    []PluginLog
}

func ReadPlugins(ctx context.Context, target Target) (PluginCatalog, error) {
	var result PluginCatalog
	var plugins struct {
		Type    string   `json:"type"`
		Plugins []Plugin `json:"plugins"`
	}
	if err := Call(ctx, target, "plugin.list", nil, &plugins); err != nil {
		return result, err
	}
	if plugins.Type != "plugin_list" {
		return result, fmt.Errorf("unexpected plugin list")
	}
	result.Plugins = plugins.Plugins
	var actions struct {
		Type    string         `json:"type"`
		Actions []PluginAction `json:"actions"`
	}
	if err := Call(ctx, target, "plugin.action.list", nil, &actions); err != nil {
		return result, err
	}
	if actions.Type != "plugin_action_list" {
		return result, fmt.Errorf("unexpected action list")
	}
	result.Actions = actions.Actions
	var logs struct {
		Type string      `json:"type"`
		Logs []PluginLog `json:"logs"`
	}
	if err := Call(ctx, target, "plugin.log.list", struct {
		Limit int `json:"limit"`
	}{30}, &logs); err != nil {
		return result, err
	}
	if logs.Type != "plugin_log_list" {
		return result, fmt.Errorf("unexpected plugin logs")
	}
	result.Logs = logs.Logs
	return result, nil
}
func SetPluginEnabled(ctx context.Context, target Target, id string, enabled bool) error {
	method := "plugin.disable"
	if enabled {
		method = "plugin.enable"
	}
	return Call(ctx, target, method, struct {
		ID string `json:"plugin_id"`
	}{id}, nil)
}

type PluginContext struct {
	PaneID      string `json:"focused_pane_id,omitempty"`
	TabID       string `json:"tab_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Source      string `json:"invocation_source"`
}

func InvokePluginAction(ctx context.Context, target Target, plugin, action string, invocation PluginContext) error {
	return Call(ctx, target, "plugin.action.invoke", struct {
		Plugin  string        `json:"plugin_id"`
		Action  string        `json:"action_id"`
		Context PluginContext `json:"context"`
	}{plugin, action, invocation}, nil)
}
func OpenPluginPane(ctx context.Context, target Target, plugin, entry, placement, pane string) error {
	if placement != "tab" && placement != "split" {
		return fmt.Errorf("this plugin entry requires Herdr's popup or overlay client")
	}
	return Call(ctx, target, "plugin.pane.open", struct {
		Plugin    string `json:"plugin_id"`
		Entry     string `json:"entrypoint"`
		Placement string `json:"placement"`
		Pane      string `json:"target_pane_id"`
		Direction string `json:"direction"`
		Focus     bool   `json:"focus"`
	}{plugin, entry, placement, pane, "right", false}, nil)
}

type OutputWatch struct {
	Text    string
	Regex   bool
	Timeout time.Duration
}
type OutputMatch struct {
	PaneID      string `json:"pane_id"`
	MatchedLine string `json:"matched_line"`
}

// WaitForOutput owns an independent connection. The caller bounds and cancels
// its lifetime; ordinary request lanes remain available throughout the wait.
func WaitForOutput(ctx context.Context, target Target, pane string, watch OutputWatch) (OutputMatch, error) {
	var result OutputMatch
	if watch.Text == "" || watch.Timeout <= 0 || watch.Timeout > 24*time.Hour {
		return result, fmt.Errorf("enter a match and a timeout up to 24 hours")
	}
	ctx, cancel := context.WithTimeout(ctx, watch.Timeout+time.Second)
	defer cancel()
	conn, err := localipc.DialContext(ctx, target.Socket)
	if err != nil {
		return result, err
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline, _ := ctx.Deadline()
	conn.SetDeadline(deadline)
	kind := "substring"
	if watch.Regex {
		kind = "regex"
	}
	params := struct {
		Pane    string `json:"pane_id"`
		Source  string `json:"source"`
		Strip   bool   `json:"strip_ansi"`
		Timeout int64  `json:"timeout_ms"`
		Lines   int    `json:"lines"`
		Match   struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"match"`
	}{Pane: pane, Source: "recent", Strip: true, Timeout: watch.Timeout.Milliseconds(), Lines: 80}
	params.Match.Type = kind
	params.Match.Value = watch.Text
	id := fmt.Sprintf("output-%d", connectionIDs.Add(1))
	if err = sendRequest(conn, id, "pane.wait_for_output", params); err != nil {
		return result, err
	}
	line, err := readLine(bufio.NewReader(conn))
	if err != nil {
		return result, err
	}
	var response struct {
		Type string `json:"type"`
		Pane string `json:"pane_id"`
		Line string `json:"matched_line"`
	}
	if err = decodeResponse(line, id, &response); err != nil {
		return result, err
	}
	if response.Type != "output_matched" || response.Pane != pane {
		return result, fmt.Errorf("unexpected output match identity")
	}
	return OutputMatch{response.Pane, response.Line}, nil
}

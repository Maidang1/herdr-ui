package app

import (
	"strings"
	"unicode"

	"github.com/Maidang1/herdr-ui/internal/session"
)

// NoticeText prepares the shared text for desktop notifications and in-app toasts.
func NoticeText(n Notice, snapshot *session.Snapshot, agent string) Notice {
	if n.Kind != "NeedsAttention" && n.Kind != "Finished" {
		return n
	}
	agent = strings.TrimSpace(agent)
	workspaceID, tabID := n.WorkspaceID, n.TabID
	var project, task string
	if snapshot != nil {
		if pane, ok := snapshot.Pane(n.PaneID); ok {
			workspaceID, tabID = pane.WorkspaceID, pane.TabID
			if agent == "" && strings.TrimSpace(pane.Agent) != "" {
				agent = pane.Agent
			}
			task = noticeLabel(pane.Label)
			if task == "" {
				task = noticeLabel(pane.Title)
			}
			if task == pane.ID || task == pane.TerminalID {
				task = ""
			}
		}
		if tab, ok := snapshot.Tab(tabID); ok {
			if workspaceID == "" {
				workspaceID = tab.WorkspaceID
			}
			if task == "" {
				task = noticeLabel(tab.Label)
			}
		}
		if workspace, ok := snapshot.Workspace(workspaceID); ok {
			project = noticeLabel(workspace.Label)
		}
	}
	agent = strings.TrimSpace(agent)
	if agent == "" {
		for _, suffix := range []string{" needs attention", " finished"} {
			if strings.HasSuffix(n.Title, suffix) {
				agent = strings.TrimSpace(strings.TrimSuffix(n.Title, suffix))
				break
			}
		}
	}
	if agent == "" {
		agent = "Agent"
	}
	if n.Kind == "NeedsAttention" {
		n.Title = agent + " is waiting for input"
	} else {
		n.Title = agent + " completed the task"
	}

	// Herdr's generated body is workspace · workspace-number [· tab-label].
	// Only that shape is location boilerplate: arbitrary message text, including
	// task numbers, must survive unchanged.
	body := n.Body
	parts := strings.Split(body, " · ")
	if len(parts) >= 2 && noticeLabel(parts[1]) == "" && strings.TrimSpace(parts[1]) != "" {
		if project == "" {
			project = noticeLabel(parts[0])
		}
		body = ""
		for _, part := range parts[2:] {
			if label := noticeLabel(part); label != "" && label != project && label != task {
				if task == "" {
					task = label
					continue
				}
				if body != "" {
					body += " · "
				}
				body += label
			}
		}
	}
	context := project
	if task != "" && task != project {
		if context != "" {
			context += " · "
		}
		context += task
	}
	if body == project || body == task || body == context {
		body = ""
	}
	if context != "" && body != "" {
		n.Body = context + "\n" + body
	} else {
		n.Body = context + body
	}
	return n
}

func noticeLabel(label string) string {
	label = strings.TrimSpace(label)
	if strings.TrimFunc(label, unicode.IsDigit) == "" {
		return ""
	}
	return label
}

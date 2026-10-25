package ui

import (
	"fmt"

	"github.com/Maidang1/herdr-ui/internal/app"
)

// A prompt names what it acts on and says what follows, as a Mac alert
// does: the title asks about the specific tab, workspace or pane, the
// message gives the consequence, and the default button is the verb.

// promptFor is the dialog an action asks with before acting on the view's
// workspace, tab or pane. Its submit and focus are the caller's.
func promptFor(action string, v app.View) shortcutPrompt {
	switch action {
	case "close_tab":
		message := "Its pane closes, and the program running in it ends."
		if n := len(v.Panes); n > 1 {
			message = fmt.Sprintf("Its %d panes close, and the programs running in them end.", n)
		}
		return shortcutPrompt{title: "Close " + named("tab", tabName(v)) + "?", message: message, action: "Close Tab", destructive: true, confirm: true}
	case "close_workspace":
		return shortcutPrompt{title: "Close " + named("workspace", v.WorkspaceName) + "?", message: "Its tabs close along with the workspaces linked to it, and the programs running in them end.", action: "Close Workspace", destructive: true, confirm: true}
	case "remove_worktree":
		return shortcutPrompt{title: "Remove the worktree checkout of " + named("workspace", v.WorkspaceName) + "?", message: "Its folder is deleted. The branch and its commits stay in the repository.", action: "Remove Checkout", destructive: true, confirm: true}
	case "rename_workspace":
		return shortcutPrompt{title: "Rename workspace", value: v.WorkspaceName, placeholder: "Workspace name", action: "Rename"}
	case "rename_tab":
		return shortcutPrompt{title: "Rename tab", value: tabName(v), placeholder: "Tab name", action: "Rename"}
	case "rename_pane":
		value := ""
		for _, pane := range v.Panes {
			if pane.ID == v.PaneID {
				value = pane.Label
			}
		}
		return shortcutPrompt{title: "Rename pane", value: value, placeholder: "Pane name", action: "Rename"}
	case "new_worktree":
		return shortcutPrompt{title: "New worktree", message: "Creates a Git worktree on a new branch of " + named("workspace", v.WorkspaceName) + " and opens it as a workspace.", placeholder: "Branch name", action: "Create", required: true}
	case "open_worktree":
		return shortcutPrompt{title: "Open worktree", message: "Opens an existing Git worktree as a workspace.", placeholder: "Path to the worktree", action: "Open", required: true}
	}
	return shortcutPrompt{title: titleCase(action), action: "OK", confirm: true}
}

// closeAgentPrompt asks before ending the agent running in a pane.
func closeAgentPrompt(agent string) shortcutPrompt {
	return shortcutPrompt{title: "Close the pane running " + agent + "?", message: agent + " stops, and its session in this pane ends.", action: "Close Pane", destructive: true, confirm: true}
}

// groupClosePrompt asks before a close that takes linked workspaces with it.
func groupClosePrompt(workspace string) shortcutPrompt {
	return shortcutPrompt{title: "Close linked workspaces?", message: "Closing the last pane of " + named("workspace", workspace) + " closes it and every workspace linked to it.", action: "Close All", destructive: true, confirm: true}
}

// searchPrompt asks for the text copy mode searches its history for.
func searchPrompt(query string, reverse bool) shortcutPrompt {
	title := "Search history"
	if reverse {
		title = "Search history backward"
	}
	return shortcutPrompt{title: title, value: query, placeholder: "Text to find", action: "Search", required: true}
}

func tabName(v app.View) string {
	for _, tab := range v.Tabs {
		if tab.ID == v.TabID {
			return tab.Name
		}
	}
	return ""
}

// named is a thing by its name, as in tab “Server”, or "this tab" when it
// has none.
func named(kind, name string) string {
	if name == "" {
		return "this " + kind
	}
	return kind + " “" + name + "”"
}

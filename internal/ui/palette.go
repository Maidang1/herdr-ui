package ui

import (
	"strings"

	"herdr-ui/internal/herdr"

	"github.com/egoist/mygo/ui"
)

// The command palette lists the commands the daemon offers, so that the user
// can run one without knowing where it lives.

// drawPalette shows the commands of the session.
func (a *App) drawPalette(c *ui.Context, snapshot *herdr.Snapshot, query string) {
	theme := c.Theme()
	commands := filterCommands(snapshot, query)
	entries := a.paletteEntries(commands, a.localCommands())

	ui.Modal(c, &a.palette, func() {
		ui.Column(c).Width(560).MaxWidth(560).Padding(12).Gap(8).
			Background(theme.Surface).Border(1, theme.Border).Radius(10).Children(func() {
			value := query
			field := ui.TextInput(c, &value)
			field.Placeholder("Search the commands")
			field.AutoFocus()
			field.OnChange(func() {
				a.mu.Lock()
				a.query = value
				a.mu.Unlock()
			})
			field.OnSubmit(func() {
				if len(entries) > 0 {
					entries[0].run()
					a.closePalette()
				}
			})
			ui.Column(c).MaxHeight(320).Children(func() {
				for _, entry := range entries {
					entry := entry
					row := ui.Row(c).Padding(6, 8).Gap(8).Radius(6)
					if row.Hovered() {
						row.Background(theme.SurfaceHover)
					}
					if row.Clicked() {
						entry.run()
						a.closePalette()
					}
					row.Children(func() {
						ui.Text(c, entry.description).Grow(1)
						if entry.binding != "" {
							ui.Text(c, entry.binding).FontSize(11).TextColor(theme.TextMuted)
						}
					})
				}
				if len(entries) == 0 {
					ui.Text(c, "No command matches.").TextColor(theme.TextMuted).FontSize(12)
				}
			})
		})
	})
}

// closePalette closes the command palette.
func (a *App) closePalette() {
	a.mu.Lock()
	a.palette = false
	a.query = ""
	a.mu.Unlock()
	a.invalidate()
}

// runCommand asks the daemon to carry out a command.
func (a *App) runCommand(commandID string) {
	if commandID == "" {
		return
	}
	go func() {
		ctx, cancel := contextWithTimeout()
		defer cancel()
		if err := a.endpoint.Request(ctx, "command.invoke", map[string]any{"command_id": commandID}, nil); err != nil {
			a.setStatus("command: %v", err)
			return
		}
		a.setStatus("ran the command")
	}()
}

// paletteEntry is one line of the command palette.
type paletteEntry struct {
	// description is what the line says.
	description string
	// binding is the key that runs it, when there is one.
	binding string
	// run carries it out.
	run func()
}

// paletteEntries puts the commands of the daemon and the commands of the
// window in one list.
func (a *App) paletteEntries(commands []herdr.Command, local []localCommand) []paletteEntry {
	entries := make([]paletteEntry, 0, len(commands)+len(local))
	for _, command := range local {
		entries = append(entries, paletteEntry{
			description: command.description,
			binding:     command.binding,
			run:         command.run,
		})
	}
	for _, command := range commands {
		command := command
		entries = append(entries, paletteEntry{
			description: command.Description,
			binding:     command.BindingLabel,
			run: func() {
				a.runCommand(command.CommandID)
			},
		})
	}
	return entries
}

// localCommand is a command the window carries itself.
type localCommand struct {
	// description is what the command says.
	description string
	// binding is the key that runs it.
	binding string
	// run carries it out.
	run func()
}

// localCommands are the commands of the window itself.
func (a *App) localCommands() []localCommand {
	return []localCommand{
		{description: "Settings", binding: "⌘,", run: a.openSettings},
		{description: "Copy mode", binding: "⇧⌘C", run: a.enterCopyMode},
		{description: "Find in the output", binding: "⌘F", run: a.openFind},
		{description: "Release notes", binding: "", run: a.openUpdatePanel},
	}
}

// filterCommands returns the commands that match what the user typed.
func filterCommands(snapshot *herdr.Snapshot, query string) []herdr.Command {
	if snapshot == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	var commands []herdr.Command
	for _, command := range snapshot.Commands {
		if query == "" {
			commands = append(commands, command)
			continue
		}
		haystack := strings.ToLower(strings.Join([]string{
			command.Description,
			command.Action,
			command.BindingLabel,
			command.CommandID,
		}, " "))
		if strings.Contains(haystack, query) {
			commands = append(commands, command)
		}
	}
	return commands
}

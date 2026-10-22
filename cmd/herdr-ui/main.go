// Command herdr-ui is a native window for the Herdr daemon.
//
// With no arguments it opens the window. With a command it asks a running
// window to do something, over the control socket.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"herdr-ui/internal/control"
	"herdr-ui/internal/herdr"
	"herdr-ui/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-ui:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("herdr-ui", flag.ContinueOnError)
	socket := flags.String("socket", "", "the control socket of a running window")
	cols := flags.Int("cols", 160, "how many cells the window shows")
	rows := flags.Int("rows", 45, "how many rows the window shows")
	ssh := flags.String("ssh", "", "the host to reach over SSH, as user@host")
	session := flags.String("session", "default", "the session of the host to reach")
	if err := flags.Parse(args); err != nil {
		return err
	}
	rest := flags.Args()

	if len(rest) == 0 {
		return ui.Run(ui.Options{
			ControlSocket: *socket,
			CellWidthPx:   8,
			CellHeightPx:  17,
			Cols:          uint16(*cols),
			Rows:          uint16(*rows),
			SSHTarget:     *ssh,
			SSHSession:    *session,
		})
	}

	path := *socket
	if path == "" {
		resolved, err := control.DefaultSocketPath()
		if err != nil {
			return err
		}
		path = resolved
	}
	client := control.NewClient(path)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	switch rest[0] {
	case "status":
		response, err := client.CallAndCheck(ctx, control.Request{Method: "status"})
		if err != nil {
			return err
		}
		fmt.Print(control.Describe(response))
		return nil
	case "browser":
		return browser(ctx, client, rest[1:])
	case "notes":
		return notes(ctx, client, rest[1:])
	case "tab":
		return tab(ctx, client, rest[1:])
	case "window":
		return window(ctx, client, rest[1:])
	case "command":
		return command(ctx, client, rest[1:])
	case "agent":
		return agent(ctx, client, rest[1:])
	case "find":
		return find(ctx, client, rest[1:])
	default:
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

// browser carries out the browser commands.
func browser(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("browser needs a command: open, list, close, reload, navigate, back, forward, text, zoom, feedback")
	}
	switch args[0] {
	case "open":
		if len(args) < 2 {
			return fmt.Errorf("browser open needs an address or a file")
		}
		response, err := client.CallAndCheck(ctx, control.Request{
			Method: "browser.open",
			Page:   &control.PageTarget{URL: args[1]},
		})
		if err != nil {
			return err
		}
		if response.Page != nil {
			fmt.Printf("opened %s %s\n", response.Page.ID, response.Page.URL)
			return nil
		}
		fmt.Println(control.Describe(response))
		return nil
	case "list":
		response, err := client.CallAndCheck(ctx, control.Request{Method: "browser.list"})
		if err != nil {
			return err
		}
		fmt.Print(control.Describe(response))
		return nil
	case "close", "reload", "back", "forward", "text":
		if len(args) < 2 {
			return fmt.Errorf("browser %s needs a page", args[0])
		}
		response, err := client.CallAndCheck(ctx, control.Request{
			Method: "browser." + args[0],
			Page:   &control.PageTarget{ID: args[1]},
		})
		if err != nil {
			return err
		}
		fmt.Print(control.Describe(response))
		return nil
	case "navigate":
		if len(args) < 3 {
			return fmt.Errorf("browser navigate needs a page and an address")
		}
		response, err := client.CallAndCheck(ctx, control.Request{
			Method: "browser.navigate",
			Page:   &control.PageTarget{ID: args[1]},
			URL:    args[2],
		})
		if err != nil {
			return err
		}
		fmt.Print(control.Describe(response))
		return nil
	case "zoom":
		if len(args) < 2 {
			return fmt.Errorf("browser zoom needs a page and a change")
		}
		change := "+"
		if len(args) > 2 {
			change = args[2]
		}
		response, err := client.CallAndCheck(ctx, control.Request{
			Method: "browser.zoom",
			Page:   &control.PageTarget{ID: args[1]},
			Change: change,
		})
		if err != nil {
			return err
		}
		fmt.Printf("zoom %d%%\n", int(response.Zoom))
		return nil
	case "feedback":
		if len(args) < 2 {
			return fmt.Errorf("browser feedback needs notes")
		}
		response, err := client.CallAndCheck(ctx, control.Request{
			Method: "browser.feedback",
			Text:   strings.Join(args[1:], " "),
		})
		if err != nil {
			return err
		}
		fmt.Print(control.Describe(response))
		return nil
	default:
		return fmt.Errorf("unknown browser command %q", args[0])
	}
}

// notes carries out the notes commands.
func notes(ctx context.Context, client *control.Client, args []string) error {
	if len(args) < 2 || args[0] != "send" {
		return fmt.Errorf("notes needs the command send and the notes")
	}
	response, err := client.CallAndCheck(ctx, control.Request{
		Method: "notes.send",
		Text:   strings.Join(args[1:], " "),
	})
	if err != nil {
		return err
	}
	fmt.Print(control.Describe(response))
	return nil
}

// tab carries out the tab commands.
func tab(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("tab needs a command: list, focus, close")
	}
	request := control.Request{Method: "tab", Op: args[0]}
	if len(args) > 1 {
		request.Target = args[1]
	}
	response, err := client.CallAndCheck(ctx, request)
	if err != nil {
		return err
	}
	fmt.Print(control.Describe(response))
	return nil
}

// window carries out the window commands.
func window(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("window needs a command: list, focus")
	}
	request := control.Request{Method: "window", Op: args[0]}
	if len(args) > 1 {
		request.Target = args[1]
	}
	response, err := client.CallAndCheck(ctx, request)
	if err != nil {
		return err
	}
	fmt.Print(control.Describe(response))
	return nil
}

// command carries out the command commands.
func command(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command needs a command: list, invoke")
	}
	request := control.Request{Method: "command", Op: args[0]}
	if len(args) > 1 {
		request.Target = args[1]
	}
	response, err := client.CallAndCheck(ctx, request)
	if err != nil {
		return err
	}
	fmt.Print(control.Describe(response))
	return nil
}

// agent carries out the agent commands.
func agent(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		args = []string{"list"}
	}
	request := control.Request{Method: "agent", Op: args[0]}
	response, err := client.CallAndCheck(ctx, request)
	if err != nil {
		return err
	}
	fmt.Print(control.Describe(response))
	return nil
}

// find carries out the find commands.
func find(ctx context.Context, client *control.Client, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("find needs a command: search, next, previous, close")
	}
	request := control.Request{Method: "find", Op: args[0]}
	if len(args) > 1 {
		request.Text = strings.Join(args[1:], " ")
	}
	response, err := client.CallAndCheck(ctx, request)
	if err != nil {
		return err
	}
	for _, note := range response.Notes {
		fmt.Println(note.Text)
	}
	return nil
}

// daemonSocketPath says where the daemon's client socket is, for the help.
func daemonSocketPath() string {
	path, err := herdr.ClientSocketPath()
	if err != nil {
		return ""
	}
	return path
}

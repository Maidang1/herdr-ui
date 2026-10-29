package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/endpoint"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/input"
	"github.com/Maidang1/herdr-ui/internal/session"
	view "github.com/Maidang1/herdr-ui/internal/ui"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// keyboardEndpoint maintains the passive command/notification projection. Its
// lifecycle follows the window, independently of terminal attach processes.
type keyboardEndpoint struct {
	mu                   sync.Mutex
	client               *endpoint.Client
	notificationClient   *endpoint.Client
	consumedNotification uint64
}

func (k *keyboardEndpoint) current() *endpoint.Client {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.client
}
func (k *keyboardEndpoint) run(ctx context.Context, socket string, changed func()) {
	for ctx.Err() == nil {
		client := endpoint.New(socket)
		wake := make(chan struct{}, 1)
		client.SetChanged(func() {
			changed()
			select {
			case wake <- struct{}{}:
			default:
			}
		})
		k.mu.Lock()
		k.client = client
		k.mu.Unlock()
		if err := client.Start(ctx); err == nil {
			for client.Latest().Connected && ctx.Err() == nil {
				select {
				case <-ctx.Done():
				case <-wake:
				}
			}
		}
		client.Close()
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func keyboardHooks(ctx context.Context, s *session.Session, k *keyboardEndpoint, screen *view.Window, post func(func()), closeWindow func(), binary string, target herdr.Target) view.ActionHooks {
	_, env, _ := target.Attach(binary, "keyboard-overlay", os.Environ())
	return view.ActionHooks{
		Detach: closeWindow,
		// The settings window replaces this hook in state.go; the
		// config editor below stays reachable as EditConfig from the
		// Shortcuts page.
		Settings:       func() error { return editHerdrConfigWith(binary, target, screen, env) },
		WriteClipboard: func(text string) error { return mygo.Clipboard.Write(transfer.TextData(text)) },
		RevealPath:     func(path string) { mygo.Shell.ShowItemInFolder(path) },
		ReadImage:      func() []byte { return mygo.Clipboard.ReadImage() },
		EditText: func(text string) error {
			file, err := os.CreateTemp("", "herdr-history-*.txt")
			if err != nil {
				return err
			}
			path := file.Name()
			if _, err = file.WriteString(text); err != nil {
				file.Close()
				os.Remove(path)
				return err
			}
			if err = file.Close(); err != nil {
				return err
			}
			command, err := commandForEditor(path)
			if err != nil {
				return err
			}
			return screen.OpenCommandSurface("Scrollback editor", command, env, "")
		},
		PasteImage: func(pane string, data []byte) error {
			client := k.current()
			if client == nil || !client.Latest().Connected {
				return fmt.Errorf("Herdr image endpoint is disconnected")
			}
			cols, rows := screen.SurfaceSize()
			client.SetSurfaceSize(cols, rows)
			go func() {
				call, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				err := client.SendImage(call, pane, "png", data)
				post(func() { screen.ReportActionError(err) })
			}()
			return nil
		},
		InvokeCommand: func(index int, op session.Operation, done func(error)) {
			bindings, err := input.LoadBindings("")
			if err != nil {
				done(err)
				return
			}
			client := k.current()
			if client == nil {
				done(fmt.Errorf("Herdr command catalog is disconnected"))
				return
			}
			id, err := matchCommand(client.Latest(), bindings, index)
			if err != nil {
				done(err)
				return
			}
			intentRevision := screen.SelectionRevision()
			publicationRevision := s.Latest().Revision
			if bindings.Commands[index].Type != "plugin_action" {
				op.SelectedText = nil
			}
			op.Action = "invoke_command"
			op.CommandID = id
			result := s.Execute(ctx, op)
			go func() {
				r := <-result
				publicationRevision = max(publicationRevision, s.Latest().Revision)
				post(func() {
					if r.Status != session.Succeeded {
						done(fmt.Errorf("%s: %s", r.Status, r.Error))
						return
					}
					if bindings.Commands[index].Type == "popup" || bindings.Commands[index].Type == "plugin_action" {
						if err := screen.OpenCommandSurface("Herdr command", []string{binary}, env, ""); err != nil {
							done(err)
							return
						}
					} else if bindings.Commands[index].Type == "pane" {
						screen.FollowCommandFocus(op.TargetID, op.Epoch, intentRevision, publicationRevision, op.PaneID)
					}
					done(nil)
				})
			}()
		},
		Notices: func() []app.Notice { return k.notices(s.Latest().Snapshot) },
	}
}
func configPath() string {
	home, _ := os.UserHomeDir()
	return herdr.ConfigPath(home, os.Getenv)
}

// editHerdrConfig opens Herdr's own configuration in the user's editor,
// inside the window's command surface. The settings window's Shortcuts
// page and any fallback Settings path both land here.
func editHerdrConfig(binary string, target herdr.Target, screen *view.Window) error {
	_, env, _ := target.Attach(binary, "keyboard-overlay", os.Environ())
	return editHerdrConfigWith(binary, target, screen, env)
}

func editHerdrConfigWith(_ string, _ herdr.Target, screen *view.Window, env []string) error {
	path := configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte("# Herdr configuration\n[keys]\nprefix = \"ctrl+b\"\n"), 0600); err != nil {
			return err
		}
	}
	command, err := commandForEditor(path)
	if err != nil {
		return err
	}
	return screen.OpenCommandSurface("Herdr configuration", command, env, "")
}
func matchCommand(state endpoint.State, b input.Bindings, index int) (string, error) {
	if !state.Connected || state.Snapshot == nil {
		return "", fmt.Errorf("Herdr command catalog is not ready")
	}
	if index < 0 || index >= len(b.Commands) {
		return "", fmt.Errorf("command binding no longer exists")
	}
	action := map[string]string{"shell": "Shell", "pane": "Pane", "popup": "Popup", "plugin_action": "PluginAction"}[b.Commands[index].Type]
	name := fmt.Sprintf("command[%d]", index)
	labels := map[string]bool{}
	for label, a := range b.Actions {
		if a == name {
			labels[label] = true
		}
	}
	id := ""
	for _, command := range state.Snapshot.Commands {
		if command.Action != action {
			continue
		}
		matched := false
		for _, label := range append(append([]string{}, command.BindingLabels...), command.BindingLabel) {
			normalized, err := input.Normalize(strings.TrimSpace(label))
			if err == nil && labels[normalized] {
				matched = true
			}
		}
		if matched {
			if id != "" && id != command.CommandID {
				return "", fmt.Errorf("ambiguous command binding; reload configuration")
			}
			id = command.CommandID
		}
	}
	if id == "" {
		return "", fmt.Errorf("command binding is absent from server catalog; reload configuration")
	}
	return id, nil
}

// notices returns the notifications received since the last call. A new
// connection starts its own sequence, so its whole backlog is new.
func (k *keyboardEndpoint) notices(snapshot *session.Snapshot) []app.Notice {
	client := k.current()
	if client == nil {
		return nil
	}
	state := client.Latest()
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.notificationClient != client {
		k.notificationClient = client
		k.consumedNotification = 0
	}
	var fresh []app.Notice
	for _, n := range state.Notifications {
		if n.ID <= k.consumedNotification {
			continue
		}
		k.consumedNotification = n.ID
		notice := app.Notice{Kind: n.Kind, Title: n.Title, Body: n.Body, WorkspaceID: n.WorkspaceID, TabID: n.TabID, PaneID: n.PaneID, Position: n.Position, Received: n.ReceivedAt}
		fresh = append(fresh, app.NoticeText(notice, snapshot, n.Agent))
	}
	return fresh
}

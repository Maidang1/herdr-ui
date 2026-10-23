# Keyboard support

The native window now dispatches the complete Herdr action catalog on its supported macOS key-event surface. A binding is no longer considered implemented merely because its action name can be parsed.

Default prefix: Ctrl+B. Release the prefix before pressing the action key. Press it twice to send the prefix literally. Custom prefix and bindings are read from `HERDR_CONFIG_PATH`, then the standard XDG/user config location.

| Area | Actions |
| --- | --- |
| Workspaces | Create, rename, close, previous/next, numbered selection, workspace navigation |
| Worktrees | Create a branch/checkout, open a checkout by path, remove a checkout after confirmation |
| Tabs | Create, rename, close, previous/next, numbered selection, move previous/next with wrapping |
| Panes | Split, directional focus, swap, cycle, last pane, rename, close, zoom, resize mode and individual resize actions |
| Agents | Previous/next and numbered focus |
| Navigation | Workspace preview with Enter/Escape, configured directional keys, implicit prefix actions, searchable workspace/tab/pane/agent navigator |
| History | Copy mode, character/line selection, word/paragraph/page motions, smart-case search and repeat, clipboard copy, scrollback editor |
| Client | Help, settings, config reload, sidebar toggle, detach, visible notification target, image paste |
| Custom commands | Shell, pane, popup, plugin actions; lookup uses the server's opaque command catalog |

Ordinary navigation remains local to the window. Navigating to a hidden pane in a zoomed tab uses explicit server focus and waits for the matching layout. Mutations capture workspace/tab/pane identity; later selection changes cannot retarget an already queued operation. Network-uncertain mutations are not retried.

Settings and scrollback open an embedded editor using `$EDITOR` (default `vi`). Saving settings does not automatically reload them: use the reload-config shortcut. Popup and plugin commands use an embedded Herdr client to display server-owned interactive surfaces. This command view owns its keyboard input and can be detached with its close button.

Copy mode captures the pane's full history using revision-checked selection extraction, avoiding the `pane.read` 1,000-line limit. Terminal output can continue behind the capture. A plugin invoked with selected text resolves actual server cell coordinates and verifies the extracted text before invocation; changed or ambiguous history produces an explicit rejection instead of silently supplying different text.

Notifications and custom-command metadata arrive through a passive generation-1 endpoint connection. Image paste briefly activates a surface for the explicit target pane, sends the image, then releases the surface; Herdr requires this and it can change server foreground focus/geometry. Without an image, the normal key input proceeds to the terminal.

Invalid or conflicting individual keybindings appear as diagnostics and are disabled without disabling valid bindings. Supported custom configuration includes arrays, indexed overlays, navigation bindings, command bindings and the `fullscreen` alias.

The pinned mygo native key API exposes F1–F12 and Ctrl/Alt/Shift/Command. It cannot emit F13–F255 or Herdr's independent Hyper modifier. Those configurations are diagnosed and disabled; supporting them requires extending the upstream native event API. No claim is made that these unavailable hardware events work.

Validation commands:

```sh
./scripts/go.sh test -race ./...
./scripts/go.sh vet ./...
make build
HERDR_UI_M0_WINDOW_BINARY="$PWD/bin/herdr-ui" ./scripts/go.sh test -tags integration ./integration -v -count=1
```

Integration tests create isolated sockets, configuration, repositories and worktrees under `/tmp`; they do not connect to existing user sessions.

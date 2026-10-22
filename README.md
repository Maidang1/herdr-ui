# herdr-ui

A native window for the [Herdr](https://herdr.dev/) daemon, written in Go
with [mygo](https://github.com/egoist/mygo).

The daemon owns the terminals and the session: workspaces, tabs, panes and
the coding agents that work in them. This program is a client of that
daemon. It paints the terminals the daemon sends, and it answers the
commands that ask it to open a page, to focus a tab or to send notes.

It is a rewrite of the Rust GUI in `herdr-gpui`, in Go and mygo.

## Requirements

- Go 1.27.1 or later (mygo asks for it; Go fetches the toolchain itself).
- A running Herdr daemon. The window talks to the daemon's client socket.

## Build and run

```sh
go build -o herdr-ui ./cmd/herdr-ui
./herdr-ui
```

The window opens on the session the daemon is running. It listens on a
control socket, so a second copy of the program can drive it:

```sh
herdr-ui status
herdr-ui browser open https://example.com
herdr-ui browser list
herdr-ui browser text page-1
herdr-ui browser zoom page-1 150
herdr-ui notes send "the tests pass"
herdr-ui tab list
herdr-ui command list
herdr-ui agent list
herdr-ui find search "error"
herdr-ui find next
```

The socket is `$XDG_STATE_HOME/herdr/herdr-ui/control.sock`, or
`~/.local/state/herdr/herdr-ui/control.sock`. `HERDR_UI_CONTROL_SOCKET` and
the `--socket` flag move it.

A daemon on another machine is reached over SSH: the ssh binary does the
encryption, the remote herdr bridges its client socket to the connection,
and the window speaks the same protocol over it.

```sh
./herdr-ui --ssh user@host
./herdr-ui --ssh user@host --session work
```

The window of a remote host keeps its own control socket, so that it does
not answer for the window of this machine.

## What the window shows

- A sidebar with the workspaces of the session, the agents that work in
  them, the worktrees of the repository and the coding agents the machine
  has installed. Clicking a workspace focuses it in the daemon.
- A tab bar with the tabs of the focused workspace, with the browser pages
  beside them, with the status the daemon puts on the right, and with a
  button that asks for another tab.
- A connection that keeps its ground: when the daemon drops it, the window
  keeps the picture of the session under a card that says why it is waiting
  and offers to try at once, and the endpoint opens the connection again on
  its own, with the keyboard and the session as they were.
- The terminals, painted from the cells the daemon sends: the colors the
  programs asked for, the cursor, the wide characters and the backgrounds.
  Each pane carries a title above its content, and aiming at a pane focuses
  it, the way the multiplexer does.
- The keyboard belongs to the pane that has the focus: what the user types
  goes to it, letter by letter, and the named keys, the chords and the
  function keys travel the way the Rust client sends them.
- A selection, drawn with the pointer over the cells it chooses and copied
  when the pointer comes up. Cmd-C copies it again, and Cmd-V pastes the
  clipboard into the pane.
- A copy mode, on Cmd-Shift-C: a cursor that walks the output of the pane
  and the scrollback above it, the vi keys move it, `v` marks what it
  chooses, `y` copies it, and `/` searches the scrollback.
- The links of the output: the daemon reads the cells a link covers and
  underlines them while the pointer is over them, and a click with the
  command held opens the address, unless a plugin of the session claims it
  first. An address the daemon does not read is read by the window itself,
  and a path on this machine opens in a page of its own.
- A note on the text the user chose: a released selection offers a pill, and
  the note that is written goes to the agent of the pane, in the words the
  Rust client uses, so that the quoted text reads as data and not as
  instructions. An agent that is not there to read it leaves the note on
  the clipboard.
- A menu on the right-click of a pane: split it right or down, zoom it,
  scroll its output, begin the copy mode, focus it or close it.
- A status bar with the title, the folder and the agent of the pane. Typing
  there sends the text to that pane.
- Browser pages, each a web view. A local file is served from the folder it
  sits in, so that the files it refers to load beside it.
- A command palette, on Cmd-K, listing the commands the daemon offers with
  their key bindings.
- A find bar, on Cmd-F, that searches the output of the pane and marks the
  match it is on. The window searches the cells it has painted, which are
  the cells the user looks at: the daemon answers a search of its own with
  the revision of the content it searched, and the content of a live pane
  moves on before the answer arrives.
- A banner for what the daemon has to say: a new version, a release note
  or an announcement, which the user can dismiss. The banner opens a panel
  with the notes of the release and the command that installs the update,
  when the daemon offers one.
- The settings of the window, read from the files the Rust client uses:
  `config-gpui.toml` holds the managed defaults and `config-gpui.local.toml`
  the overrides the user writes. The window reads the size of the
  characters of the terminal and of the window, the height of a row, whether
  a selection stays highlighted after it was copied, the theme, the layout of
  the sidebar, the notices and the shortcuts, and writes its changes into the
  overrides one key at a time, so that the comments in that file stay as they
  are. Cmd-, opens them, and the command palette holds them among the
  commands of the daemon.
- A theme for the terminal, the way the Rust client has it: a Ghostty theme
  file out of `~/.config/herdr/themes`, `~/.config/ghostty/themes` or the
  themes of an installed Ghostty, or one of the palettes the Rust client
  writes by hand. The theme paints the terminal only; the rest of the window
  follows the system. A theme may name one for the light side of the system
  and another for the dark side, as in `light:Nord,dark:Dracula`. The name
  `Follow Herdr` takes the theme the daemon names in its own configuration,
  and finds the theme file of that name among the ones of the system.
- A layout for the sidebar, the way the Rust client has it: `normal`,
  `compact`, `comfortable`, and any of them with `-rounded` at the end. The
  rounded layouts draw the rows as cards with rounded, bordered highlights
  instead of the rows of the system.
- The notices of the daemon, as cards in the corner of the terminal, with a
  sound of the system for a task that is done and for an agent that asks for
  attention. The bell of a pane reaches the window as well: the settings say
  whether the icon of the window bounces and whether the alert sound plays.
- The shortcuts of the window, which the user may change: the settings hold
  them under `[keybindings]`, in the words the Rust client uses
  (`cmd-shift-n`), and the window writes them into the overrides the same
  way it writes the other settings.

## How it is put together

```
cmd/herdr-ui          the program and its commands
internal/bincode      the bincode 2 subset the wire model uses
internal/herdr        the daemon protocol: hello, snapshots, requests, surfaces
internal/ui           the window: sidebar, tabs, terminals, browser pages
internal/config       the settings of the window, read and written in place
internal/theme        the themes of the terminal, Ghostty files among them
internal/control      the control socket and the client that talks to it
```

### The daemon protocol

The daemon's client socket speaks bincode behind a little endian u32 length.
The first frame is a named hello; the daemon answers with a welcome that
lists the methods it serves, and then pushes a snapshot of the session and
the painted surfaces.

The numbers bincode uses are tagged rather than packed: a number that fits
in 250 is one byte, and otherwise a marker byte says how many bytes follow
(251 for u16, 252 for u32, 253 for u64). A signed number is written the way
zigzag writes it. `internal/bincode` implements that, and its tests carry
the bytes the Rust client sends, so a change that breaks the wire shows up
as a failing test rather than as a daemon that hangs up.

A snapshot arrives as JSON inside a named control message, and so does the
answer to a request. The painted surfaces arrive as bincode: the cells of
the terminal grid, one pane per rectangle of it.

### The control protocol

The window answers one JSON request per connection, on its own socket. The
request names a method and says what to act on; the answer says whether the
request was carried out.

```json
{"method": "browser.open", "page": {"url": "https://example.com"}}
{"status": "opened", "page": {"id": "page-1", "url": "https://example.com"}}
```

The methods are `status`, `app`, `window`, `tab`, `command`, `agent`,
`notes.send`, `browser.open`, `browser.list`, `browser.close`,
`browser.reload`, `browser.navigate`, `browser.back`, `browser.forward`,
`browser.text`, `browser.zoom`, `browser.feedback` and `find`.

## Tests

```sh
go test ./...
```

The tests cover the bincode encoding against the bytes the Rust client
sends, the wire model, the control socket, the colours of the cells, the
themes, the shortcuts and the settings of the window, and the window itself,
which is laid out in memory by mygo's tester. The pointer
and the keyboard are driven there too: the tester presses, moves and
releases over the terminal, and types into it.

The live test talks to the daemon the machine runs, and is skipped unless
it is asked for:

```sh
HERDR_UI_LIVE=1 go test ./internal/herdr/ -run TestLive -v
```

Two of the live tests need a pane of their own to type into, and say so
when the variable is missing:

```sh
HERDR_UI_LIVE=1 HERDR_UI_PANE=w1:p1 go test ./internal/herdr/ -run TestLive -v
```

## What this rewrite does not cover

The Rust GUI in `herdr-gpui` is a large program: it also drives the
plugins, the worktrees, the teleport between machines, the security
scanner, the updater and the agent views. This rewrite covers the part that
makes a window a window — the session, the terminals, the panes and their
menus, the browser pages, the command palette, the worktrees, the agents
and the control socket — and leaves the rest to the daemon, which already
owns most of it.

What is still missing: the sessions of the Rust client, which open and drive
the windows of other machines, and the parts of its settings that belong to
the cloud features it has (Azure, Boat, Coder, Daytona). The window honours
the settings that shape it, and leaves the rest to the Rust client.

The updater panel shows what the daemon says about the version and the
command it gives for installing it; the download itself is the daemon's,
and the Rust GUI does it with its own updater, which signs a manifest and
unpacks it.

The daemon does not serve its `plugin.*` methods to endpoint clients, the
way the Rust GUI notes, so there is no plugin panel here either: `herdr
plugin` on the host is the way to reach them.

The copy mode is the part of the Rust GUI that leans most on the daemon's
own text: the motions that depend on the words of the output, and the
searches of the scrollback, are the daemon's, and the daemon refuses a
search whose revision has moved on, which the window answers by trying
again with the revision it has painted.

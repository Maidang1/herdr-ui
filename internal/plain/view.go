package plain

import (
	"runtime"
	"strconv"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/herdr"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// stripHeight is the one line of chrome: the tabs, the new tab button and
// the Herdr status, sharing the title bar.
const stripHeight = 40

// View draws the window: one sheet in the terminal's own background, a
// hairline under the strip, and nothing else.
func (w *Window) View(c *native.Context) {
	p := design.Colors(c.Theme().Dark)
	root := native.Column(c).Fill().Background(p.Content).TextColor(p.Text).FontSize(13).AlignItems(native.Stretch)
	root.HandleInput(func(ev native.InputEvent) bool {
		if ev.Kind != native.InputKeyDown {
			return false
		}
		if command, ok := Shortcut(runtime.GOOS, ev.Mods, ev.Key); ok {
			return w.Command(command)
		}
		return false
	})
	root.Children(func() {
		w.strip(c, p)
		native.Box(c).Height(1).Shrink(0).FillWidth().Background(p.Border)
		t := w.current()
		switch {
		case t == nil:
			w.empty(c, p)
		case t.err != "":
			w.notice(c, p, "The shell could not start", t.err)
		case t.term == nil:
			w.notice(c, p, "Starting shell…", "")
		default:
			terminal.View(c, t.term).Key("plain-terminal-"+strconv.Itoa(t.id)).Grow(1).FillWidth().AutoFocus()
		}
	})
}

func (w *Window) strip(c *native.Context, p design.Palette) {
	bar := c.TitleBar()
	native.Row(c.Key("plain-strip")).FillWidth().Height(stripHeight).Shrink(0).Padding(0, 8, 0, 8).Gap(4).AlignItems(native.Center).DragWindow().Children(func() {
		native.Box(c).Width(bar.Left).Shrink(0)
		native.Row(c.Key("plain-tabs")).Grow(1).MinWidth(0).Gap(2).AlignItems(native.Center).Clip().Children(func() {
			for i, t := range w.tabs {
				w.tabPill(c, p, i, t)
			}
			plus := native.ButtonBase(c.Key("plain-new-tab")).Size(24, 24).Shrink(0).Radius(12).Center().Label("New tab")
			hoverable(plus, p)
			plus.Children(func() { native.Icon(c, icons.Plus).Size(14, 14).TextColor(p.Secondary) })
			if plus.Clicked() {
				w.NewTab()
			}
			native.Box(c).Grow(1).FillHeight().DragWindow()
		})
		w.herdrChip(c, p)
		native.Box(c).Width(bar.Right).Shrink(0)
	})
}

func hoverable(e native.Element, p design.Palette) {
	if e.Pressed() {
		e.Background(p.Pressed)
	} else if e.Hovered() {
		e.Background(p.Hover)
	}
	if e.FocusVisible() {
		e.Border(2, p.Focus)
	}
}

// tabPill is a tab: its title, and its close button while hovered or on
// show. A single tab is a quiet title rather than a pill.
func (w *Window) tabPill(c *native.Context, p design.Palette, i int, t *tab) {
	selected := i == w.selected
	title := t.title(w.opts.Shell)
	pill := native.ButtonBase(c.Key("plain-tab:"+strconv.Itoa(t.id))).Height(26).Shrink(1).MinWidth(0).MaxWidth(220).Radius(8).Padding(0, 4, 0, 10).Label(title).Background(native.Color{})
	if selected && len(w.tabs) > 1 {
		pill.Background(p.Selected)
	} else if len(w.tabs) > 1 {
		hoverable(pill, p)
	}
	pill.Children(func() {
		native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
			text := native.Text(c, title).FontSize(12).SingleLine().MinWidth(0).Shrink(1).TextColor(p.Secondary)
			if selected {
				text.TextColor(p.Text).FontWeight(500)
			}
			close := native.ButtonBase(c.Key("plain-close:"+strconv.Itoa(t.id))).Size(18, 18).Shrink(0).Radius(9).Center().Label("Close " + title)
			hoverable(close, p)
			if !selected && !pill.Hovered() {
				close.Opacity(0)
			}
			close.Children(func() { native.Icon(c, icons.X).Size(10, 10).TextColor(p.Secondary) })
			if close.Clicked() {
				w.closeTab(t)
			}
		})
	})
	if pill.Clicked() {
		w.selected = i
	}
}

// herdrChip says whether Herdr is there: a quiet way back when it is, an
// invitation to install it when it is not. Its panel explains both.
func (w *Window) herdrChip(c *native.Context, p design.Palette) {
	install := herdr.Installation{}
	if w.opts.Installation != nil {
		install = w.opts.Installation()
	}
	label, color := "Herdr not installed", p.Tertiary
	if install.Installed() {
		label, color = "Herdr ready", p.Success
	} else if install.Path != "" {
		label, color = "Herdr not responding", p.Warning
	}
	chip := native.ButtonBase(c.Key("plain-herdr")).Height(24).Shrink(0).Radius(12).Padding(0, 10).Label(label)
	hoverable(chip, p)
	chip.Children(func() {
		native.Row(c).Gap(6).AlignItems(native.Center).Children(func() {
			native.Box(c).Size(6, 6).Radius(3).Background(color)
			native.Text(c, label).FontSize(11).TextColor(p.Secondary)
		})
	})
	if chip.Clicked() {
		w.herdrOpen = !w.herdrOpen
		w.message = ""
	}
	native.PopoverBase(c, chip, &w.herdrOpen, func(panel native.Element) {
		panel.Width(300).Radius(12).Padding(16).Gap(12).Background(p.Elevated).Border(1, p.Border).Shadow(0, 8, 24, 0, p.ContentShadow)
		panel.Children(func() {
			native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
				native.Icon(c, icons.Terminal).Size(16, 16).TextColor(p.Secondary)
				native.Text(c, "Plain terminal").FontSize(14).FontWeight(600)
			})
			switch {
			case install.Installed():
				native.Text(c, "Herdr "+install.Version+" is installed. Connect to manage workspaces, worktrees and agents in one window.").FontSize(12).TextColor(p.Secondary).FillWidth()
				if w.opts.ConnectHerdr != nil && native.Button(c, "Open Herdr Workspaces").FillWidth().Clicked() {
					if err := w.opts.ConnectHerdr(); err != nil {
						w.message = err.Error()
					} else {
						w.herdrOpen = false
					}
				}
			case install.Path != "":
				native.Text(c, "Herdr was found at "+install.Path+" but did not answer: "+install.Error).FontSize(12).TextColor(p.Secondary).FillWidth()
			default:
				native.Text(c, "Hergo works as an ordinary terminal. Install Herdr to add workspaces, Git worktrees and coding agent tracking.").FontSize(12).TextColor(p.Secondary).FillWidth()
				if w.opts.OpenURL != nil && native.Button(c, "Installation Guide").FillWidth().Clicked() {
					w.opts.OpenURL(herdr.InstallURL)
				}
			}
			if w.opts.Recheck != nil && native.Button(c, "Check Again").FillWidth().Clicked() {
				w.message = "Checking…"
				w.opts.Recheck()
			}
			if w.opts.AlwaysPlain != nil && w.opts.SetAlwaysPlain != nil {
				always := w.opts.AlwaysPlain()
				if native.Checkbox(c.Key("plain-always"), &always, "Always open as a plain terminal").Changed() {
					w.opts.SetAlwaysPlain(always)
				}
			}
			if w.message != "" {
				native.Text(c, w.message).FontSize(11).TextColor(p.Tertiary)
			}
		})
	})
}

func (w *Window) empty(c *native.Context, p design.Palette) {
	native.Column(c).Grow(1).FillWidth().Center().Gap(10).Children(func() {
		native.Icon(c, icons.Terminal).Size(28, 28).TextColor(p.Tertiary)
		native.Text(c, "No open shells").FontSize(15).FontWeight(600)
		hint := "Press ⌘T for a new tab."
		if runtime.GOOS != "darwin" {
			hint = "Press Ctrl+Shift+T for a new tab."
		}
		native.Text(c, hint).FontSize(12).TextColor(p.Secondary)
		if native.Button(c, "New Tab").Clicked() {
			w.NewTab()
		}
	})
}

func (w *Window) notice(c *native.Context, p design.Palette, title, detail string) {
	native.Column(c).Grow(1).FillWidth().Center().Gap(8).Padding(24).Children(func() {
		native.Text(c, title).FontSize(15).FontWeight(600)
		if detail != "" {
			native.Text(c, detail).FontSize(12).TextColor(p.Secondary)
		}
	})
}

// Recheck finished; the next frame shows the result.
func (w *Window) Rechecked() {
	if w.message == "Checking…" {
		w.message = ""
	}
}

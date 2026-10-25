package ui

import (
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
	"sort"
	"strings"
	"time"
)

// actionBar appears only for transient input modes or actionable errors.
func (w *Window) actionBar(c *native.Context, v app.View, p design.Palette) {
	a := w.controller
	if a == nil {
		return
	}
	if a.machine.Resize {
		native.Row(c).Padding(4, 16).Gap(6).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.Keyboard).Size(14, 14).TextColor(p.Warning)
			native.Text(c, "Resize: arrows / h j k l · Esc to finish").FontSize(12).TextColor(p.Warning)
		})
	}
	message := a.message
	if message == "" && len(a.machine.Bindings.Diagnostics) > 0 {
		message = strings.Join(a.machine.Bindings.Diagnostics, "; ")
	}
	if a.guardKnown && !a.guarded {
		message = "Keyboard shortcuts unavailable: input composition status cannot be read."
	}
	if v.ActionMessage != "" {
		message = v.ActionMessage
	}
	if a.bindingError != "" {
		message = "Keyboard bindings unavailable: " + a.bindingError
	}
	if message != "" {
		native.Text(c, message).TextColor(p.Warning).Padding(4, 16).FontSize(12)
	}
}

// tickNotifications takes Herdr's new notices into the queue and advances
// it, handing system notifications out as they become ready. It returns the
// in-app toast to show and how soon the queue needs another tick.
func (w *Window) tickNotifications(now time.Time) (app.Toast, bool, time.Duration) {
	if w.hooks.Notices != nil {
		for _, n := range w.hooks.Notices() {
			w.toasts.Receive(n)
		}
	}
	settings := w.settings.snapshot.Toast
	toast, ok, system, wake := w.toasts.Tick(now, app.ToastContext{
		Snapshot:      w.app.Publication().Snapshot,
		SelectedTabID: w.currentView().TabID,
		Active:        w.appearance.active,
		Delivery:      settings.Delivery,
		Delay:         time.Duration(settings.DelaySeconds) * time.Second,
	})
	if w.hooks.PostNotification != nil {
		for _, n := range system {
			w.hooks.PostNotification(n)
		}
	}
	return toast, ok, wake
}

// PumpNotifications advances notifications outside a frame, so system
// notifications go out while the window is hidden and draws nothing. It
// returns how soon to pump again, zero when nothing is waiting.
func (w *Window) PumpNotifications() time.Duration {
	_, _, wake := w.tickNotifications(time.Now())
	return wake
}

// notificationToast floats above the workspace without taking terminal focus
// or contributing to its layout. Modal dialogs are rendered above it.
func (w *Window) notificationToast(c *native.Context, p design.Palette) {
	if w.controller == nil {
		return
	}
	toast, ok, wake := w.tickNotifications(c.Now())
	if wake > 0 {
		c.After(wake)
	}
	if !ok {
		return
	}
	dot := p.Accent
	if toast.Kind == "NeedsAttention" {
		dot = p.Danger
	}
	position := toast.Position
	if position == "" {
		position = w.settings.snapshot.Toast.Position
	}
	width, _ := c.Size()
	native.Overlay(c.Key("notification-overlay"), func() {
		card := native.Column(c.Key("notification-toast")).Label("Notification toast").Absolute().Width(min(360, max(0, width-32))).Padding(12, 12, 12, 16).Gap(10).Radius(w.shape.Panel).Background(p.Elevated).Border(1, p.Border).Shadow(0, 6, 24, 0, p.ContentShadow).TextColor(p.Text).FontSize(13)
		switch position {
		case "top-left":
			card.Top(56).Left(16)
		case "top-right":
			card.Top(56).Right(16)
		case "bottom-left":
			card.Bottom(16).Left(16)
		default:
			card.Bottom(16).Right(16)
		}
		card.Children(func() {
			native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
				native.Box(c).Size(7, 7).Shrink(0).Radius(3.5).Background(dot)
				native.Text(c, toast.Title).Bold().Grow(1).MinWidth(0)
				if w.iconButton(c, "dismiss-notification", icons.X, "Dismiss notification", p).KeepFocus().Clicked() {
					w.toasts.Dismiss()
				}
			})
			if toast.Body != "" {
				native.Text(c, toast.Body).FontSize(12).TextColor(p.Secondary).FillWidth().PaddingX(17)
			}
			if toast.PaneID != "" || toast.TabID != "" || toast.WorkspaceID != "" {
				native.Row(c).FillWidth().Children(func() {
					native.Box(c).Grow(1)
					if native.Button(c, "Open").Label("Open notification").KeepFocus().Clicked() {
						w.Dispatch("open_notification_target")
					}
				})
			}
		})
	})
}

// OpenNotice selects the most specific place a notice names, as opening
// its toast or system notification does.
func (w *Window) OpenNotice(n app.Notice) {
	switch {
	case n.PaneID != "":
		w.selectShortcutPane(n.PaneID)
	case n.TabID != "":
		w.app.SelectTab(n.TabID)
	case n.WorkspaceID != "":
		w.app.SelectWorkspace(n.WorkspaceID)
	}
}

// referencePanel lists every command with its keys, grouped as the menu
// bar groups them, one line each: the command, its ⌘ key, its prefix chord.
func (w *Window) referencePanel(c *native.Context, p design.Palette) {
	a := w.controller
	open := true
	width, height := c.Size()
	native.DialogBase(c.Key("keyboard-reference-dialog"), &open, func(backdrop, panel native.Element) {
		backdrop.Background(native.RGBA(0, 0, 0, .32))
		panel.Label("Keyboard reference panel").Width(min(560, max(0, width-48))).Height(min(680, max(0, height-48))).Padding(20, 20, 0, 20).Gap(12).Background(p.Elevated).Border(1, p.Border).Radius(w.shape.Dialog).Shadow(0, 16, 48, 0, p.ContentShadow).AlignItems(native.Stretch)

		native.Row(c).FillWidth().Gap(10).AlignItems(native.Start).Children(func() {
			native.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
				native.Text(c, "Keyboard reference").FontSize(15).FontWeight(600)
				prefix := keyGlyphs(a.machine.Bindings.Prefix)
				native.Text(c, "Press "+prefix+", release it, then the key. ⌘ keys work on their own.").FontSize(12).TextColor(p.Secondary).FillWidth()
			})
			close := w.iconButton(c, "close-reference", icons.X, "Close", p)
			if w.helpFocusPending {
				close.Focus()
				w.helpFocusPending = false
			}
			if close.Clicked() {
				w.Dispatch("help")
			}
		})
		if a.bindingError != "" {
			native.Text(c, a.bindingError).FontSize(12).TextColor(p.Warning).FillWidth()
		}
		if a.guardKnown && !a.guarded {
			native.Text(c, "Keyboard shortcuts are unavailable while input composition cannot be observed. Layout actions remain available from the menu.").FontSize(12).TextColor(p.Warning).FillWidth()
		}
		native.Box(c).FillWidth().Height(1).Shrink(0).Background(p.Border)
		native.Scroll(c.Key("shortcut-reference-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			native.Column(c).FillWidth().Gap(16).Padding(0, 0, 20, 0).AlignItems(native.Stretch).Children(func() {
				for _, group := range referenceGroups() {
					w.referenceGroup(c, p, group.title, func() {
						for _, id := range group.actions {
							referenceRow(c, p, "shortcut:"+id, w.ActionLabel(id), directKeyLabel(id), keyGlyphs(w.BindingLabel(id)))
						}
					})
				}
				commands := a.machine.Bindings.Commands
				if len(commands) > 0 {
					w.referenceGroup(c, p, "Commands", func() {
						for i, command := range commands {
							chord := keyGlyphs(w.BindingLabel(fmt.Sprintf("command[%d]", i)))
							if chord == "" {
								continue
							}
							description := command.Description
							if description == "" {
								description = command.Command
							}
							referenceRow(c, p, fmt.Sprintf("command:%d", i), description, "", chord)
						}
					})
				}
				keys := make([]string, 0, len(a.machine.Bindings.Navigation))
				for key := range a.machine.Bindings.Navigation {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				w.referenceGroup(c, p, "Modes", func() {
					for _, key := range keys {
						referenceRow(c, p, "navigate:"+key, "Navigate "+strings.ReplaceAll(strings.TrimPrefix(a.machine.Bindings.Navigation[key], "navigate_"), "_", " "), "", keyGlyphs(key))
					}
					native.Text(c, "Resize mode: the arrows or h j k l resize, Esc or Return finishes. Press the prefix twice to send it to the program.").FontSize(12).TextColor(p.Tertiary).FillWidth().Padding(6, 0, 0, 0)
				})
				native.Text(c, "Loaded from "+a.machine.Bindings.Source).FontSize(11).TextColor(p.Tertiary).FillWidth().Selectable()
			})
		})
	})
	if !open && w.referenceOpen() {
		w.Dispatch("help")
		w.BlurInput()
	}
}

// referenceGroup titles one group of the reference.
func (w *Window) referenceGroup(c *native.Context, p design.Palette, title string, rows func()) {
	native.Column(c.Key("reference-group:" + title)).FillWidth().AlignItems(native.Stretch).Children(func() {
		native.Text(c, title).FontSize(12).FontWeight(600).TextColor(p.Secondary).Padding(0, 0, 4, 0)
		rows()
	})
}

// referenceRow is one command and its keys: the ⌘ key in the text's color,
// the prefix chord quieter, a dash when it has neither.
func referenceRow(c *native.Context, p design.Palette, key, label, direct, chord string) {
	native.Row(c.Key(key)).FillWidth().Height(28).Gap(12).AlignItems(native.Center).Children(func() {
		native.Text(c, label).FontSize(13).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
		if direct != "" {
			native.Text(c, direct).FontSize(12).Shrink(0)
		}
		if chord != "" {
			native.Text(c, chord).FontSize(12).TextColor(p.Secondary).Shrink(0)
		}
		if direct == "" && chord == "" {
			native.Text(c, "—").FontSize(12).TextColor(p.Tertiary).Shrink(0)
		}
	})
}

// referenceGroups are the menu bar's sections, then the commands no menu
// lists, as General.
func referenceGroups() []struct {
	title   string
	actions []string
} {
	listed := map[string]bool{}
	var groups []struct {
		title   string
		actions []string
	}
	for _, section := range MenuBar {
		var actions []string
		for _, group := range section.Groups {
			for _, id := range group {
				listed[id] = true
				actions = append(actions, id)
			}
		}
		groups = append(groups, struct {
			title   string
			actions []string
		}{section.Title, actions})
	}
	var general []string
	for _, action := range Actions {
		if !listed[action.ID] {
			general = append(general, action.ID)
		}
	}
	return append([]struct {
		title   string
		actions []string
	}{{"General", general}}, groups...)
}

// copyShown is how long the copy confirmation stays.
const copyShown = 1200 * time.Millisecond

// ShowCopied confirms a copy from a terminal, as Ghostty's clipboard-copy
// notification does; it runs on the UI thread.
func (w *Window) ShowCopied() { w.copied = true }

// copyConfirmation floats a small "Copied" pill at the foot of the window,
// passing the pointer through to the terminal under it.
func (w *Window) copyConfirmation(c *native.Context, p design.Palette) {
	if w.copied {
		w.copied, w.copiedAt = false, c.Now()
	}
	since := c.Now().Sub(w.copiedAt)
	if w.copiedAt.IsZero() || since >= copyShown {
		return
	}
	c.After(copyShown - since)
	native.Overlay(c.Key("copy-overlay"), func() {
		native.Row(c.Key("copy-confirmation-row")).Absolute().Left(0).Right(0).Bottom(24).Justify(native.Center).PassThrough().Children(func() {
			native.Row(c.Key("copy-confirmation")).Label("Copied to clipboard").Padding(6, 12).Gap(6).AlignItems(native.Center).Radius(14).Background(p.Elevated).Border(1, p.Border).Shadow(0, 4, 16, 0, p.ContentShadow).PassThrough().Children(func() {
				native.Icon(c, icons.Check).Size(14, 14).TextColor(p.Success)
				native.Text(c, "Copied").FontSize(12).TextColor(p.Text)
			})
		})
	})
}

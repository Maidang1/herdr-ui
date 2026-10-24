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

func (w *Window) referencePanel(c *native.Context, p design.Palette) {
	a := w.controller
	open := true
	width, height := c.Size()
	native.DialogBase(c.Key("keyboard-reference-dialog"), &open, func(backdrop, panel native.Element) {
		backdrop.Background(native.RGBA(0, 0, 0, .28))
		panel.Label("Keyboard reference panel").Width(min(560, max(0, width-48))).Height(min(680, max(0, height-48))).Padding(20).Gap(12).Background(p.Elevated).Border(1, p.Border).Radius(w.shape.Dialog).Shadow(0, 12, 36, 0, p.ContentShadow)

		native.Row(c).FillWidth().Height(28).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.Keyboard).Size(16, 16).TextColor(p.Secondary)
			native.Text(c, "Keyboard reference").Bold().Grow(1).PaddingX(8)
			close := w.iconButton(c, "close-reference", icons.X, "Close", p)
			if w.helpFocusPending {
				close.Focus()
				w.helpFocusPending = false
			}
			if close.Clicked() {
				w.Dispatch("help")
			}
		})
		native.Text(c, "Bindings loaded by this client").FontSize(12).TextColor(p.Secondary)
		native.Text(c, a.machine.Bindings.Source).FontSize(12).TextColor(p.Secondary).Selectable()
		if a.bindingError != "" {
			native.Text(c, a.bindingError).FontSize(12).TextColor(p.Warning)
		}
		if a.guardKnown && !a.guarded {
			native.Text(c, "Keyboard shortcuts are unavailable while input composition cannot be observed. Layout actions remain available from the menu.").FontSize(12).TextColor(p.Warning)
		}
		native.Scroll(c.Key("shortcut-reference-scroll")).Grow(1).MinHeight(0).FillWidth().Children(func() {
			native.Column(c).FillWidth().Gap(8).Children(func() {
				for _, action := range Actions {
					label := keyGlyphs(w.BindingLabel(action.ID))
					if label == "" {
						label = "Not assigned"
					}
					native.Column(c.Key("shortcut:" + action.ID)).FillWidth().Gap(2).Children(func() {
						native.Text(c, w.ActionLabel(action.ID)).FontSize(13)
						native.Text(c, label).FontSize(12).TextColor(p.Secondary)
					})
				}
				for i, command := range a.machine.Bindings.Commands {
					label := keyGlyphs(w.BindingLabel(fmt.Sprintf("command[%d]", i)))
					if label == "" {
						continue
					}
					description := command.Description
					if description == "" {
						description = command.Command
					}
					native.Text(c, description).FontSize(13)
					native.Text(c, label).FontSize(12).TextColor(p.Secondary)
				}
				keys := make([]string, 0, len(a.machine.Bindings.Navigation))
				for key := range a.machine.Bindings.Navigation {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					native.Text(c, "Navigate: "+key+" → "+strings.TrimPrefix(a.machine.Bindings.Navigation[key], "navigate_")).FontSize(12).TextColor(p.Secondary)
				}
				native.Text(c, "Press the prefix, release it, then press an action key. Press the prefix twice to send it literally.").FontSize(12).TextColor(p.Secondary)
				native.Text(c, "Resize mode: arrows or h j k l. Esc or Enter finishes resizing.").FontSize(12).TextColor(p.Secondary)
			})
		})
	})
	if !open && w.referenceOpen() {
		w.Dispatch("help")
		w.BlurInput()
	}
}

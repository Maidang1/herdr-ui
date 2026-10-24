package ui

import (
	"fmt"
	"github.com/Maidang1/herdr-ui/internal/app"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	native "github.com/egoist/mygo/ui"
	"sort"
	"strings"
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

// notificationToast floats above the workspace without taking terminal focus
// or contributing to its layout. Modal dialogs are rendered above it.
func (w *Window) notificationToast(c *native.Context, p design.Palette) {
	if w.controller == nil || w.hooks.NotificationText == nil {
		return
	}
	title, remaining := w.hooks.NotificationText()
	if title == "" {
		return
	}
	if remaining > 0 {
		c.After(remaining)
	}
	width, _ := c.Size()
	native.Overlay(c.Key("notification-overlay"), func() {
		native.Column(c.Key("notification-toast")).Label("Notification toast").Absolute().Top(56).Right(16).Width(min(360, max(0, width-32))).Padding(16).Gap(12).Radius(w.shape.Panel).Background(p.Elevated).Border(1, p.Border).Shadow(0, 6, 24, 0, p.ContentShadow).TextColor(p.Text).FontSize(13).Children(func() {
			native.Text(c, title).FillWidth()
			native.Row(c).FillWidth().Children(func() {
				native.Box(c).Grow(1)
				if native.Button(c, "Open notification").KeepFocus().Clicked() {
					w.Dispatch("open_notification_target")
				}
			})
		})
	})
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

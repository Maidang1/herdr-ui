package ui

import (
	"fmt"
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	"github.com/bytedance/herdr-ui/internal/icons"
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
	if w.hooks.NotificationText != nil {
		if title, remaining := w.hooks.NotificationText(); title != "" {
			native.Row(c).Padding(8, 16).Gap(8).Children(func() {
				native.Text(c, title).Grow(1)
				if native.Button(c, "Open notification").Clicked() {
					w.Dispatch("open_notification_target")
				}
			})
			if remaining > 0 {
				c.After(remaining)
			}
		}
	}
	if a.machine.Awaiting || a.machine.Resize {
		hint := "Prefix"
		if a.machine.Resize {
			hint = "Resize: arrows / h j k l · Esc to finish"
		}
		native.Row(c).Padding(4, 16).Gap(6).AlignItems(native.Center).Children(func() {
			native.Icon(c, icons.Keyboard).Size(14, 14).TextColor(p.Warning)
			native.Text(c, hint).FontSize(12).TextColor(p.Warning)
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
func (w *Window) referencePanel(c *native.Context, p design.Palette) {
	a := w.controller
	panel := native.Column(c.Key("keyboard-reference")).Label("Keyboard reference panel").Width(referenceWidth).Shrink(0).FillHeight().Padding(16).Gap(12).Background(p.Elevated).Border(1, p.Border).Radius(16)
	if panel.FocusWithin() && panel.Shortcut(0, native.KeyEscape) {
		w.Dispatch("help")
	}
	panel.Children(func() {
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
					label := w.BindingLabel(action.ID)
					if label == "" {
						label = "Not assigned"
					}
					native.Column(c.Key("shortcut:" + action.ID)).FillWidth().Gap(2).Children(func() {
						native.Text(c, w.ActionLabel(action.ID)).FontSize(13)
						native.Text(c, label).FontSize(12).TextColor(p.Secondary)
					})
				}
				for i, command := range a.machine.Bindings.Commands {
					label := w.BindingLabel(fmt.Sprintf("command[%d]", i))
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
}

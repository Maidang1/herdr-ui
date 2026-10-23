package ui

import (
	"github.com/bytedance/herdr-ui/internal/app"
	"github.com/bytedance/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

func (w *Window) actionBar(c *native.Context, v app.View, p design.Palette) {
	a := w.controller
	if a == nil {
		return
	}
	native.Row(c).Padding(4, 12).Gap(8).AlignItems(native.Center).Children(func() {
		for _, action := range []Action{Actions[0], Actions[1], Actions[8]} {
			if native.Button(c, action.Label).Disabled(!w.Can(action.ID)).Tooltip(w.BindingLabel(action.ID)).Clicked() {
				w.Dispatch(action.ID)
			}
		}
		native.Button(c, "Layout").Menu(func(m *native.Menu) {
			for _, action := range Actions {
				label := action.Label
				if binding := w.BindingLabel(action.ID); binding != "" {
					label += " · " + binding
				}
				if m.Item(label).Disabled(!w.Can(action.ID)).Chosen() {
					w.Dispatch(action.ID)
				}
			}
		})
		native.Box(c).Grow(1)
		if a.machine.Resize {
			native.Text(c, "Resize: arrows / h j k l · Esc to finish").TextColor(p.Secondary)
		} else if a.machine.Awaiting {
			native.Text(c, "Prefix · Esc to cancel").TextColor(p.Secondary)
		}
		if native.Button(c, "?").Label("Keyboard reference").Clicked() {
			w.Dispatch("help")
		}
	})
	message := a.message
	if a.guardKnown && !a.guarded {
		message = "Business shortcuts unavailable: rebuild with make build for composition protection."
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
	if a.help {
		panel := native.Column(c.Key("keyboard-reference")).Height(210).Shrink(0).FillWidth().Padding(12, 16).Gap(4).Background(p.Inset)
		if panel.Shortcut(0, native.KeyEscape) {
			w.Dispatch("help")
		}
		panel.Children(func() {
			native.Row(c).FillWidth().Children(func() {
				native.Text(c, "Keyboard reference").Bold().Grow(1)
				if native.Button(c, "Close").Clicked() {
					a.help = false
					if w.terminal != nil {
						w.terminal.focused = ""
					}
				}
			})
			native.Text(c, "Bindings: "+a.machine.Bindings.Source).FontSize(12).TextColor(p.Secondary)
			if !a.guarded {
				native.Text(c, "Business shortcuts require the composition-aware build (make build).").FontSize(12).TextColor(p.Warning)
			}
			native.Scroll(c).Grow(1).Children(func() {
				native.Column(c).Gap(3).Children(func() {
					for _, action := range Actions {
						native.Text(c, action.Label+"   "+w.BindingLabel(action.ID)).FontSize(12)
					}
					native.Text(c, "Press prefix twice to send it literally. Unsupported actions are reserved for future milestones.").FontSize(12).TextColor(p.Secondary)
				})
			})
		})
	}
}

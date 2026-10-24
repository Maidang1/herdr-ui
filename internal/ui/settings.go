package ui

import (
	"strconv"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/Maidang1/herdr-ui/internal/icons"
	"github.com/egoist/mygo/plugins/terminal"
	native "github.com/egoist/mygo/ui"
)

// SettingsSnapshot is the settings' value object crossing from the
// application layer; the window keeps its own editable copy.
type SettingsSnapshot struct {
	Theme           string
	Font            terminal.Font
	CopyOnSelect    bool
	MinimumContrast float32
	Notifications   bool
}

// settingsUI is the settings window's state: the page on show and the
// working copy of every value, with the float64 mirrors the sliders bind
// to.
type settingsUI struct {
	page     int
	snapshot SettingsSnapshot
	// sizeDefault is the size Actual Size and the settings' reset return
	// to: what the persisted file held at launch.
	sizeDefault float32
	size        float64
	lineHeight  float64
	contrast    float64
	themeIndex  int
	weightIndex int
}

var settingsPages = []string{"Appearance", "Terminal", "Notifications", "Shortcuts"}

var settingsPageIcons = []*native.SVG{icons.Circle, icons.Terminal, icons.MessageCircle, icons.Keyboard}

// install seeds the working copy from the persisted values.
func (s *settingsUI) install(snapshot SettingsSnapshot) {
	if snapshot.Font.Size <= 0 {
		snapshot.Font.Size = 13
	}
	if snapshot.Font.Weight <= 0 {
		snapshot.Font.Weight = 400
	}
	if snapshot.Font.LineHeight <= 0 {
		snapshot.Font.LineHeight = 1
	}
	if snapshot.MinimumContrast < 1 {
		snapshot.MinimumContrast = 1
	}
	s.snapshot = snapshot
	s.sizeDefault = snapshot.Font.Size
	s.resetMirrors()
}

func (s *settingsUI) resetMirrors() {
	s.size = float64(s.snapshot.Font.Size)
	s.lineHeight = float64(s.snapshot.Font.LineHeight)
	s.contrast = float64(s.snapshot.MinimumContrast)
	s.themeIndex = themeIndexOf(s.snapshot.Theme)
	s.weightIndex = weightIndexOf(s.snapshot.Font.Weight)
}

func themeIndexOf(theme string) int {
	switch theme {
	case "light":
		return 1
	case "dark":
		return 2
	}
	return 0
}

func themeFromIndex(i int) string { return []string{"system", "light", "dark"}[max(0, min(2, i))] }

var fontWeights = []int{300, 400, 500, 600}

func weightIndexOf(weight int) int {
	for i, candidate := range fontWeights {
		if candidate >= weight {
			return i
		}
	}
	return len(fontWeights) - 1
}

// SettingsView renders the settings window: a page list on the left, the
// page's cards on the right, in the System Settings shape Kero and Herdr
// use. Every change applies immediately through the hooks, which persist.
func (w *Window) SettingsView(c *native.Context) {
	p := w.appearancePalette(c)
	s := &w.settings
	s.resetMirrors()
	root := native.Row(c.Key("settings-root")).Fill().Gap(0).Background(p.Shell).TextColor(p.Text).FontSize(13)
	root.Children(func() {
		native.Column(c.Key("settings-nav")).Width(196).Shrink(0).FillHeight().Padding(12, 8).Gap(2).Background(p.Sidebar).Children(func() {
			native.Text(c, "Settings").FontSize(13).FontWeight(700).Padding(6, 8)
			for i, name := range settingsPages {
				page := i
				button := native.ButtonBase(c.Key("settings-page:"+name)).FillWidth().Radius(6).Padding(6, 8).Label(name)
				if s.page == page {
					button.Background(p.Selected)
				} else {
					w.controlState(button, p)
				}
				button.Children(func() {
					native.Row(c).Gap(8).AlignItems(native.Center).Children(func() {
						native.Icon(c, settingsPageIcons[page]).Size(15, 15).TextColor(p.Secondary)
						text := native.Text(c, name).FontSize(13)
						if s.page == page {
							text.FontWeight(600)
						}
					})
				})
				if button.Clicked() {
					s.page = page
				}
			}
		})
		native.Scroll(c.Key("settings-content")).Grow(1).MinWidth(0).Children(func() {
			native.Column(c).Padding(20, 24).Gap(14).FillWidth().Children(func() {
				native.Text(c, settingsPages[s.page]).FontSize(16).FontWeight(700)
				switch s.page {
				case 0:
					w.settingsAppearance(c, p, s)
				case 1:
					w.settingsTerminal(c, p, s)
				case 2:
					w.settingsNotifications(c, p, s)
				default:
					w.settingsShortcuts(c, p)
				}
			})
		})
	})
}

func (w *Window) settingsCard(c *native.Context, key, title string, p design.Palette, body func()) {
	native.Column(c.Key(key)).FillWidth().Gap(8).Padding(14).Radius(10).Background(p.Inset).Border(1, p.Border).Children(func() {
		native.Text(c, title).FontSize(13).FontWeight(600)
		body()
	})
}

func (w *Window) settingsAppearance(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsCard(c, "settings-theme", "Theme", p, func() {
		native.Text(c, "Light or dark chrome, or follow the system.").FontSize(12).TextColor(p.Secondary)
		segment := native.Segmented(c.Key("settings-theme-choice"), &s.themeIndex, "System", "Light", "Dark")
		if segment.Changed() {
			s.snapshot.Theme = themeFromIndex(s.themeIndex)
			if w.hooks.Theme != nil {
				w.hooks.Theme(s.snapshot.Theme)
			}
		}
	})
}

func (w *Window) settingsTerminal(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsCard(c, "settings-font", "Font", p, func() {
		native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
			native.Text(c, "Size").Width(110).TextColor(p.Secondary)
			slider := native.Slider(c.Key("settings-font-size"), &s.size, 8, 48)
			native.Text(c, strconv.FormatFloat(s.size, 'f', -1, 64)).Width(28).TextColor(p.Secondary)
			if slider.Changed() {
				s.snapshot.Font.Size = clampFontSize(float32(s.size))
				w.pushFont()
			}
		})
		native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
			native.Text(c, "Weight").Width(110).TextColor(p.Secondary)
			segment := native.Segmented(c.Key("settings-font-weight"), &s.weightIndex, "Light", "Regular", "Medium", "Semibold")
			if segment.Changed() {
				s.snapshot.Font.Weight = fontWeights[max(0, min(len(fontWeights)-1, s.weightIndex))]
				w.pushFont()
			}
		})
		native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
			native.Text(c, "Line height").Width(110).TextColor(p.Secondary)
			slider := native.Slider(c.Key("settings-line-height"), &s.lineHeight, 1, 2)
			native.Text(c, strconv.FormatFloat(s.lineHeight, 'f', 2, 64)).Width(28).TextColor(p.Secondary)
			if slider.Changed() {
				s.snapshot.Font.LineHeight = max(1, min(2, float32(s.lineHeight)))
				w.pushFont()
			}
		})
	})
	w.settingsCard(c, "settings-preview", "Preview", p, func() {
		// A monospace sample at the chosen size and weight; the real
		// terminals apply the same values as they render.
		native.Text(c, "herdr pane run w1:p1 \"just test\"").Font("monospace").FontSize(s.snapshot.Font.Size)
		native.Text(c, "← preview at "+strconv.FormatFloat(float64(s.snapshot.Font.Size), 'f', -1, 64)+"pt").FontSize(11).TextColor(p.Tertiary)
	})
	w.settingsCard(c, "settings-selection", "Selection", p, func() {
		box := native.Checkbox(c.Key("settings-copy-on-select"), &s.snapshot.CopyOnSelect, "Copy on select")
		if box.Changed() && w.hooks.CopyOnSelect != nil {
			w.hooks.CopyOnSelect(s.snapshot.CopyOnSelect)
		}
	})
	w.settingsCard(c, "settings-contrast", "Minimum contrast", p, func() {
		native.Text(c, "Keeps text readable over program colors; 1 leaves colors untouched.").FontSize(12).TextColor(p.Secondary)
		native.Row(c).FillWidth().Gap(10).AlignItems(native.Center).Children(func() {
			slider := native.Slider(c.Key("settings-contrast-slider"), &s.contrast, 1, 7)
			native.Text(c, strconv.FormatFloat(s.contrast, 'f', 1, 64)).Width(28).TextColor(p.Secondary)
			if slider.Changed() {
				s.snapshot.MinimumContrast = max(1, min(7, float32(s.contrast)))
				if w.hooks.MinimumContrast != nil {
					w.hooks.MinimumContrast(s.snapshot.MinimumContrast)
				}
			}
		})
	})
}

func (w *Window) settingsNotifications(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsCard(c, "settings-notifications", "Agent status", p, func() {
		box := native.Checkbox(c.Key("settings-notifications-toggle"), &s.snapshot.Notifications, "Notify when an agent needs you or finishes")
		if box.Changed() && w.hooks.Notifications != nil {
			w.hooks.Notifications(s.snapshot.Notifications)
		}
		native.Text(c, "Only while this window is not focused; the Dock badge follows the same switch.").FontSize(12).TextColor(p.Secondary)
	})
}

func (w *Window) settingsShortcuts(c *native.Context, p design.Palette) {
	a := w.controller
	w.settingsCard(c, "settings-keybindings", "Keybindings", p, func() {
		native.Text(c, "Prefix chords load from Herdr's own configuration; direct window keys are fixed.").FontSize(12).TextColor(p.Secondary)
		if a != nil {
			native.Text(c, a.machine.Bindings.Source).FontSize(11).TextColor(p.Tertiary).Selectable()
		}
		if button := native.Button(c, "Edit Herdr configuration"); button.Clicked() && w.hooks.EditConfig != nil {
			w.shortcutError(w.hooks.EditConfig())
		}
	})
	w.settingsCard(c, "settings-actions", "Actions", p, func() {
		for _, action := range Actions {
			label := w.BindingLabel(action.ID)
			if label == "" {
				label = "—"
			}
			native.Row(c.Key("settings-action:" + action.ID)).FillWidth().Children(func() {
				native.Text(c, w.ActionLabel(action.ID)).FontSize(12).Grow(1).SingleLine()
				native.Text(c, label).FontSize(12).TextColor(p.Secondary)
			})
		}
	})
}

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
	Toast           ToastSettings
}

// ToastSettings is how agent notifications reach the user, in the terms of
// Herdr's [ui.toast]: Delivery is "off", "herdr" (in-app) or "system".
type ToastSettings struct {
	Delivery, Position string
	DelaySeconds       int
}

var (
	toastDeliveries     = []string{"off", "herdr", "system"}
	toastDeliveryLabels = []string{"Off", "In-app", "System"}
	toastPositions      = []string{"top-left", "top-right", "bottom-left", "bottom-right"}
	toastPositionLabels = []string{"Top left", "Top right", "Bottom left", "Bottom right"}
	toastMaxSliderDelay = 10.0
)

func indexOf(values []string, value string, fallback int) int {
	for i, v := range values {
		if v == value {
			return i
		}
	}
	return fallback
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
	delivery    int
	position    int
	delay       float64
}

var settingsPages = []string{"Appearance", "Terminal", "Notifications", "Shortcuts"}

var settingsPageIcons = []*native.SVG{icons.Palette, icons.Terminal, icons.MessageCircle, icons.Keyboard}

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
	s.delivery = indexOf(toastDeliveries, s.snapshot.Toast.Delivery, 0)
	s.position = indexOf(toastPositions, s.snapshot.Toast.Position, 3)
	s.delay = float64(s.snapshot.Toast.DelaySeconds)
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

// The settings window keeps one rhythm, as System Settings does: a fixed
// label column, every control starting at the same line, sliders of one
// length with their value beside them, and captions under the rows they
// explain — never mixed into them.
const (
	settingsLabelWidth  = 96
	settingsSliderWidth = 172
	settingsValueWidth  = 52
	settingsRowHeight   = 28
)

// SettingsView renders the settings window: a page list on the left, the
// page's cards on the right, in the System Settings shape Kero and Herdr
// use. Every change applies immediately through the hooks, which persist.
func (w *Window) SettingsView(c *native.Context) {
	p := w.appearancePalette(c)
	s := &w.settings
	s.resetMirrors()
	root := native.Row(c.Key("settings-root")).Fill().AlignItems(native.Stretch).Background(p.Inset).TextColor(p.Text).FontSize(13)
	root.Children(func() {
		native.Column(c.Key("settings-nav")).Width(180).Shrink(0).FillHeight().Padding(18, 10).Gap(4).Background(p.Sidebar).Children(func() {
			native.Text(c, "Settings").FontSize(11).FontWeight(700).TextColor(p.Tertiary).Padding(0, 10, 10, 10)
			for i, name := range settingsPages {
				page := i
				button := native.ButtonBase(c.Key("settings-page:"+name)).FillWidth().Height(34).Justify(native.Start).Radius(7).Padding(0, 10).Label(name).Selected(s.page == page)
				if s.page == page {
					button.Background(p.Accent.Alpha(.12))
				} else {
					w.controlState(button, p)
				}
				button.Children(func() {
					native.Row(c).FillWidth().Gap(9).AlignItems(native.Center).Children(func() {
						native.Icon(c, settingsPageIcons[page]).Size(15, 15).TextColor(p.Secondary)
						text := native.Text(c, name).FontSize(13).TextColor(p.Text)
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
		native.Box(c).Width(1).FillHeight().Shrink(0).Background(p.Border)
		native.Scroll(c.Key("settings-content")).Grow(1).FillHeight().MinWidth(0).AlignItems(native.Stretch).Children(func() {
			native.Column(c).Padding(24).Gap(16).FillWidth().MaxWidth(680).Shrink(0).AlignItems(native.Stretch).Children(func() {
				native.Column(c).Gap(6).Children(func() {
					native.Text(c, settingsPages[s.page]).FontSize(22).FontWeight(700)
					descriptions := []string{"Make Herdr feel at home.", "Fine-tune your terminal experience.", "Choose when Herdr gets your attention.", "Keep your workflow close at hand."}
					native.Text(c, descriptions[s.page]).FontSize(12).TextColor(p.Secondary)
				})
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

// settingsCard is one grouped card: a semibold caption, then rows on a
// single rhythm.
func (w *Window) settingsCard(c *native.Context, key, title string, p design.Palette, body func()) {
	native.Column(c.Key(key)).FillWidth().Shrink(0).Gap(4).Padding(12).Radius(10).Background(p.Elevated).Border(1, p.Border).Children(func() {
		native.Text(c, title).FontSize(13).FontWeight(600).PaddingY(4)
		body()
	})
}

// settingsRow puts one control on the shared grid: label column, then the
// control starting where every control starts.
func settingsRow(c *native.Context, label string, control func()) {
	native.Row(c).FillWidth().Height(settingsRowHeight).AlignItems(native.Center).Gap(10).Children(func() {
		native.Text(c, label).Width(settingsLabelWidth).Shrink(0).FontSize(12).TextColor(c.Theme().TextMuted)
		control()
	})
}

// settingsCaption explains the row above it, in the smaller muted voice
// every caption shares.
func settingsCaption(c *native.Context, p design.Palette, text string) {
	native.Text(c, text).FontSize(11).TextColor(p.Tertiary).FillWidth().Padding(2, 0, 4, 0)
}

// settingsSlider lays a slider and its value with unit on the shared grid.
func settingsSlider(c *native.Context, key string, value *float64, lo, hi float64, format func(float64) string) native.Element {
	slider := native.Slider(c.Key(key), value, lo, hi).Width(settingsSliderWidth).Shrink(0)
	native.Text(c, format(*value)).Width(settingsValueWidth).Shrink(0).FontSize(12).TextColor(c.Theme().TextMuted)
	return slider
}

func (w *Window) settingsAppearance(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsCard(c, "settings-theme", "Theme", p, func() {
		native.Row(c).FillWidth().Gap(12).Children(func() {
			for i, name := range []string{"System", "Light", "Dark"} {
				selected := s.themeIndex == i
				button := native.ButtonBase(c.Key("settings-theme:" + name)).Grow(1).Basis(0).MinWidth(0).Padding(6).Radius(8).Label(name).Selected(selected)
				button.Border(1, p.Border)
				w.controlState(button, p)
				if selected {
					button.Border(2, p.Accent).Background(p.Accent.Alpha(.06))
				}
				button.Children(func() {
					native.Column(c).FillWidth().Gap(8).Children(func() {
						settingsThemePreview(c, i, p)
						native.Text(c, name).FontSize(12).FontWeight(600).TextColor(p.Text)
					})
				})
				if button.Clicked() {
					s.themeIndex = i
					s.snapshot.Theme = themeFromIndex(i)
					if w.hooks.Theme != nil {
						w.hooks.Theme(s.snapshot.Theme)
					}
				}
			}
		})
		settingsCaption(c, p, "System follows your Mac’s light or dark appearance.")
	})
}

// Miniature windows make the appearance options recognizable before selection.
func settingsThemePreview(c *native.Context, mode int, p design.Palette) {
	light, dark := design.Colors(false), design.Colors(true)
	base := light
	if mode == 2 {
		base = dark
	}
	native.Column(c).FillWidth().Height(72).Radius(5).Clip().Background(base.Content).Border(1, p.Border).Children(func() {
		native.Row(c).FillWidth().Height(14).PaddingX(5).Gap(3).AlignItems(native.Center).Background(base.Sidebar).Children(func() {
			for range 3 {
				native.Box(c).Size(3, 3).Radius(2).Background(base.Tertiary.Alpha(.5))
			}
		})
		native.Row(c).FillWidth().Grow(1).AlignItems(native.Stretch).Children(func() {
			native.Column(c).Width(24).Padding(4).Gap(4).Background(base.Sidebar).Children(func() {
				native.Box(c).FillWidth().Height(4).Radius(2).Background(base.Accent.Alpha(.35))
				native.Box(c).FillWidth().Height(3).Radius(2).Background(base.Border)
			})
			panel := base
			if mode == 0 {
				panel = dark
			}
			native.Column(c).Grow(1).Padding(8).Gap(5).Background(panel.Content).Children(func() {
				native.Box(c).Width(28).Height(3).Radius(2).Background(panel.Accent)
				native.Box(c).FillWidth().Height(3).Radius(2).Background(panel.Secondary.Alpha(.35))
				native.Box(c).Width(20).Height(3).Radius(2).Background(panel.Secondary.Alpha(.2))
			})
		})
	})
}

func formatSize(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) + " pt" }

// formatMultiple trims trailing zeros, so 1 shows as "1×" and 1.2 as "1.20×".
func formatMultiple(v float64) string {
	text := strconv.FormatFloat(v, 'f', 2, 64)
	for len(text) > 1 && text[len(text)-1] == '0' {
		if text[len(text)-2] == '.' {
			text = text[:len(text)-2]
			break
		}
		text = text[:len(text)-1]
	}
	return text + "×"
}

func formatContrast(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) + "×" }

func (w *Window) settingsTerminal(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsCard(c, "settings-font", "Font", p, func() {
		settingsRow(c, "Size", func() {
			slider := settingsSlider(c, "settings-font-size", &s.size, 8, 48, formatSize)
			if slider.Changed() {
				s.snapshot.Font.Size = clampFontSize(float32(s.size))
				w.pushFont()
			}
		})
		settingsRow(c, "Weight", func() {
			segment := native.Segmented(c.Key("settings-font-weight"), &s.weightIndex, "Light", "Regular", "Medium", "Semibold").Shrink(0)
			if segment.Changed() {
				s.snapshot.Font.Weight = fontWeights[max(0, min(len(fontWeights)-1, s.weightIndex))]
				w.pushFont()
			}
		})
		settingsRow(c, "Line height", func() {
			slider := settingsSlider(c, "settings-line-height", &s.lineHeight, 1, 2, formatMultiple)
			if slider.Changed() {
				s.snapshot.Font.LineHeight = max(1, min(2, float32(s.lineHeight)))
				w.pushFont()
			}
		})
		settingsCaption(c, p, "The running terminals take each change as you make it.")
	})
	w.settingsCard(c, "settings-preview", "Preview", p, func() {
		native.Text(c, "herdr pane run w1:p1 \"just test\"").Font("monospace").FontSize(s.snapshot.Font.Size)
		native.Row(c).Gap(6).PaddingY(2).Children(func() {
			native.Text(c, "ok").Font("monospace").FontSize(s.snapshot.Font.Size).TextColor(p.Success)
			native.Text(c, "warn").Font("monospace").FontSize(s.snapshot.Font.Size).TextColor(p.Warning)
			native.Text(c, "error").Font("monospace").FontSize(s.snapshot.Font.Size).TextColor(p.Danger)
		})
		settingsCaption(c, p, "Preview at "+strconv.FormatFloat(float64(s.snapshot.Font.Size), 'f', -1, 64)+" pt, weight "+strconv.Itoa(s.snapshot.Font.Weight)+".")
	})
	w.settingsCard(c, "settings-selection", "Selection", p, func() {
		settingsRow(c, "Copy on select", func() {
			box := native.Checkbox(c.Key("settings-copy-on-select"), &s.snapshot.CopyOnSelect, "Copy a selection to the clipboard as it ends")
			if box.Changed() && w.hooks.CopyOnSelect != nil {
				w.hooks.CopyOnSelect(s.snapshot.CopyOnSelect)
			}
		})
	})
	w.settingsCard(c, "settings-contrast", "Minimum contrast", p, func() {
		settingsRow(c, "Contrast", func() {
			slider := settingsSlider(c, "settings-contrast-slider", &s.contrast, 1, 7, formatContrast)
			if slider.Changed() {
				s.snapshot.MinimumContrast = max(1, min(7, float32(s.contrast)))
				if w.hooks.MinimumContrast != nil {
					w.hooks.MinimumContrast(s.snapshot.MinimumContrast)
				}
			}
		})
		settingsCaption(c, p, "Keeps text readable over program colors; 1× leaves colors untouched.")
	})
}

func (w *Window) settingsNotifications(c *native.Context, p design.Palette, s *settingsUI) {
	changed := false
	w.settingsCard(c, "settings-notifications", "Agent notifications", p, func() {
		settingsRow(c, "Delivery", func() {
			if native.Segmented(c.Key("settings-notifications-delivery"), &s.delivery, toastDeliveryLabels...).Shrink(0).Changed() {
				s.snapshot.Toast.Delivery = toastDeliveries[max(0, min(len(toastDeliveries)-1, s.delivery))]
				changed = true
			}
		})
		settingsCaption(c, p, "When an agent needs you or finishes. In-app shows a card in this window; System posts to Notification Center while you are not watching that tab.")
		if s.snapshot.Toast.Delivery == "herdr" {
			settingsRow(c, "Position", func() {
				if native.Segmented(c.Key("settings-notifications-position"), &s.position, toastPositionLabels...).Shrink(0).Changed() {
					s.snapshot.Toast.Position = toastPositions[max(0, min(len(toastPositions)-1, s.position))]
					changed = true
				}
			})
		}
		if s.snapshot.Toast.Delivery != "off" {
			settingsRow(c, "Delay", func() {
				// Herdr allows up to an hour; the slider covers the useful range
				// and leaves a longer configured delay alone until moved.
				hi := max(toastMaxSliderDelay, float64(s.snapshot.Toast.DelaySeconds))
				if settingsSlider(c, "settings-notifications-delay", &s.delay, 0, hi, formatSeconds).Changed() {
					s.delay = float64(int(s.delay + .5))
					s.snapshot.Toast.DelaySeconds = int(s.delay)
					changed = true
				}
			})
			settingsCaption(c, p, "Holds a notification back, then drops it if the agent has moved on.")
		}
		settingsCaption(c, p, "Starts from Herdr’s own [ui.toast] settings. The Dock badge counts agents waiting for you unless notifications are off.")
	})
	if changed && w.hooks.Notifications != nil {
		w.hooks.Notifications(s.snapshot.Toast)
	}
}

func formatSeconds(v float64) string { return strconv.Itoa(int(v+.5)) + " s" }

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
			label := keyGlyphs(w.BindingLabel(action.ID))
			if label == "" {
				label = "—"
			}
			native.Row(c.Key("settings-action:" + action.ID)).FillWidth().Height(20).AlignItems(native.Center).Children(func() {
				native.Text(c, w.ActionLabel(action.ID)).FontSize(12).Grow(1).SingleLine()
				native.Text(c, label).FontSize(12).TextColor(p.Secondary)
			})
		}
	})
}

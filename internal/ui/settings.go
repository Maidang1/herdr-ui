package ui

import (
	"strconv"
	"time"

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
	// OptionAsAlt makes Option the Meta key; Cursor is "block", "bar" or
	// "underline", and CursorBlink whether it blinks.
	OptionAsAlt bool
	Cursor      string
	CursorBlink bool
	Toast       ToastSettings
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
	cursorStyles        = []string{"block", "bar", "underline"}
	cursorStyleLabels   = []string{"Block", "Bar", "Underline"}
)

// systemFontLabel names the family "" picks: the system's monospaced font.
const systemFontLabel = "System monospaced"

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
	cursor      int
	// family mirrors Font.Family for the picker, systemFontLabel for "";
	// families lists the installed monospaced families, read once.
	family   string
	families []string
}

var settingsPages = []string{"Appearance", "Terminal", "Notifications", "Shortcuts", "Updates"}

var settingsPageIcons = []*native.SVG{icons.Palette, icons.Terminal, icons.MessageCircle, icons.Keyboard, icons.RotateCw}

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
	if indexOf(cursorStyles, snapshot.Cursor, -1) < 0 {
		snapshot.Cursor = "block"
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
	s.cursor = indexOf(cursorStyles, s.snapshot.Cursor, 0)
	s.family = s.snapshot.Font.Family
	if s.family == "" {
		s.family = systemFontLabel
	}
}

// fontChoices lists the picker's families: the system's monospaced font,
// then every installed monospaced family, and the chosen one should it be
// missing, as an uninstalled font is.
func (s *settingsUI) fontChoices() []string {
	if s.families == nil {
		s.families = []string{}
		for _, f := range native.FontFamilies() {
			if f.Monospaced {
				s.families = append(s.families, f.Name)
			}
		}
	}
	choices := append([]string{systemFontLabel}, s.families...)
	if s.family != systemFontLabel && indexOf(choices, s.family, -1) < 0 {
		choices = append(choices, s.family)
	}
	return choices
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

// The settings window draws its pages as System Settings does: a centered
// column of titled groups, each a card whose rows put the setting's name
// and what it does on the left and its control at the right edge, divided
// by hairlines. Footnotes sit under the card they explain.
const (
	settingsColumnWidth = 600
	settingsPagePadding = 32
	settingsRowHeight   = 44
	settingsRowPadding  = 14
	settingsSliderWidth = 160
	settingsValueWidth  = 44
	settingsSelectWidth = 220
)

// SettingsView renders the settings window as System Settings: a page
// list on the window's sidebar material on the left, under the traffic
// lights of its hidden title bar, and the page's groups on an opaque page
// on the right. Every change applies immediately through the hooks, which
// persist.
func (w *Window) SettingsView(c *native.Context) {
	p := w.appearancePalette(c)
	s := &w.settings
	s.resetMirrors()
	if w.translucent(c) {
		c.Root().Background(native.Transparent)
	} else {
		c.Root().Background(p.Sidebar)
	}
	titleBar := c.TitleBar().Height
	root := native.Row(c.Key("settings-root")).Fill().AlignItems(native.Stretch).TextColor(p.Text).FontSize(13)
	root.Children(func() {
		native.Column(c.Key("settings-nav")).Width(180).Shrink(0).FillHeight().Padding(max(18, titleBar+8), 10, 18, 10).Gap(2).Background(p.Sidebar).DragWindow().Children(func() {
			native.Text(c, "Settings").FontSize(11).FontWeight(700).TextColor(p.Tertiary).Padding(0, 10, 8, 10)
			for i, name := range settingsPages {
				page := i
				selected := s.page == page
				button := native.ButtonBase(c.Key("settings-page:"+name)).FillWidth().Height(32).Justify(native.Start).Radius(8).Padding(0, 10).Label(name).Selected(selected)
				// The selected page is filled with the accent, as a Mac
				// sidebar's selection in the key window.
				icon, label := p.Secondary, p.Text
				if selected {
					button.Background(p.Accent)
					icon, label = c.Theme().AccentText, c.Theme().AccentText
					if button.FocusVisible() {
						button.Border(2, p.Focus)
					}
				} else {
					w.controlState(button, p)
				}
				button.Children(func() {
					native.Row(c).FillWidth().Gap(9).AlignItems(native.Center).Children(func() {
						native.Icon(c, settingsPageIcons[page]).Size(15, 15).TextColor(icon)
						text := native.Text(c, name).FontSize(13).TextColor(label)
						if selected {
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
		native.Column(c.Key("settings-pane")).Grow(1).FillHeight().MinWidth(0).AlignItems(native.Stretch).Background(p.Grouped).Children(func() {
			// The hidden title bar's strip drags the window; the page
			// scrolls under it.
			if titleBar > 0 {
				native.Box(c.Key("settings-title-bar")).FillWidth().Height(titleBar).Shrink(0).DragWindow()
			}
			w.settingsPage(c, p, s, titleBar)
		})
	})
}

func (w *Window) settingsPage(c *native.Context, p design.Palette, s *settingsUI, titleBar float32) {
	top := float32(28)
	if titleBar > 0 {
		top = 4
	}
	native.Scroll(c.Key("settings-content")).Grow(1).FillHeight().MinWidth(0).AlignItems(native.Center).Children(func() {
		native.Column(c.Key("settings-page")).FillWidth().MaxWidth(settingsColumnWidth+2*settingsPagePadding).Shrink(0).Padding(top, settingsPagePadding, 40, settingsPagePadding).Gap(24).AlignItems(native.Stretch).Children(func() {
			native.Column(c).Gap(4).Children(func() {
				native.Text(c, settingsPages[s.page]).FontSize(22).FontWeight(700)
				descriptions := []string{"Make Herdr feel at home.", "Fine-tune your terminal experience.", "Choose when Herdr gets your attention.", "Keep your workflow close at hand.", "Keep Herdr UI up to date."}
				native.Text(c, descriptions[s.page]).FontSize(13).TextColor(p.Secondary)
			})
			switch s.page {
			case 0:
				w.settingsAppearance(c, p, s)
			case 1:
				w.settingsTerminal(c, p, s)
			case 2:
				w.settingsNotifications(c, p, s)
			case 4:
				w.settingsUpdates(c, p)
			default:
				w.settingsShortcuts(c, p)
			}
		})
	})
}

// settingsGroup builds the rows of one group's card, dividing each from
// the one before.
type settingsGroup struct {
	c    *native.Context
	p    design.Palette
	rows int
}

// settingsSection is one titled group: the title above its card, and the
// footnote, when there is one, under it.
func (w *Window) settingsSection(c *native.Context, key, title, footnote string, p design.Palette, rows func(g *settingsGroup)) {
	native.Column(c.Key(key)).FillWidth().Shrink(0).Gap(8).AlignItems(native.Stretch).Children(func() {
		native.Text(c, title).FontSize(13).FontWeight(600).PaddingX(4)
		g := &settingsGroup{c: c, p: p}
		// The card stands off the grouped page by its own fill, as System
		// Settings' do; Increase Contrast outlines it.
		card := native.Column(c.Key(key + "-card")).Label(title + " settings").FillWidth().Radius(10).Background(p.Elevated).Clip().AlignItems(native.Stretch)
		if c.Preferences().HighContrast {
			card.Border(1, p.Border)
		}
		card.Children(func() { rows(g) })
		if footnote != "" {
			native.Text(c, footnote).FontSize(11).TextColor(p.Tertiary).FillWidth().PaddingX(4)
		}
	})
}

// divide draws the hairline above every row but the first, inset from the
// card's leading edge as list separators are.
func (g *settingsGroup) divide() {
	if g.rows > 0 {
		native.Row(g.c).FillWidth().Height(1).Shrink(0).Padding(0, 0, 0, settingsRowPadding).Children(func() {
			native.Box(g.c).Grow(1).FillHeight().Background(g.p.Border)
		})
	}
	g.rows++
}

// row lays out one setting: its name, and under it what it does when that
// needs saying, on the left; the control at the right edge.
func (g *settingsGroup) row(label, detail string, control func()) {
	g.divide()
	c, p := g.c, g.p
	native.Row(c).FillWidth().MinHeight(settingsRowHeight).Padding(9, settingsRowPadding).Gap(16).AlignItems(native.Center).Children(func() {
		native.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			native.Text(c, label).FontSize(13)
			if detail != "" {
				native.Text(c, detail).FontSize(11).TextColor(p.Secondary).FillWidth()
			}
		})
		native.Row(c).Shrink(0).Gap(10).AlignItems(native.Center).Children(control)
	})
}

// shortcut is a short row of the action reference: the action, its ⌘ key
// and its prefix chord, "—" when it has neither.
func (g *settingsGroup) shortcut(label, direct, chord string) {
	g.divide()
	c, p := g.c, g.p
	native.Row(c).FillWidth().Height(32).PaddingX(settingsRowPadding).Gap(12).AlignItems(native.Center).Children(func() {
		native.Text(c, label).FontSize(12).Grow(1).MinWidth(0).SingleLine().Ellipsis("…")
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

// block holds content spanning the card, as the theme choices and the
// terminal preview.
func (g *settingsGroup) block(fn func()) {
	g.divide()
	native.Column(g.c).FillWidth().Padding(settingsRowPadding).Gap(10).AlignItems(native.Stretch).Children(fn)
}

// settingsSlider lays a slider and its value, aligned on its figures.
func settingsSlider(c *native.Context, key string, value *float64, lo, hi float64, format func(float64) string) native.Element {
	slider := native.Slider(c.Key(key), value, lo, hi).Width(settingsSliderWidth).Shrink(0)
	native.Text(c, format(*value)).Width(settingsValueWidth).Shrink(0).FontSize(12).FontFeatures("tnum").TextAlign(native.End).TextColor(c.Theme().TextMuted)
	return slider
}

// toggle is a row that is itself a switch, so the whole row takes the
// click: its name and detail on the left, the switch's look at the right
// edge, drawn as the theme's Switch draws it.
func (g *settingsGroup) toggle(key, label, detail string, on *bool) native.Element {
	g.divide()
	c, p := g.c, g.p
	row := native.SwitchBase(c.Key(key), on).Label(label).FillWidth().MinHeight(settingsRowHeight).Padding(9, settingsRowPadding)
	if row.Hovered() {
		row.Background(p.Hover)
	}
	if row.FocusVisible() {
		row.Border(2, p.Focus)
	}
	knob := float32(0)
	if *on {
		knob = 1
	}
	knob = row.Animate("knob", knob, 140*time.Millisecond)
	t := c.Theme()
	row.Children(func() {
		native.Row(c).FillWidth().Gap(16).AlignItems(native.Center).Children(func() {
			native.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
				native.Text(c, label).FontSize(13)
				if detail != "" {
					native.Text(c, detail).FontSize(11).TextColor(p.Secondary).FillWidth()
				}
			})
			native.Box(c).Size(t.Space(9), t.Space(5)).Shrink(0).Radius(t.Space(2.5)).Background(t.Border.Mix(t.Text, .15).Mix(t.Accent, knob)).Draw(func(paint *native.Painter, r native.Rect) {
				in := t.Space(.5)
				d := r.H - 2*in
				at := native.Rect{X: r.X + in + knob*(r.W-r.H), Y: r.Y + in, W: d, H: d}
				paint.Shadow(at, d/2, 0, 1, 3, 0, native.RGBA(0, 0, 0, .25))
				paint.Fill(at, native.RGB(255, 255, 255), d/2)
			})
		})
	})
	return row
}

func (w *Window) settingsAppearance(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsSection(c, "settings-theme", "Theme", "System follows your Mac’s light or dark appearance.", p, func(g *settingsGroup) {
		g.block(func() {
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
		})
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
	w.settingsSection(c, "settings-font", "Font", "Running terminals update as you change these.", p, func(g *settingsGroup) {
		g.block(func() { w.settingsPreview(c, p, s) })
		g.row("Family", "", func() {
			if native.Select(c.Key("settings-font-family"), &s.family, s.fontChoices()).Width(settingsSelectWidth).Shrink(0).Changed() {
				s.snapshot.Font.Family = s.family
				if s.family == systemFontLabel {
					s.snapshot.Font.Family = ""
				}
				w.pushFont()
			}
		})
		g.row("Size", "", func() {
			if settingsSlider(c, "settings-font-size", &s.size, 8, 48, formatSize).Changed() {
				s.snapshot.Font.Size = clampFontSize(float32(s.size))
				w.pushFont()
			}
		})
		g.row("Weight", "", func() {
			if native.Segmented(c.Key("settings-font-weight"), &s.weightIndex, "Light", "Regular", "Medium", "Semibold").Shrink(0).Changed() {
				s.snapshot.Font.Weight = fontWeights[max(0, min(len(fontWeights)-1, s.weightIndex))]
				w.pushFont()
			}
		})
		g.row("Line height", "", func() {
			if settingsSlider(c, "settings-line-height", &s.lineHeight, 1, 2, formatMultiple).Changed() {
				s.snapshot.Font.LineHeight = max(1, min(2, float32(s.lineHeight)))
				w.pushFont()
			}
		})
	})
	w.settingsSection(c, "settings-cursor", "Cursor", "Programs that set their own cursor, such as editors, keep theirs.", p, func(g *settingsGroup) {
		g.row("Shape", "", func() {
			if native.Segmented(c.Key("settings-cursor-shape"), &s.cursor, cursorStyleLabels...).Shrink(0).Changed() {
				s.snapshot.Cursor = cursorStyles[max(0, min(len(cursorStyles)-1, s.cursor))]
				w.pushCursor()
			}
		})
		if g.toggle("settings-cursor-blink", "Blink", "", &s.snapshot.CursorBlink).Changed() {
			w.pushCursor()
		}
	})
	w.settingsSection(c, "settings-input", "Input", "", p, func(g *settingsGroup) {
		if g.toggle("settings-option-as-alt", "Option as Meta", "Sends ⌥B, ⌥F and other Meta shortcuts to the shell and editors. Option then no longer types special characters.", &s.snapshot.OptionAsAlt).Changed() && w.hooks.OptionAsAlt != nil {
			w.hooks.OptionAsAlt(s.snapshot.OptionAsAlt)
		}
		if g.toggle("settings-copy-on-select", "Copy on select", "Copies text to the clipboard as soon as you finish selecting it.", &s.snapshot.CopyOnSelect).Changed() && w.hooks.CopyOnSelect != nil {
			w.hooks.CopyOnSelect(s.snapshot.CopyOnSelect)
		}
	})
	w.settingsSection(c, "settings-readability", "Readability", "", p, func(g *settingsGroup) {
		g.row("Minimum contrast", "Adjusts text that would be hard to read over its background. 1× keeps program colors.", func() {
			if settingsSlider(c, "settings-contrast-slider", &s.contrast, 1, 7, formatContrast).Changed() {
				s.snapshot.MinimumContrast = max(1, min(7, float32(s.contrast)))
				if w.hooks.MinimumContrast != nil {
					w.hooks.MinimumContrast(s.snapshot.MinimumContrast)
				}
			}
		})
	})
}

// settingsPreview shows a few terminal lines on the terminal's own
// background in the chosen font, weight and line height, ending at the
// chosen cursor, which blinks when it is set to.
func (w *Window) settingsPreview(c *native.Context, p design.Palette, s *settingsUI) {
	f := s.snapshot.Font
	family := f.Family
	if family == "" {
		family = "monospace"
	}
	text := func(value string, color native.Color) {
		native.Text(c, value).Font(family).FontSize(f.Size).FontWeight(f.Weight).LineHeight(f.LineHeight).TextColor(color).SingleLine()
	}
	// Words part by one cell, as the terminal spaces them: text drops
	// trailing spaces.
	cell := f.Size * .6
	native.Column(c.Key("settings-preview")).Label("Preview").FillWidth().Padding(12, 14).Gap(2).Radius(8).Background(p.Content).Border(1, p.Border).Clip().Children(func() {
		native.Row(c).Gap(cell).Children(func() {
			text("~/herdr", p.Accent)
			text("❯", p.Tertiary)
			text("herdr pane run w1:p1 \"just test\"", p.Text)
		})
		native.Row(c).Gap(cell).Children(func() {
			text("ok", p.Success)
			text("warn", p.Warning)
			text("error", p.Danger)
		})
		native.Row(c).Gap(cell).AlignItems(native.Center).Children(func() {
			text("~/herdr", p.Accent)
			text("❯", p.Tertiary)
			settingsPreviewCursor(c, s.snapshot.Cursor, s.snapshot.CursorBlink, f, p)
		})
	})
}

// cursorBlink is the cursor's half period, as the terminal blinks it.
const cursorBlink = 600 * time.Millisecond

func settingsPreviewCursor(c *native.Context, style string, blink bool, f terminal.Font, p design.Palette) {
	cell, height := f.Size*.6, f.Size*1.2*max(1, f.LineHeight)
	color := p.Text.Alpha(.85)
	if blink {
		phase := c.Now().Sub(time.Time{}) % (2 * cursorBlink)
		c.After(cursorBlink - phase%cursorBlink)
		if phase >= cursorBlink {
			color = native.Color{}
		}
	}
	cursor := native.Column(c.Key("settings-preview-cursor")).Size(cell, height).Justify(native.End)
	switch style {
	case "bar":
		cursor.Children(func() { native.Box(c).Width(2).FillHeight().Background(color) })
	case "underline":
		cursor.Children(func() { native.Box(c).FillWidth().Height(2).Background(color) })
	default:
		cursor.Background(color)
	}
}

func (w *Window) settingsNotifications(c *native.Context, p design.Palette, s *settingsUI) {
	changed := false
	w.settingsSection(c, "settings-notifications", "Agent notifications", "Starts from Herdr’s own [ui.toast] settings. The Dock badge counts agents waiting for you unless notifications are off.", p, func(g *settingsGroup) {
		g.row("Delivery", "When an agent needs you or finishes. System posts to Notification Center while you are not watching that tab.", func() {
			if native.Segmented(c.Key("settings-notifications-delivery"), &s.delivery, toastDeliveryLabels...).Shrink(0).Changed() {
				s.snapshot.Toast.Delivery = toastDeliveries[max(0, min(len(toastDeliveries)-1, s.delivery))]
				changed = true
			}
		})
		if s.snapshot.Toast.Delivery == "herdr" {
			g.row("Position", "Where the in-app card appears.", func() {
				label := toastPositionLabels[max(0, min(len(toastPositionLabels)-1, s.position))]
				if native.Select(c.Key("settings-notifications-position"), &label, toastPositionLabels).Width(160).Shrink(0).Changed() {
					s.position = indexOf(toastPositionLabels, label, 3)
					s.snapshot.Toast.Position = toastPositions[s.position]
					changed = true
				}
			})
		}
		if s.snapshot.Toast.Delivery != "off" {
			g.row("Delay", "Holds a notification back, then drops it if the agent has moved on.", func() {
				// Herdr allows up to an hour; the slider covers the useful range
				// and leaves a longer configured delay alone until moved.
				hi := max(toastMaxSliderDelay, float64(s.snapshot.Toast.DelaySeconds))
				if settingsSlider(c, "settings-notifications-delay", &s.delay, 0, hi, formatSeconds).Changed() {
					s.delay = float64(int(s.delay + .5))
					s.snapshot.Toast.DelaySeconds = int(s.delay)
					changed = true
				}
			})
		}
	})
	if changed && w.hooks.Notifications != nil {
		w.hooks.Notifications(s.snapshot.Toast)
	}
}

func formatSeconds(v float64) string { return strconv.Itoa(int(v+.5)) + " s" }

func (w *Window) settingsShortcuts(c *native.Context, p design.Palette) {
	a := w.controller
	w.settingsSection(c, "settings-keybindings", "Keybindings", "⌘ keys are fixed, as on any Mac terminal. Prefix chords come from Herdr’s own configuration.", p, func(g *settingsGroup) {
		source := ""
		if a != nil {
			source = a.machine.Bindings.Source
		}
		g.row("Herdr configuration", source, func() {
			if native.Button(c, "Edit…").Label("Edit Herdr configuration").Clicked() && w.hooks.EditConfig != nil {
				w.shortcutError(w.hooks.EditConfig())
			}
		})
	})
	w.settingsSection(c, "settings-actions", "Actions", "", p, func(g *settingsGroup) {
		for _, action := range Actions {
			g.shortcut(w.ActionLabel(action.ID), directKeyLabel(action.ID), keyGlyphs(w.BindingLabel(action.ID)))
		}
	})
}

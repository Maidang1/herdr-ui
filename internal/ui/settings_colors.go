package ui

import (
	"strings"

	"github.com/Maidang1/herdr-ui/internal/design"
	native "github.com/egoist/mygo/ui"
)

func colorSchemes() []string {
	return append([]string{"system", "hergo-light", "hergo-dark"}, design.HerdrThemeNames()...)
}

func colorSchemeLabel(name string) string {
	switch name {
	case "system":
		return "System"
	case "hergo-light":
		return "Hergo Light"
	case "hergo-dark":
		return "Hergo Dark"
	case "", "herdr":
		return "Follow Herdr configuration"
	}
	return name
}

// selectedColorScheme presents the native palette and appearance as one choice.
func (s *settingsUI) selectedColorScheme() string {
	if s.snapshot.ColorScheme != "native" {
		return s.snapshot.ColorScheme
	}
	switch s.snapshot.Theme {
	case "light":
		return "hergo-light"
	case "dark":
		return "hergo-dark"
	default:
		return "system"
	}
}

func (w *Window) chooseColorScheme(name string) {
	s := &w.settings
	s.schemeError = ""
	scheme, appearance := name, ""
	switch name {
	case "system":
		scheme, appearance = "native", "system"
	case "hergo-light":
		scheme, appearance = "native", "light"
	case "hergo-dark":
		scheme, appearance = "native", "dark"
	}
	if w.hooks.ColorScheme != nil {
		if err := w.hooks.ColorScheme(scheme); err != nil {
			s.schemeError = "Unable to apply colors. " + err.Error()
			return
		}
	}
	s.snapshot.ColorScheme = scheme
	if appearance != "" {
		s.snapshot.Theme = appearance
		if w.hooks.Theme != nil {
			w.hooks.Theme(appearance)
		}
	}
	if name != "herdr" {
		s.manualScheme = name
	}
}

func (w *Window) settingsColors(c *native.Context, p design.Palette, s *settingsUI) {
	w.settingsSection(c, "settings-colors", "Colors", "", p, func(g *settingsGroup) {
		follow := s.snapshot.ColorScheme == "herdr"
		if g.toggle("settings-follow-herdr", "Follow Herdr", "Automatically sync colors with your Herdr configuration.", &follow).Changed() {
			name := s.manualScheme
			if follow {
				name = "herdr"
			}
			w.chooseColorScheme(name)
		}
		if strings.TrimSpace(s.schemeError) != "" {
			g.block(func() { native.Text(c, s.schemeError).TextColor(p.Danger).FillWidth() })
		}
	})
}

func (w *Window) settingsColorChoice(c *native.Context, p design.Palette, s *settingsUI, g *settingsGroup) {
	following := s.snapshot.ColorScheme == "herdr"
	columns := 3
	if width, _ := c.Size(); width < 850 {
		columns = 2
	}
	g.block(func() {
		names := colorSchemes()
		native.Column(c.Key("settings-color-schemes")).FillWidth().Gap(12).Children(func() {
			for start := 0; start < len(names); start += columns {
				native.Row(c).FillWidth().Gap(12).Children(func() {
					for _, name := range names[start:min(start+columns, len(names))] {
						scheme := name
						preview := design.Colors(c.Theme().Dark)
						switch scheme {
						case "system", "hergo-light":
							preview = design.Colors(false)
						case "hergo-dark":
							preview = design.Colors(true)
						default:
							light, dark, err := design.HerdrThemes("", scheme)
							if err == nil {
								resolved := light
								if c.Theme().Dark {
									resolved = dark
								}
								preview = resolved.Colors
							}
						}
						selected := s.selectedColorScheme() == scheme
						if following {
							selected = s.manualScheme == scheme
						}
						button := native.ButtonBase(c.Key("settings-color-scheme:" + scheme)).Grow(1).Basis(0).MinWidth(0).Padding(6).Radius(8).Label(colorSchemeLabel(scheme)).Selected(selected).Disabled(following)
						button.Border(1, p.Border)
						w.controlState(button, p)
						if selected {
							button.Border(2, p.Accent).Background(p.Accent.Alpha(.06))
						}
						button.Children(func() {
							native.Column(c).FillWidth().Gap(8).Children(func() {
								panel := preview
								if scheme == "system" {
									panel = design.Colors(true)
								}
								settingsPalettePreview(c, preview, panel, p)
								native.Text(c, colorSchemeLabel(scheme)).FontSize(11).FontWeight(600).TextColor(p.Text).SingleLine().Ellipsis("…")
							})
						})
						if button.Clicked() {
							w.chooseColorScheme(scheme)
						}
					}
					for empty := min(start+columns, len(names)); empty < start+columns; empty++ {
						native.Box(c).Grow(1).Basis(0).MinWidth(0)
					}
				})
			}
		})
	})
}

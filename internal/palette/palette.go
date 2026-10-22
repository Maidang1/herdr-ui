// Package palette holds the colors of the window.
//
// The palettes are the ones the daemon uses, the same eighteen of them, with
// the same colors: the daemon keeps the theme of the session in its own
// configuration, and the window paints itself with it, the way the Rust
// client does.
package palette

import "strings"

// Color is one color of a palette.
//
// The Reset color stands for the color of the terminal itself, the way the
// daemon means it.
type Color struct {
	// R, G and B are the channels of the color.
	R, G, B uint8
	// Reset says the color is not one of its own, but the color of the
	// terminal behind it.
	Reset bool
}

// RGB returns a color of its own.
func RGB(r, g, b uint8) Color { return Color{R: r, G: g, B: b} }

// Palette is the set of colors the window paints with.
type Palette struct {
	// Accent is the color of the marks of the window.
	Accent Color
	// PanelBg is behind the panels and the bars.
	PanelBg Color
	// SidebarBg is behind the sidebar. Reset leaves the terminal behind it.
	SidebarBg Color
	// ActiveRowBg is behind the row that has the focus.
	ActiveRowBg Color
	// SelectionBg is behind the row that is chosen.
	SelectionBg Color
	// Surface0, Surface1 and SurfaceDim are the faces of the controls.
	Surface0   Color
	Surface1   Color
	SurfaceDim Color
	// Overlay0 and Overlay1 are the texts that say less and more.
	Overlay0 Color
	Overlay1 Color
	// Text is the color of the text that matters.
	Text Color
	// Subtext0 is the color of the labels that say less.
	Subtext0 Color
	// Mauve is the color of a branch name.
	Mauve Color
	// Green, Yellow, Red, Blue, Teal and Peach are the colors of the
	// states of the agents.
	Green  Color
	Yellow Color
	Red    Color
	Blue   Color
	Teal   Color
	Peach  Color
}

// Catppuccin is the palette of Catppuccin Mocha, the one the daemon uses by default.
func Catppuccin() Palette {
	return Palette{
		Accent:      RGB(137, 180, 250),
		PanelBg:     RGB(24, 24, 37),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(30, 30, 46),
		SelectionBg: RGB(49, 50, 68),
		Surface0:    RGB(49, 50, 68),
		Surface1:    RGB(69, 71, 90),
		SurfaceDim:  RGB(30, 30, 46),
		Overlay0:    RGB(108, 112, 134),
		Overlay1:    RGB(127, 132, 156),
		Text:        RGB(205, 214, 244),
		Subtext0:    RGB(166, 173, 200),
		Mauve:       RGB(203, 166, 247),
		Green:       RGB(166, 227, 161),
		Yellow:      RGB(249, 226, 175),
		Red:         RGB(243, 139, 168),
		Blue:        RGB(137, 180, 250),
		Teal:        RGB(148, 226, 213),
		Peach:       RGB(250, 179, 135),
	}
}

// CatppuccinLatte is the palette of Catppuccin Latte, the light flavour.
func CatppuccinLatte() Palette {
	return Palette{
		Accent:      RGB(30, 102, 245),
		PanelBg:     RGB(239, 241, 245),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(230, 233, 239),
		SelectionBg: RGB(189, 208, 245),
		Surface0:    RGB(204, 208, 218),
		Surface1:    RGB(188, 192, 204),
		SurfaceDim:  RGB(230, 233, 239),
		Overlay0:    RGB(156, 160, 176),
		Overlay1:    RGB(140, 143, 161),
		Text:        RGB(76, 79, 105),
		Subtext0:    RGB(108, 111, 133),
		Mauve:       RGB(136, 57, 239),
		Green:       RGB(64, 160, 43),
		Yellow:      RGB(223, 142, 29),
		Red:         RGB(210, 15, 57),
		Blue:        RGB(30, 102, 245),
		Teal:        RGB(23, 146, 153),
		Peach:       RGB(254, 100, 11),
	}
}

// Terminal is the palette of the colors of the terminal itself.
func Terminal() Palette {
	return Palette{
		Accent:      RGB(0, 0, 205),
		PanelBg:     Color{Reset: true},
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(169, 169, 169),
		SelectionBg: Color{Reset: true},
		Surface0:    Color{Reset: true},
		Surface1:    RGB(169, 169, 169),
		SurfaceDim:  RGB(169, 169, 169),
		Overlay0:    RGB(128, 128, 128),
		Overlay1:    RGB(229, 229, 229),
		Text:        Color{Reset: true},
		Subtext0:    RGB(128, 128, 128),
		Mauve:       RGB(128, 128, 128),
		Green:       RGB(0, 205, 0),
		Yellow:      RGB(205, 205, 0),
		Red:         RGB(255, 102, 102),
		Blue:        RGB(0, 0, 205),
		Teal:        RGB(0, 205, 205),
		Peach:       RGB(205, 205, 0),
	}
}

// TokyoNight is the palette of Tokyo Night.
func TokyoNight() Palette {
	return Palette{
		Accent:      RGB(122, 162, 247),
		PanelBg:     RGB(26, 27, 38),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(35, 38, 54),
		SelectionBg: RGB(45, 54, 80),
		Surface0:    RGB(36, 40, 59),
		Surface1:    RGB(65, 72, 104),
		SurfaceDim:  RGB(26, 27, 38),
		Overlay0:    RGB(86, 95, 137),
		Overlay1:    RGB(105, 113, 150),
		Text:        RGB(192, 202, 245),
		Subtext0:    RGB(169, 177, 214),
		Mauve:       RGB(187, 154, 247),
		Green:       RGB(158, 206, 106),
		Yellow:      RGB(224, 175, 104),
		Red:         RGB(247, 118, 142),
		Blue:        RGB(122, 162, 247),
		Teal:        RGB(125, 207, 255),
		Peach:       RGB(255, 158, 100),
	}
}

// TokyoNightDay is the palette of Tokyo Night Day.
func TokyoNightDay() Palette {
	return Palette{
		Accent:      RGB(46, 125, 233),
		PanelBg:     RGB(225, 226, 231),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(210, 211, 218),
		SelectionBg: RGB(182, 202, 231),
		Surface0:    RGB(196, 200, 218),
		Surface1:    RGB(168, 174, 203),
		SurfaceDim:  RGB(210, 211, 218),
		Overlay0:    RGB(137, 144, 179),
		Overlay1:    RGB(104, 112, 154),
		Text:        RGB(55, 96, 191),
		Subtext0:    RGB(97, 114, 176),
		Mauve:       RGB(120, 71, 189),
		Green:       RGB(88, 117, 57),
		Yellow:      RGB(140, 108, 62),
		Red:         RGB(245, 42, 101),
		Blue:        RGB(46, 125, 233),
		Teal:        RGB(17, 140, 116),
		Peach:       RGB(177, 92, 0),
	}
}

// Dracula is the palette of Dracula.
func Dracula() Palette {
	return Palette{
		Accent:      RGB(189, 147, 249),
		PanelBg:     RGB(40, 42, 54),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(55, 60, 82),
		SelectionBg: RGB(70, 63, 93),
		Surface0:    RGB(68, 71, 90),
		Surface1:    RGB(98, 114, 164),
		SurfaceDim:  RGB(40, 42, 54),
		Overlay0:    RGB(98, 114, 164),
		Overlay1:    RGB(130, 140, 180),
		Text:        RGB(248, 248, 242),
		Subtext0:    RGB(210, 210, 220),
		Mauve:       RGB(255, 121, 198),
		Green:       RGB(80, 250, 123),
		Yellow:      RGB(241, 250, 140),
		Red:         RGB(255, 85, 85),
		Blue:        RGB(139, 233, 253),
		Teal:        RGB(139, 233, 253),
		Peach:       RGB(255, 184, 108),
	}
}

// Nord is the palette of Nord.
func Nord() Palette {
	return Palette{
		Accent:      RGB(136, 192, 208),
		PanelBg:     RGB(46, 52, 64),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(67, 76, 94),
		SelectionBg: RGB(64, 80, 93),
		Surface0:    RGB(59, 66, 82),
		Surface1:    RGB(67, 76, 94),
		SurfaceDim:  RGB(46, 52, 64),
		Overlay0:    RGB(76, 86, 106),
		Overlay1:    RGB(100, 110, 130),
		Text:        RGB(236, 239, 244),
		Subtext0:    RGB(216, 222, 233),
		Mauve:       RGB(180, 142, 173),
		Green:       RGB(163, 190, 140),
		Yellow:      RGB(235, 203, 139),
		Red:         RGB(191, 97, 106),
		Blue:        RGB(129, 161, 193),
		Teal:        RGB(143, 188, 187),
		Peach:       RGB(208, 135, 112),
	}
}

// Gruvbox is the palette of Gruvbox.
func Gruvbox() Palette {
	return Palette{
		Accent:      RGB(215, 153, 33),
		PanelBg:     RGB(40, 40, 40),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(50, 49, 48),
		SelectionBg: RGB(75, 63, 39),
		Surface0:    RGB(60, 56, 54),
		Surface1:    RGB(80, 73, 69),
		SurfaceDim:  RGB(40, 40, 40),
		Overlay0:    RGB(146, 131, 116),
		Overlay1:    RGB(168, 153, 132),
		Text:        RGB(235, 219, 178),
		Subtext0:    RGB(213, 196, 161),
		Mauve:       RGB(211, 134, 155),
		Green:       RGB(184, 187, 38),
		Yellow:      RGB(250, 189, 47),
		Red:         RGB(251, 73, 52),
		Blue:        RGB(131, 165, 152),
		Teal:        RGB(142, 192, 124),
		Peach:       RGB(254, 128, 25),
	}
}

// GruvboxLight is the palette of Gruvbox Light.
func GruvboxLight() Palette {
	return Palette{
		Accent:      RGB(7, 102, 120),
		PanelBg:     RGB(251, 241, 199),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(242, 229, 188),
		SelectionBg: RGB(235, 219, 178),
		Surface0:    RGB(235, 219, 178),
		Surface1:    RGB(213, 196, 161),
		SurfaceDim:  RGB(242, 229, 188),
		Overlay0:    RGB(146, 131, 116),
		Overlay1:    RGB(124, 111, 100),
		Text:        RGB(60, 56, 54),
		Subtext0:    RGB(80, 73, 69),
		Mauve:       RGB(143, 63, 113),
		Green:       RGB(121, 116, 14),
		Yellow:      RGB(181, 118, 20),
		Red:         RGB(157, 0, 6),
		Blue:        RGB(7, 102, 120),
		Teal:        RGB(66, 123, 88),
		Peach:       RGB(175, 58, 3),
	}
}

// OneDark is the palette of One Dark.
func OneDark() Palette {
	return Palette{
		Accent:      RGB(97, 175, 239),
		PanelBg:     RGB(40, 44, 52),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(49, 54, 64),
		SelectionBg: RGB(51, 70, 89),
		Surface0:    RGB(44, 49, 58),
		Surface1:    RGB(62, 68, 81),
		SurfaceDim:  RGB(40, 44, 52),
		Overlay0:    RGB(92, 99, 112),
		Overlay1:    RGB(115, 122, 135),
		Text:        RGB(171, 178, 191),
		Subtext0:    RGB(150, 156, 168),
		Mauve:       RGB(198, 120, 221),
		Green:       RGB(152, 195, 121),
		Yellow:      RGB(229, 192, 123),
		Red:         RGB(224, 108, 117),
		Blue:        RGB(97, 175, 239),
		Teal:        RGB(86, 182, 194),
		Peach:       RGB(209, 154, 102),
	}
}

// OneLight is the palette of One Light.
func OneLight() Palette {
	return Palette{
		Accent:      RGB(64, 120, 242),
		PanelBg:     RGB(250, 250, 250),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(216, 219, 226),
		SelectionBg: RGB(205, 219, 248),
		Surface0:    RGB(240, 240, 241),
		Surface1:    RGB(229, 229, 230),
		SurfaceDim:  RGB(245, 245, 246),
		Overlay0:    RGB(160, 161, 167),
		Overlay1:    RGB(104, 107, 119),
		Text:        RGB(56, 58, 66),
		Subtext0:    RGB(104, 107, 119),
		Mauve:       RGB(166, 38, 164),
		Green:       RGB(80, 161, 79),
		Yellow:      RGB(193, 132, 1),
		Red:         RGB(228, 86, 73),
		Blue:        RGB(64, 120, 242),
		Teal:        RGB(1, 132, 188),
		Peach:       RGB(152, 104, 1),
	}
}

// Solarized is the palette of Solarized.
func Solarized() Palette {
	return Palette{
		Accent:      RGB(38, 139, 210),
		PanelBg:     RGB(0, 43, 54),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(22, 75, 87),
		SelectionBg: RGB(8, 62, 85),
		Surface0:    RGB(7, 54, 66),
		Surface1:    RGB(88, 110, 117),
		SurfaceDim:  RGB(0, 43, 54),
		Overlay0:    RGB(88, 110, 117),
		Overlay1:    RGB(101, 123, 131),
		Text:        RGB(147, 161, 161),
		Subtext0:    RGB(131, 148, 150),
		Mauve:       RGB(211, 54, 130),
		Green:       RGB(133, 153, 0),
		Yellow:      RGB(181, 137, 0),
		Red:         RGB(220, 50, 47),
		Blue:        RGB(38, 139, 210),
		Teal:        RGB(42, 161, 152),
		Peach:       RGB(203, 75, 22),
	}
}

// SolarizedLight is the palette of Solarized Light.
func SolarizedLight() Palette {
	return Palette{
		Accent:      RGB(38, 139, 210),
		PanelBg:     RGB(253, 246, 227),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(238, 232, 213),
		SelectionBg: RGB(201, 220, 223),
		Surface0:    RGB(238, 232, 213),
		Surface1:    RGB(147, 161, 161),
		SurfaceDim:  RGB(238, 232, 213),
		Overlay0:    RGB(147, 161, 161),
		Overlay1:    RGB(88, 110, 117),
		Text:        RGB(101, 123, 131),
		Subtext0:    RGB(131, 148, 150),
		Mauve:       RGB(211, 54, 130),
		Green:       RGB(133, 153, 0),
		Yellow:      RGB(181, 137, 0),
		Red:         RGB(220, 50, 47),
		Blue:        RGB(38, 139, 210),
		Teal:        RGB(42, 161, 152),
		Peach:       RGB(203, 75, 22),
	}
}

// Kanagawa is the palette of Kanagawa.
func Kanagawa() Palette {
	return Palette{
		Accent:      RGB(126, 156, 216),
		PanelBg:     RGB(31, 31, 40),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(54, 54, 70),
		SelectionBg: RGB(50, 56, 75),
		Surface0:    RGB(42, 42, 55),
		Surface1:    RGB(54, 54, 70),
		SurfaceDim:  RGB(31, 31, 40),
		Overlay0:    RGB(114, 113, 105),
		Overlay1:    RGB(135, 134, 125),
		Text:        RGB(220, 215, 186),
		Subtext0:    RGB(200, 195, 170),
		Mauve:       RGB(149, 127, 184),
		Green:       RGB(118, 148, 106),
		Yellow:      RGB(192, 163, 110),
		Red:         RGB(195, 64, 67),
		Blue:        RGB(126, 156, 216),
		Teal:        RGB(127, 180, 202),
		Peach:       RGB(255, 160, 102),
	}
}

// KanagawaLotus is the palette of Kanagawa Lotus.
func KanagawaLotus() Palette {
	return Palette{
		Accent:      RGB(77, 105, 155),
		PanelBg:     RGB(242, 236, 188),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(213, 206, 163),
		SelectionBg: RGB(220, 213, 172),
		Surface0:    RGB(220, 213, 172),
		Surface1:    RGB(201, 203, 209),
		SurfaceDim:  RGB(213, 206, 163),
		Overlay0:    RGB(160, 156, 172),
		Overlay1:    RGB(138, 137, 128),
		Text:        RGB(84, 84, 100),
		Subtext0:    RGB(67, 67, 108),
		Mauve:       RGB(98, 76, 131),
		Green:       RGB(111, 137, 78),
		Yellow:      RGB(119, 113, 63),
		Red:         RGB(200, 64, 83),
		Blue:        RGB(77, 105, 155),
		Teal:        RGB(78, 140, 162),
		Peach:       RGB(204, 109, 0),
	}
}

// RosePine is the palette of Rosé Pine.
func RosePine() Palette {
	return Palette{
		Accent:      RGB(196, 167, 231),
		PanelBg:     RGB(25, 23, 36),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(38, 35, 58),
		SelectionBg: RGB(59, 52, 75),
		Surface0:    RGB(31, 29, 46),
		Surface1:    RGB(38, 35, 58),
		SurfaceDim:  RGB(38, 35, 58),
		Overlay0:    RGB(110, 106, 134),
		Overlay1:    RGB(144, 140, 170),
		Text:        RGB(224, 222, 244),
		Subtext0:    RGB(200, 197, 220),
		Mauve:       RGB(196, 167, 231),
		Green:       RGB(49, 116, 143),
		Yellow:      RGB(246, 193, 119),
		Red:         RGB(235, 111, 146),
		Blue:        RGB(49, 116, 143),
		Teal:        RGB(156, 207, 216),
		Peach:       RGB(234, 154, 151),
	}
}

// RosePineDawn is the palette of Rosé Pine Dawn.
func RosePineDawn() Palette {
	return Palette{
		Accent:      RGB(144, 122, 169),
		PanelBg:     RGB(250, 244, 237),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(227, 217, 207),
		SelectionBg: RGB(242, 233, 225),
		Surface0:    RGB(242, 233, 225),
		Surface1:    RGB(255, 250, 243),
		SurfaceDim:  RGB(242, 233, 225),
		Overlay0:    RGB(152, 147, 165),
		Overlay1:    RGB(121, 117, 147),
		Text:        RGB(70, 66, 97),
		Subtext0:    RGB(121, 117, 147),
		Mauve:       RGB(144, 122, 169),
		Green:       RGB(40, 105, 131),
		Yellow:      RGB(234, 157, 52),
		Red:         RGB(180, 99, 122),
		Blue:        RGB(40, 105, 131),
		Teal:        RGB(86, 148, 159),
		Peach:       RGB(215, 130, 126),
	}
}

// Vesper is the palette of Vesper.
func Vesper() Palette {
	return Palette{
		Accent:      RGB(255, 199, 153),
		PanelBg:     RGB(26, 26, 26),
		SidebarBg:   Color{Reset: true},
		ActiveRowBg: RGB(16, 16, 16),
		SelectionBg: RGB(35, 35, 35),
		Surface0:    RGB(35, 35, 35),
		Surface1:    RGB(40, 40, 40),
		SurfaceDim:  RGB(16, 16, 16),
		Overlay0:    RGB(92, 92, 92),
		Overlay1:    RGB(126, 126, 126),
		Text:        RGB(255, 255, 255),
		Subtext0:    RGB(160, 160, 160),
		Mauve:       RGB(255, 209, 168),
		Green:       RGB(153, 255, 228),
		Yellow:      RGB(255, 199, 153),
		Red:         RGB(255, 128, 128),
		Blue:        RGB(176, 176, 176),
		Teal:        RGB(102, 221, 204),
		Peach:       RGB(255, 199, 153),
	}
}

// ByName returns the palette of a name.
//
// The names are the ones the daemon uses, and the aliases it accepts are the
// ones it accepts, so that a name the daemon understands works here too.
func ByName(name string) (Palette, bool) {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(name), "_", "-"), " ", "-")) {
	case "catppuccin", "catppuccin-mocha":
		return Catppuccin(), true
	case "catppuccin-latte", "latte", "light":
		return CatppuccinLatte(), true
	case "terminal":
		return Terminal(), true
	case "tokyo-night", "tokyonight":
		return TokyoNight(), true
	case "tokyo-night-day", "tokyo-day", "tokyonight-day":
		return TokyoNightDay(), true
	case "dracula":
		return Dracula(), true
	case "nord":
		return Nord(), true
	case "gruvbox", "gruvbox-dark":
		return Gruvbox(), true
	case "gruvbox-light":
		return GruvboxLight(), true
	case "one-dark", "onedark":
		return OneDark(), true
	case "one-light", "onelight":
		return OneLight(), true
	case "solarized":
		return Solarized(), true
	case "solarized-light":
		return SolarizedLight(), true
	case "kanagawa":
		return Kanagawa(), true
	case "kanagawa-lotus", "lotus":
		return KanagawaLotus(), true
	case "rose-pine", "rosepine":
		return RosePine(), true
	case "rose-pine-dawn", "rosepine-dawn", "dawn":
		return RosePineDawn(), true
	case "vesper":
		return Vesper(), true
	}
	return Palette{}, false
}

// Names are the names of the palettes, in the order the daemon lists them.
var Names = []string{
	"catppuccin",
	"catppuccin-latte",
	"terminal",
	"tokyo-night",
	"tokyo-night-day",
	"dracula",
	"nord",
	"gruvbox",
	"gruvbox-light",
	"one-dark",
	"one-light",
	"solarized",
	"solarized-light",
	"kanagawa",
	"kanagawa-lotus",
	"rose-pine",
	"rose-pine-dawn",
	"vesper",
}

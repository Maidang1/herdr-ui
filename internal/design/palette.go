// Package design maps the main specification's semantic tokens to native UI.
package design

import "github.com/egoist/mygo/ui"

type Palette struct {
	Shell, Top, Middle, Bottom, Sidebar, Content, Inset, Text, Secondary, Tertiary, Border, Selected, Hover, Accent, Working, Idle, Warning, Danger, Success ui.Color
}

func Colors(dark bool) Palette {
	if dark {
		return Palette{ui.Hex("#211D28"), ui.Hex("#2C2435"), ui.Hex("#2C252C"), ui.Hex("#302D25"), ui.Hex("#29232F"), ui.Hex("#211F25"), ui.Hex("#29262F"), ui.Hex("#F2EFF5"), ui.Hex("#C2BBCB"), ui.Hex("#B0A7BD"), ui.Hex("#FFFFFF12"), ui.Hex("#40394A"), ui.Hex("#FFFFFF0A"), ui.Hex("#78B7FF"), ui.Hex("#C4A6EA"), ui.Hex("#B0A7BD"), ui.Hex("#E7BF72"), ui.Hex("#FF9CA5"), ui.Hex("#83C79B")}
	}
	return Palette{ui.Hex("#EEE7EF"), ui.Hex("#EAE0EF"), ui.Hex("#F3E8E7"), ui.Hex("#F4EED3"), ui.Hex("#EEE7EF"), ui.Hex("#FFFFFF"), ui.Hex("#F5F3F7"), ui.Hex("#252329"), ui.Hex("#5D5866"), ui.Hex("#625D6B"), ui.Hex("#3026380F"), ui.Hex("#FFFFFF"), ui.Hex("#FFFFFF52"), ui.Hex("#0057A6"), ui.Hex("#68448F"), ui.Hex("#5D5866"), ui.Hex("#795000"), ui.Hex("#9F2934"), ui.Hex("#1E623C")}
}

package widgets

import (
	"herdr-ui/internal/design"

	"github.com/egoist/mygo/ui"
)

// SectionHeader is the head of a section of the sidebar.
//
// The design file's rule is that the title of a section is the first grade
// of text, in bold, and that what follows it — the way a section is
// sorted, the number of things in it — is the third grade.
func SectionHeader(c *ui.Context, t design.Tokens, title, trailing string) ui.Element {
	return ui.Row(c).
		Height(design.SidebarSectionHeight).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Padding(0, design.SpaceXL).
		Children(func() {
			Label(c, t, title, LabelOptions{
				Size:   design.FontUI,
				Color:  t.TextPrimary,
				Weight: design.WeightMedium,
			})
			if trailing != "" {
				Label(c, t, trailing, text(t, design.FontBody, t.TextTertiary))
			}
		})
}

// SidebarHeader is the head of the sidebar itself.
//
// The design file's rule is that the name of the panel is the second grade
// of text in bold, with the letters a little apart.
func SidebarHeader(c *ui.Context, t design.Tokens, title string) ui.Element {
	return ui.Row(c).
		Height(design.SidebarHeaderHeight).
		Gap(design.SpaceLG).
		AlignItems(ui.Center).
		Padding(0, design.SpaceXL).
		Children(func() {
			Label(c, t, title, LabelOptions{
				Size:    design.FontBody,
				Color:   t.TextSecondary,
				Weight:  design.WeightMedium,
				Spacing: 0.9,
			})
			ui.Row(c).Grow(1)
			// The button that hides the sidebar.
			Icon(c, t, "panel-left", design.IconMedium, t.TextTertiary)
		})
}

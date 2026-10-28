package terminalhost

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

type pluginInstance struct{ terminal *terminal.Terminal }

func (p *pluginInstance) View(c *ui.Context) ui.Element {
	// Attach may replay OSC colors or flatten the canvas into RGB cells.
	// Follow this window's theme while retaining local highlights. The
	// assertion supports pristine mygo.
	if t, ok := any(p.terminal).(interface{ SetFollowThemeColors(bool) }); ok {
		t.SetFollowThemeColors(true)
	}
	return terminal.View(c, p.terminal)
}
func (p *pluginInstance) Send(b []byte)           { p.terminal.Send(b) }
func (p *pluginInstance) SetFont(f terminal.Font) { p.terminal.SetFont(f) }
func (p *pluginInstance) SetCopyOnSelect(c terminal.CopyOnSelect) {
	p.terminal.SetCopyOnSelect(c)
}
func (p *pluginInstance) SetMinimumContrast(ratio float32) {
	p.terminal.SetMinimumContrast(ratio)
}

// The optional assertions keep ordinary builds compatible with pristine
// mygo, whose running terminals keep the options they started with.
func (p *pluginInstance) SetOptionAsAlt(on bool) {
	if t, ok := any(p.terminal).(interface{ SetOptionAsAlt(bool) }); ok {
		t.SetOptionAsAlt(on)
	}
}
func (p *pluginInstance) SetCursor(style terminal.CursorStyle, noBlink bool) {
	if t, ok := any(p.terminal).(interface {
		SetCursor(terminal.CursorStyle, bool)
	}); ok {
		t.SetCursor(style, noBlink)
	}
}
func (p *pluginInstance) Search(text string) { p.terminal.Search(text) }
func (p *pluginInstance) SearchNext()        { p.terminal.SearchNext() }
func (p *pluginInstance) SearchPrevious()    { p.terminal.SearchPrevious() }
func (p *pluginInstance) EndSearch()         { p.terminal.EndSearch() }
func (p *pluginInstance) Close() error {
	err := p.terminal.Close()
	// Close hangs up the PTY; Done fences the plugin's terminal finalization.
	// This runs in Host's cleanup job, within the caller's shared budget.
	<-p.terminal.Done()
	return err
}
func openTerminal(o terminal.Options) (instance, error) {
	if len(o.Command) == 0 || !filepath.IsAbs(o.Command[0]) {
		return nil, fmt.Errorf("attach requires an absolute Herdr executable")
	}
	// This factory always runs off the UI thread. An unavailable remote cwd
	// must not prevent the local attach client from starting.
	if o.Dir != "" {
		d, err := os.Open(o.Dir)
		if err != nil {
			o.Dir = ""
		} else {
			info, err := d.Stat()
			d.Close()
			if err != nil || !info.IsDir() {
				o.Dir = ""
			}
		}
	}
	t, err := terminal.New(o)
	if err != nil {
		return nil, err
	}
	// Herdr's attach client takes the mouse even over a plain shell, so a
	// drag must select locally rather than go to Herdr, which drops it.
	if d, ok := any(t).(interface{ SetDragSelects(bool) }); ok {
		d.SetDragSelects(true)
	}
	return &pluginInstance{t}, nil
}
func terminalThemes() (*terminal.Theme, *terminal.Theme) {
	light, dark := *terminal.LightTheme(), *terminal.DarkTheme()
	// The labels are translucent; a terminal paints its foreground solid,
	// so it takes the color the label shows on the terminal's background.
	lp, dp := design.Colors(false), design.Colors(true)
	light.Background = lp.Content
	light.Foreground = design.Over(lp.Text, lp.Content)
	light.Cursor = light.Foreground
	light.CursorText = light.Background
	light.Selection = design.NativeTheme(false, false).Selection
	light.SelectionText = light.Foreground
	dark.Background = dp.Content
	dark.Foreground = design.Over(dp.Text, dp.Content)
	dark.Cursor = dark.Foreground
	dark.CursorText = dark.Background
	dark.Selection = design.NativeTheme(true, false).Selection
	dark.SelectionText = dark.Foreground
	return &light, &dark
}

// The optional assertion keeps ordinary builds compatible with pristine mygo.
func (p *pluginInstance) InputState() InputState {
	if t, ok := any(p.terminal).(interface{ Composing() bool }); ok {
		return InputState{Composing: t.Composing(), Guarded: true}
	}
	return InputState{}
}

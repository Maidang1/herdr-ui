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

func (p *pluginInstance) View(c *ui.Context) ui.Element { return terminal.View(c, p.terminal) }
func (p *pluginInstance) Send(b []byte)                 { p.terminal.Send(b) }
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
	return &pluginInstance{t}, nil
}
func terminalThemes() (*terminal.Theme, *terminal.Theme) {
	light, dark := *terminal.LightTheme(), *terminal.DarkTheme()
	light.Background = design.Colors(false).Content
	light.Foreground = design.Colors(false).Text
	light.Cursor = light.Foreground
	light.CursorText = light.Background
	light.Selection = ui.Hex("#DCEAF8")
	light.SelectionText = light.Foreground
	dark.Background = design.Colors(true).Content
	dark.Foreground = design.Colors(true).Text
	dark.Cursor = dark.Foreground
	dark.CursorText = dark.Background
	dark.Selection = ui.Hex("#354B66")
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

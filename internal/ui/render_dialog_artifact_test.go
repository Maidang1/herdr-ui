package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	native "github.com/egoist/mygo/ui"
)

// TestRenderDialogArtifacts exports the window's dialogs, for review of
// their look in both appearances.
func TestRenderDialogArtifacts(t *testing.T) {
	dir := os.Getenv("HERDR_UI_RENDER_DIR")
	if dir == "" {
		t.Skip("set HERDR_UI_RENDER_DIR to export dialog renders")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	dialogs := map[string]func(w *Window){
		"close-tab":       func(w *Window) { w.Dispatch("close_tab") },
		"close-workspace": func(w *Window) { w.Dispatch("close_workspace") },
		"close-agent":     func(w *Window) { w.ClosePaneKey() },
		"rename-tab":      func(w *Window) { w.Dispatch("rename_tab") },
		"new-worktree":    func(w *Window) { w.Dispatch("new_worktree") },
		"reference":       func(w *Window) { w.Dispatch("help") },
		"navigator":       func(w *Window) { w.Dispatch("goto") },
	}
	for name, open := range dialogs {
		for _, dark := range []bool{false, true} {
			w := populatedChromeWindow(t)
			open(w)
			if w.controller.shortcut.prompt == nil && name != "reference" && name != "navigator" {
				w.controller.shortcut.prompt = renderPrompt(w, name)
			}
			tester := native.NewTester(w.View, 1000, 667)
			tester.SetTitleBar(native.TitleBar{Left: 80, Height: 44})
			tester.SetDark(dark)
			tester.Frame()
			tester.Frame()
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("dialog-%s-dark-%t.png", name, dark)))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tester.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func renderPrompt(w *Window, name string) *shortcutPrompt {
	v := w.currentView()
	var prompt shortcutPrompt
	switch name {
	case "close-agent":
		prompt = closeAgentPrompt("claude")
	case "rename-tab":
		prompt = promptFor("rename_tab", v)
	case "new-worktree":
		prompt = promptFor("new_worktree", v)
	default:
		prompt = promptFor(strings.ReplaceAll(name, "-", "_"), v)
	}
	prompt.focus, prompt.submit = true, func(string) {}
	return &prompt
}

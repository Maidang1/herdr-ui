package main

import (
	"context"
	"crypto/sha256"
	"github.com/Maidang1/herdr-ui/internal/design"
	"github.com/egoist/mygo/plugins/terminal"
	"os"
	"time"
)

// Herdr's themes describe UI tokens, not an ANSI palette. Keep the terminal
// plugin's ANSI colors while using Herdr's canvas, text and selection colors.
func terminalColorTheme(resolved *design.HerdrTheme) *terminal.Theme {
	if resolved == nil {
		return nil
	}
	colors := resolved.Theme
	t := terminal.LightTheme()
	if colors.Dark {
		t = terminal.DarkTheme()
	}
	t.Background, t.Foreground = resolved.Colors.Content, colors.Text
	t.Cursor, t.CursorText = colors.Text, t.Background
	t.Selection, t.SelectionText = colors.Selection, colors.Text
	return t
}

// watchHerdrColors notices content changes, including atomic saves and file
// creation/deletion. Failed parses are handled by applyColors, which keeps the
// last valid colors until the next edit.
func watchHerdrColors(ctx context.Context, path string, interval time.Duration, changed func()) {
	fingerprint := func() ([sha256.Size]byte, string) {
		data, err := os.ReadFile(path)
		if err != nil {
			return [sha256.Size]byte{}, err.Error()
		}
		return sha256.Sum256(data), ""
	}
	previous, previousError := fingerprint()
	changed()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, currentError := fingerprint()
			if current == previous && currentError == previousError {
				continue
			}
			previous, previousError = current, currentError
			changed()
		}
	}
}

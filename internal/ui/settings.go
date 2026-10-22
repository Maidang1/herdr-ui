package ui

import (
	"fmt"

	"herdr-ui/internal/config"
	"herdr-ui/internal/theme"

	"github.com/egoist/mygo/ui"
)

// The settings of the window.
//
// They live in the files the Rust client uses: `config-gpui.toml` holds the
// managed defaults and `config-gpui.local.toml` the overrides the user
// writes. The window reads both when it opens, and writes its changes into
// the overrides, so that the comments in that file stay as they are.
//
// The settings the window honors are the ones it can act on: the size of
// the characters of the terminal and of the rest of the window, the height
// of a row, and whether a selection stays highlighted after it was copied.

// settingsPanelOpen says the settings are showing.
func (a *App) settingsPanelOpen() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settingsOpen
}

// openSettings shows the settings.
//
// The settings the user changes are held apart from the ones in force, so
// that nothing changes until the user saves them.
func (a *App) openSettings() {
	a.mu.Lock()
	a.settingsOpen = true
	a.pending = a.settings
	if a.pendingBindings == nil {
		a.pendingBindings = map[string]*string{}
	}
	for _, action := range actions {
		if _, ok := a.pendingBindings[action.Name]; ok {
			continue
		}
		text := action.Default
		if set, ok := a.keybindings[action.Name]; ok && set != "" {
			text = set
		}
		value := text
		a.pendingBindings[action.Name] = &value
	}
	a.mu.Unlock()
	a.invalidate()
}

// closeSettings closes the settings.
func (a *App) closeSettings() {
	a.mu.Lock()
	a.settingsOpen = false
	a.mu.Unlock()
	a.invalidate()
}

// applySettings puts the settings the window reads into it.
func (a *App) applySettings(settings config.Settings) {
	size := float32(settings.TerminalSize)
	if size < 6 {
		size = defaultFontSize
	}
	lineHeight := settings.TerminalLineHeight
	if lineHeight <= 0 {
		lineHeight = 1.4
	}
	uiSize := float32(settings.UISize)
	if uiSize < 6 {
		uiSize = 12
	}
	a.mu.Lock()
	a.settings = settings
	a.fontSize = size
	a.rowHeight = size * float32(lineHeight)
	a.uiFontSize = uiSize
	a.dropSelection = !settings.KeepSelectionAfterCopy
	a.keybindings = settings.Keybindings
	a.layout = settings.Layout
	a.notificationsEnabled = settings.NotificationsEnabled
	a.notificationsFinished = settings.NotificationsFinished
	a.soundVolume = settings.SoundVolume
	a.bellAttention = settings.BellAttention
	a.bellSound = settings.BellSound
	a.mu.Unlock()
	a.setThemeName(settings.Theme)
	a.invalidate()
}

// loadSettings reads the settings of the window and puts them into it.
func (a *App) loadSettings() {
	settings, err := config.Load()
	if err != nil {
		a.setStatus("could not read the settings: %v", err)
		return
	}
	a.applySettings(settings)
	logf("herdr-ui: terminal %.0f points, rows %.0f pixels", settings.TerminalSize, a.rowHeightPx())
}

// saveSettings writes the settings into the overrides file and puts them
// into the window.
func (a *App) saveSettings(settings config.Settings) error {
	writes := []struct {
		key   string
		value any
	}{
		{key: "terminal.size", value: settings.TerminalSize},
		{key: "terminal.line_height", value: settings.TerminalLineHeight},
		{key: "ui.size", value: settings.UISize},
		{key: "keep_selection_after_copy", value: settings.KeepSelectionAfterCopy},
		{key: "theme", value: settings.Theme},
		{key: "layout", value: settings.Layout},
		{key: "notifications.enabled", value: settings.NotificationsEnabled},
		{key: "notifications.finished", value: settings.NotificationsFinished},
		{key: "bell.attention", value: settings.BellAttention},
		{key: "bell.sound", value: settings.BellSound},
		{key: "sound.volume", value: settings.SoundVolume},
	}
	for _, write := range writes {
		if err := config.SetLocal(write.key, write.value); err != nil {
			return err
		}
	}
	// The shortcuts of the actions are written one by one, so that the
	// table keeps the keys the user has not changed.
	a.mu.Lock()
	bindings := map[string]string{}
	for name, value := range a.pendingBindings {
		if value == nil {
			continue
		}
		bindings[name] = *value
	}
	a.mu.Unlock()
	for _, action := range actions {
		if err := config.SetLocalKeybinding(action.Name, bindings[action.Name]); err != nil {
			return err
		}
	}
	a.applySettings(settings)
	return nil
}

// themeNames returns the names of the themes the user may choose.
//
// The themes of the system come first, and the name in force stands first
// of all, so that the list always holds what the window shows.
func themeNames(current string) []string {
	names := []string{}
	seen := map[string]bool{}
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}
	add(current)
	directories, err := theme.Directories()
	if err == nil {
		for _, name := range theme.Available(directories) {
			add(name)
		}
	}
	return names
}

// drawSettings shows the settings of the window.
func (a *App) drawSettings(c *ui.Context) {
	if !a.settingsPanelOpen() {
		return
	}
	theme := c.Theme()
	a.mu.Lock()
	pending := a.pending
	bindings := map[string]*string{}
	for name, value := range a.pendingBindings {
		bindings[name] = value
	}
	a.mu.Unlock()

	size := pending.TerminalSize
	lineHeight := pending.TerminalLineHeight
	uiSize := pending.UISize
	keep := pending.KeepSelectionAfterCopy
	themeName := pending.Theme
	layoutName := pending.Layout
	notifications := pending.NotificationsEnabled
	finished := pending.NotificationsFinished
	bellAttention := pending.BellAttention
	bellSound := pending.BellSound
	volume := float64(pending.SoundVolume)

	_, local, err := config.Paths()
	source := local
	if err != nil {
		source = "the overrides file"
	}

	ui.Modal(c, &a.settingsOpen, func() {
		ui.Column(c).Width(560).MaxWidth(560).Padding(16).Gap(10).
			Background(theme.Surface).Border(1, theme.Border).Radius(10).Children(func() {
			ui.Text(c, "Settings").Bold()
			ui.Text(c, "The window reads "+source).
				FontSize(11).TextColor(theme.TextMuted)

			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Size of the terminal").Width(180).FontSize(12)
				ui.NumberInput(c, &size, 8, 28, 1)
				ui.Text(c, "points").FontSize(11).TextColor(theme.TextMuted)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Height of a row").Width(180).FontSize(12)
				ui.NumberInput(c, &lineHeight, 1, 2, 0.05)
				ui.Text(c, "of the size").FontSize(11).TextColor(theme.TextMuted)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Size of the window").Width(180).FontSize(12)
				ui.NumberInput(c, &uiSize, 8, 28, 1)
				ui.Text(c, "points").FontSize(11).TextColor(theme.TextMuted)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Keep the selection").Width(180).FontSize(12)
				ui.Checkbox(c, &keep, "Keep the selection after it was copied")
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Theme").Width(180).FontSize(12)
				ui.Select(c, &themeName, themeNames(themeName))
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Layout").Width(180).FontSize(12)
				ui.Select(c, &layoutName, LayoutNames)
			})

			ui.Text(c, "Notifications").Bold().FontSize(12)
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Notices").Width(180).FontSize(12)
				ui.Checkbox(c, &notifications, "Show the notices of the daemon")
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Finished tasks").Width(180).FontSize(12)
				ui.Checkbox(c, &finished, "Show the notices that say a task is done")
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "Loudness").Width(180).FontSize(12)
				ui.Slider(c, &volume, 0, 100)
				ui.Text(c, "percent").FontSize(11).TextColor(theme.TextMuted)
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "The bell").Width(180).FontSize(12)
				ui.Checkbox(c, &bellAttention, "Bounce the icon when a pane rings")
			})
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "").Width(180).FontSize(12)
				ui.Checkbox(c, &bellSound, "Play the sound when a pane rings")
			})

			ui.Text(c, "Shortcuts").Bold().FontSize(12)
			for _, action := range actions {
				action := action
				value := bindings[action.Name]
				if value == nil {
					text := action.Default
					value = &text
				}
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Text(c, action.Title).Width(180).FontSize(12)
					ui.TextInput(c, value).Width(180)
				})
			}

			ui.Row(c).Grow(1)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "Save").Label("Save the settings").Clicked() {
					saved := config.Settings{
						TerminalSize:           size,
						TerminalLineHeight:     lineHeight,
						UISize:                 uiSize,
						KeepSelectionAfterCopy: keep,
						Theme:                  themeName,
						Layout:                 layoutName,
						NotificationsEnabled:   notifications,
						NotificationsFinished:  finished,
						BellAttention:          bellAttention,
						BellSound:              bellSound,
						SoundVolume:            int(volume),
					}
					err := a.saveSettings(saved)
					if err != nil {
						a.setStatus("could not save the settings: %v", err)
					} else {
						a.setStatus("the settings are saved")
						a.closeSettings()
					}
				}
				if ui.Button(c, "Close").Clicked() {
					a.closeSettings()
				}
			})
		})
	})
}

// settingsSummary says what the settings are, for the status bar.
func (a *App) settingsSummary() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fmt.Sprintf("terminal %.0f points", a.fontSize)
}

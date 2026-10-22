package ui

// The actions the shortcuts stand for.
//
// Each one is a thing the window does on its own: the shortcut only says
// how the user asks for it.

// togglePalette shows or hides the command palette.
func (a *App) togglePalette() {
	if a.copyModeActive() {
		return
	}
	a.mu.Lock()
	a.palette = !a.palette
	a.query = ""
	a.mu.Unlock()
	a.invalidate()
}

// toggleFinder shows or hides the find bar.
func (a *App) toggleFinder() {
	if a.copyModeActive() {
		return
	}
	a.mu.Lock()
	a.finder.open = !a.finder.open
	a.mu.Unlock()
	a.invalidate()
}

// toggleCopyMode enters the copy mode, or leaves it.
func (a *App) toggleCopyMode() {
	if a.copyModeActive() {
		a.leaveCopyMode()
		return
	}
	a.enterCopyMode()
}

// startAnnotation writes a note to the agent about the chosen text.
func (a *App) startAnnotation() {
	if a.copyModeActive() {
		return
	}
	// The note is written on the text that is chosen.
	if !a.annotationOfferShowing() {
		a.setStatus("choose some text first")
		return
	}
	a.openAnnotation()
}

// closeTab closes the tab the window shows.
func (a *App) closeTab() {
	a.mu.Lock()
	tab := a.tab
	page := a.page
	a.mu.Unlock()
	if page != "" {
		a.closePage(page)
		return
	}
	if tab == "" {
		return
	}
	a.request("tab.close", map[string]any{"tab_id": tab})
}

// nextTab shows the tab after the one the window shows.
func (a *App) nextTab() {
	a.stepTab(1)
}

// previousTab shows the tab before the one the window shows.
func (a *App) previousTab() {
	a.stepTab(-1)
}

// stepTab shows the tab that lies a number of tabs away from the one the
// window shows.
//
// The tabs run in a ring, so the tab after the last one is the first one.
func (a *App) stepTab(step int) {
	a.mu.Lock()
	snapshot := a.snapshot
	tab := a.tab
	workspace := a.workspace
	a.mu.Unlock()
	if snapshot == nil || tab == "" {
		return
	}
	tabs := snapshot.TabsOf(workspace)
	if len(tabs) == 0 {
		return
	}
	index := -1
	for position, item := range tabs {
		if item.TabID == tab {
			index = position
			break
		}
	}
	if index < 0 {
		return
	}
	next := (index + step) % len(tabs)
	if next < 0 {
		next += len(tabs)
	}
	a.focusTab(tabs[next].TabID)
}

// quit closes the window.
func (a *App) quit() {
	if a.win == nil {
		return
	}
	a.win.Close()
}

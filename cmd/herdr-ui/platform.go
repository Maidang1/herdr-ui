package main

import (
	"github.com/egoist/mygo"
	"os/exec"
	"path/filepath"
	"runtime"
)

func commandForEditor(path string) ([]string, error) {
	command, err := editorCommand(path)
	if err != nil {
		return nil, err
	}
	resolved, err := exec.LookPath(command[0])
	if err != nil {
		return nil, err
	}
	command[0], err = filepath.Abs(resolved)
	return command, err
}

func platformTitleBar() mygo.TitleBarStyle {
	if runtime.GOOS == "darwin" {
		return mygo.TitleBarHiddenInset
	}
	// Keep Snap Layouts, system dragging and the native window controls.
	return mygo.TitleBarDefault
}

func platformEditMenu() *mygo.MenuItem {
	if runtime.GOOS == "darwin" {
		return &mygo.MenuItem{Role: mygo.RoleEditMenu}
	}
	// The terminal owns Ctrl+C/Z/A. Text fields still receive their native
	// editing keys; menu equivalents must not steal those terminal signals.
	return &mygo.MenuItem{Label: "Edit", Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleCopy, Accelerator: "Ctrl+Shift+C"},
		{Role: mygo.RolePaste, Accelerator: "Ctrl+Shift+V"},
		{Role: mygo.RoleSelectAll, Accelerator: "Ctrl+Shift+A"},
	}}
}

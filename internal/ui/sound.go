package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// The sounds of the window.
//
// The daemon sends the notices of the agents, and forwards the bell of a
// pane to the client that holds the keyboard. The settings say what the
// window does with them: play a sound, ask for attention, or both.
//
// The sounds come from the system, so that the window needs no sounds of
// its own.

const (
	// soundDone is the sound of a task that is done.
	soundDone = "Glass"
	// soundRequest is the sound of an agent that asks for attention.
	soundRequest = "Ping"
	// soundBell is the sound of the bell of a pane.
	soundBell = "Basso"
)

// playSystemSound plays one of the sounds of the system.
//
// The volume is a percentage of the loudness of the sound itself. A system
// without sounds of its own plays nothing.
func playSystemSound(name string, volume int) {
	if runtime.GOOS != "darwin" {
		return
	}
	file := filepath.Join("/System/Library/Sounds", name+".aiff")
	if _, err := os.Stat(file); err != nil {
		return
	}
	level := float64(volume) / 100
	if level <= 0 {
		return
	}
	if level > 1 {
		level = 1
	}
	command := exec.Command("afplay", "-v", strconv.FormatFloat(level, 'f', 2, 64), file)
	if err := command.Start(); err != nil {
		return
	}
	go func() {
		_ = command.Wait()
	}()
}

// askForAttention bounces the icon of the window in the dock.
//
// It does nothing when the window has the attention already, and on a
// system without a dock.
func askForAttention() {
	if runtime.GOOS != "darwin" {
		return
	}
	name := "Herdr"
	if len(os.Args) > 0 {
		if base := filepath.Base(os.Args[0]); base != "" && base != "." {
			name = strings.TrimSuffix(base, ".app")
		}
	}
	script := "tell application \"System Events\" to bounce application \"" + name + "\""
	command := exec.Command("osascript", "-e", script)
	if err := command.Start(); err != nil {
		return
	}
	go func() {
		_ = command.Wait()
	}()
}

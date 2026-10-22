package ui

import (
	"os/exec"
	"runtime"
)

// Mode is the mode of the window: the light one, the dark one, or the one
// of the system.
type Mode int

const (
	// ModeSystem follows the mode of the system.
	ModeSystem Mode = iota
	// ModeLight is the light mode of the design.
	ModeLight
	// ModeDark is the dark mode of the design.
	ModeDark
)

// String names the mode.
func (m Mode) String() string {
	switch m {
	case ModeLight:
		return "light"
	case ModeDark:
		return "dark"
	}
	return "system"
}

// Dark tells whether the window is in the dark mode.
//
// A window that follows the system asks the system; the answer is kept,
// for the system is asked only when the window is drawn.
func (m Mode) Dark(systemDark bool) bool {
	switch m {
	case ModeLight:
		return false
	case ModeDark:
		return true
	}
	return systemDark
}

// next returns the mode the window goes to when the mode is changed.
//
// The order is the light mode, the dark mode, and the system's own.
func (m Mode) next() Mode {
	switch m {
	case ModeLight:
		return ModeDark
	case ModeDark:
		return ModeSystem
	}
	return ModeLight
}

// systemIsDark tells whether the system is in the dark mode.
//
// The toolkit does not say, so the system is asked: on macOS the answer is
// in the defaults of the system, and elsewhere there is no dark mode to
// follow.
func systemIsDark() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		// The key is missing when the system is in the light mode.
		return false
	}
	return len(out) > 0
}

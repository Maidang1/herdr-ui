package design

import (
	"strconv"
	"strings"
	"syscall"
)

// WindowCornerRadius is the corner radius of a window with a toolbar, as
// herdr-ui's inset title bar has: macOS 26 rounds such windows far more
// than earlier releases did.
func WindowCornerRadius() float32 {
	version, err := syscall.Sysctl("kern.osproductversion")
	if err != nil {
		return 10
	}
	major, err := strconv.Atoi(strings.SplitN(version, ".", 2)[0])
	if err != nil || major < 26 {
		return 10
	}
	return 26
}

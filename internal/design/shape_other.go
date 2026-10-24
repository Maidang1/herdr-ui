//go:build !darwin

package design

// WindowCornerRadius approximates the desktop's window corners.
func WindowCornerRadius() float32 { return 10 }

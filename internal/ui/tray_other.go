//go:build !linux

package ui

// trayHostAvailable is always true off Linux: Windows and macOS always have
// a system tray / menu-bar area.
func trayHostAvailable() bool { return true }

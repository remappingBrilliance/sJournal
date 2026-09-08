//go:build windows

package store

// NudgeInstance is a no-op on Windows, where the system tray is always
// available so the running instance can be reopened from there.
func NudgeInstance(pid int) {}

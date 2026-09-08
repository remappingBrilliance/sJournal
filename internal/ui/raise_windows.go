//go:build windows

package ui

// watchRaiseSignal is a no-op on Windows (no SIGUSR1; the tray is always
// available to reopen the window).
func (a *App) watchRaiseSignal() {}

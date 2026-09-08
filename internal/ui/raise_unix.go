//go:build !windows

package ui

import (
	"os"
	"os/signal"
	"syscall"

	"fyne.io/fyne/v2"
)

// watchRaiseSignal listens for SIGUSR1 (sent by a second launch that was
// blocked by the single-instance lock) and brings the window to the front.
// This is what keeps "minimize to tray" safe on desktops with no visible
// tray: re-running sjournal reopens the hidden window instead of erroring.
func (a *App) watchRaiseSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			fyne.Do(a.surface)
		}
	}()
}

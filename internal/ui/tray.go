package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"sjournal/internal/ui/assets"
)

// trayQuit, when set by BuildApp, routes the tray "Quit" item through the
// unsaved-changes guard. It's read and written only on the Fyne event-loop
// goroutine (BuildApp runs there; the menu callback is dispatched via
// fyne.Do), so no lock is needed.
var trayQuit func()

// SystemTray installs the system-tray icon and menu.
//
// It MUST be called before fyne.App.Run(): the GLFW desktop driver starts
// the tray subsystem exactly once, at the top of its event loop, so a
// SetSystemTrayMenu made later (e.g. from the post-login BuildApp) registers
// a menu that never actually runs.
//
// Returns whether a tray is usable. False when the driver has no desktop
// extensions, or — on Linux — when no StatusNotifierItem host is registered
// (bare GNOME with no AppIndicator/"Tray Icons" shell extension), so the
// caller can fall back to a plain quit instead of hiding into nothing.
func SystemTray(app fyne.App, win fyne.Window, releaseLock func()) bool {
	desk, ok := app.(desktop.App)
	if !ok || !trayHostAvailable() {
		return false
	}

	show := fyne.NewMenuItem("Show sJournal", func() {
		fyne.Do(func() {
			if showMainWindow != nil {
				showMainWindow()
				return
			}
			win.Show()
			win.RequestFocus()
		})
	})
	quit := fyne.NewMenuItem("Quit", func() {
		fyne.Do(func() {
			if trayQuit != nil {
				trayQuit()
				return
			}
			releaseLock()
			app.Quit()
		})
	})
	menu := fyne.NewMenu("sJournal", show, fyne.NewMenuItemSeparator(), quit)

	desk.SetSystemTrayIcon(assets.AppIcon)
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayWindow(win)
	return true
}

//go:build linux

package ui

import "github.com/godbus/dbus/v5"

// trayHostAvailable reports whether a StatusNotifierItem host is registered
// on the session bus. On GNOME this is only true when an AppIndicator /
// "Tray Icons" shell extension is installed; without one, registering a tray
// icon silently does nothing, so "minimize to tray" would hide the window
// with no way to get it back from a tray.
func trayHostAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	obj := conn.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
	v, err := obj.GetProperty("org.kde.StatusNotifierWatcher.IsStatusNotifierHostRegistered")
	if err != nil {
		return false
	}
	registered, ok := v.Value().(bool)
	return ok && registered
}

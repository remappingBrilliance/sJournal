//go:build linux

package ui

import (
	"sync"

	"github.com/godbus/dbus/v5"

	"sjournal/internal/ui/assets"
)

// The desktop-notification path talks to org.freedesktop.Notifications
// directly over D-Bus (rather than shelling out to notify-send) so that we
// can pass the app id GNOME needs to resolve the notification, set the
// urgency hint, and — the reason this is a whole file — attach a "default"
// action and listen for it, so clicking the banner raises the window.

var (
	notifyOnce sync.Once
	notifyConn *dbus.Conn

	notifyMu  sync.Mutex
	ourNotifs = map[uint32]struct{}{} // notification ids we sent, for click matching
)

func notifyInit() {
	conn, err := dbus.SessionBus()
	if err != nil {
		return
	}
	if err := conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.Notifications"),
		dbus.WithMatchObjectPath("/org/freedesktop/Notifications"),
	); err != nil {
		return
	}
	notifyConn = conn

	ch := make(chan *dbus.Signal, 16)
	conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig.Name != "org.freedesktop.Notifications.ActionInvoked" || len(sig.Body) < 1 {
				continue
			}
			id, ok := sig.Body[0].(uint32)
			if !ok {
				continue
			}
			notifyMu.Lock()
			_, mine := ourNotifs[id]
			if mine {
				delete(ourNotifs, id)
			}
			notifyMu.Unlock()
			if mine {
				activateFromNotification()
			}
		}
	}()
}

func notifyPlatform(title, body string, critical bool) bool {
	notifyOnce.Do(notifyInit)
	if notifyConn == nil {
		return false
	}

	hints := map[string]dbus.Variant{}
	if critical {
		hints["urgency"] = dbus.MakeVariant(byte(2)) // Critical
	}

	obj := notifyConn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0,
		"sJournal",   // app_name
		uint32(0),    // replaces_id
		assets.AppID, // app_icon (icon name from the installed .desktop)
		title, body,  // summary, body
		[]string{"default", "Open sJournal"}, // actions: "default" = clicking the body
		hints,
		int32(-1), // expire_timeout: server default
	)
	if call.Err != nil {
		return false
	}

	var id uint32
	if call.Store(&id) == nil {
		notifyMu.Lock()
		if len(ourNotifs) > 64 { // bound the map if the user never clicks anything
			ourNotifs = map[uint32]struct{}{}
		}
		ourNotifs[id] = struct{}{}
		notifyMu.Unlock()
	}
	return true
}

func dismissNotificationsPlatform() {
	notifyOnce.Do(notifyInit)
	if notifyConn == nil {
		return
	}
	notifyMu.Lock()
	ids := make([]uint32, 0, len(ourNotifs))
	for id := range ourNotifs {
		ids = append(ids, id)
	}
	ourNotifs = map[uint32]struct{}{}
	notifyMu.Unlock()

	obj := notifyConn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	for _, id := range ids {
		obj.Call("org.freedesktop.Notifications.CloseNotification", 0, id)
	}
}

package ui

import "fyne.io/fyne/v2"

// OnNotificationActivated is called (on the Fyne goroutine) when the user
// clicks a sJournal desktop notification. The app sets it to raise its
// window. Currently wired only on Linux, where notifications carry a
// "default" action.
var OnNotificationActivated func()

// Notify shows an ordinary desktop notification.
func Notify(title, body string) { notify(title, body, false) }

// Alert is like Notify but at "critical" urgency, so GNOME shows the banner
// even when sJournal itself is the focused window (normal-urgency
// notifications from the focused app are dropped silently into the tray).
// Used for Journal reminders, which are meant to interrupt.
func Alert(title, body string) { notify(title, body, true) }

func notify(title, body string, critical bool) {
	if notifyPlatform(title, body, critical) {
		return
	}
	if app := fyne.CurrentApp(); app != nil {
		app.SendNotification(&fyne.Notification{Title: title, Content: body})
	}
}

func activateFromNotification() {
	if OnNotificationActivated != nil {
		fyne.Do(OnNotificationActivated)
	}
}

// DismissNotifications withdraws any sJournal desktop notifications still on
// screen or in the notification tray. Called once the user has engaged with
// a reminder (or sent the window back to the tray) so a handled "Time for a
// journal entry" banner doesn't linger.
func DismissNotifications() { dismissNotificationsPlatform() }

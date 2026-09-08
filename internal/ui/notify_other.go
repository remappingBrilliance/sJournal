//go:build !linux

package ui

// notifyPlatform has no native implementation off Linux; notify() falls
// back to Fyne's SendNotification. Windows and macOS focus the app on
// notification click on their own.
func notifyPlatform(title, body string, critical bool) bool { return false }

func dismissNotificationsPlatform() {}

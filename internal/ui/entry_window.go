package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// entryWindow is the standalone journal-entry prompt the reminder puts on
// screen: a small [Add]/[Dismiss] entry in its own OS window, deliberately
// separate from the main window so a reminder never pulls the user out of
// the tab or Edit Mode session they're in.
//
// It is built once and then reused — show() re-populates and re-shows it,
// finish() only Hide()s it. It is never Close()d during normal operation.
// The reason is a GLFW bug on the Wayland backend: destroying a top-level
// window that holds keyboard focus races the compositor's input delivery,
// and the next glfwPollEvents() dispatches a queued key event against the
// torn-down surface — _glfwInputKey(NULL) → assert → abort. Firing a fresh
// window every 30 minutes made that a matter of time. Hiding and re-showing
// one persistent window keeps the glfw window object alive, so that path is
// never taken.
type entryWindow struct {
	win   fyne.Window
	title *widget.Label
	entry *EnterModeEntry

	at       time.Time
	onSubmit func(text string) error
	onDone   func()
	busy     bool // a prompt is up and not yet dealt with
}

// newEntryWindow builds the reusable prompt window (hidden until show()).
func newEntryWindow(app fyne.App) *entryWindow {
	ew := &entryWindow{}
	ew.win = app.NewWindow("Journal reminder")

	ew.title = widget.NewLabel("")
	ew.entry = NewEnterModeEntry(func(string) { ew.finish(true) })
	ew.entry.SetPlaceHolder("Enter to add · Shift+Enter for a new line")

	add := widget.NewButton("Add", func() { ew.finish(true) })
	add.Importance = widget.HighImportance
	dismiss := widget.NewButton("Dismiss", func() { ew.finish(false) })

	ew.win.SetContent(container.NewBorder(
		ew.title,
		container.NewHBox(layout.NewSpacer(), dismiss, add),
		nil, nil,
		ew.entry,
	))
	ew.win.SetCloseIntercept(func() { ew.finish(false) })
	ew.win.Resize(fyne.NewSize(420, 240))
	return ew
}

// show brings the prompt up for fire-time at. onSubmit (may be nil) writes
// the entry and returns an error to keep the prompt up on failure; onDone
// (may be nil) runs once the prompt has been dealt with, either way.
func (ew *entryWindow) show(at time.Time, onSubmit func(text string) error, onDone func()) {
	ew.at = at
	ew.onSubmit = onSubmit
	ew.onDone = onDone
	ew.busy = true

	ew.entry.SetText("")
	ew.title.SetText(fmt.Sprintf("Time for a journal entry — %s", at.Format("15:04")))
	ew.win.Show()
	ew.win.Canvas().Focus(ew.entry)
}

func (ew *entryWindow) finish(save bool) {
	if !ew.busy {
		return
	}
	if save && ew.onSubmit != nil && strings.TrimSpace(ew.entry.Text) != "" {
		if err := ew.onSubmit(ew.entry.Text); err != nil {
			ShowError(ew.win, err) // leave the prompt up so the text isn't lost
			return
		}
	}
	ew.busy = false
	if ew.onDone != nil {
		ew.onDone()
	}

	// Hiding a Wayland surface from inside the key event that triggered the
	// hide (Enter in the entry) is silently dropped by the backend — the
	// same reason App.stashToTray defers its Hide(). Push it to the next
	// loop turn, and skip it if the prompt was shown again in the meantime.
	win := ew.win
	time.AfterFunc(0, func() {
		fyne.Do(func() {
			if !ew.busy {
				win.Hide()
			}
		})
	})
}

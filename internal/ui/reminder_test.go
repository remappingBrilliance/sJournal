package ui

import (
	"errors"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// buttonNamed walks w's content for a *widget.Button whose label is text.
func buttonNamed(t *testing.T, w fyne.Window, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	walk(w.Content(), func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == text && found == nil {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("no %q button in reminder window", text)
	}
	return found
}

func entryIn(t *testing.T, w fyne.Window) *EnterModeEntry {
	t.Helper()
	var found *EnterModeEntry
	walk(w.Content(), func(o fyne.CanvasObject) {
		if e, ok := o.(*EnterModeEntry); ok && found == nil {
			found = e
		}
	})
	if found == nil {
		t.Fatal("no EnterModeEntry in reminder window")
	}
	return found
}

func lastWindow(app fyne.App) fyne.Window {
	wins := app.Driver().AllWindows()
	return wins[len(wins)-1]
}

// showTestPrompt builds a fresh reusable entry window and shows it once, the
// way Reminder.fire / the Settings preview do.
func showTestPrompt(app fyne.App, at time.Time, onSubmit func(string) error, onDone func()) *entryWindow {
	ew := newEntryWindow(app)
	ew.show(at, onSubmit, onDone)
	return ew
}

// TestReminderWindow_EnterMatchesAdd is the regression the reminder entry
// popup used to get via ShowTextEntry: pressing Enter must do exactly what
// clicking Add does — write the typed text once, then run the follow-up
// (release the guard, dismiss the notification).
func TestReminderWindow_EnterMatchesAdd(t *testing.T) {
	run := func(useEnter bool) (string, int, int) {
		app := test.NewApp()

		var got string
		var subCalls, doneCalls int
		showTestPrompt(app, time.Date(2026, 1, 2, 9, 15, 0, 0, time.UTC),
			func(s string) error { got = s; subCalls++; return nil },
			func() { doneCalls++ })

		w := lastWindow(app)
		entryIn(t, w).SetText("today was fine")

		if useEnter {
			entryIn(t, w).TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		} else {
			test.Tap(buttonNamed(t, w, "Add"))
		}
		return got, subCalls, doneCalls
	}

	gotEnter, subEnter, doneEnter := run(true)
	gotButton, subButton, doneButton := run(false)

	if subEnter != 1 || subButton != 1 {
		t.Fatalf("onSubmit calls: enter=%d button=%d, want 1 and 1", subEnter, subButton)
	}
	if doneEnter != 1 || doneButton != 1 {
		t.Fatalf("onDone calls: enter=%d button=%d, want 1 and 1", doneEnter, doneButton)
	}
	if gotEnter != "today was fine" || gotButton != "today was fine" {
		t.Fatalf("onSubmit text: enter=%q button=%q", gotEnter, gotButton)
	}
}

// TestReminderWindow_EmptyIsDiscarded: an empty entry writes nothing but
// still counts as dealt with (onDone runs, onSubmit does not).
func TestReminderWindow_EmptyIsDiscarded(t *testing.T) {
	app := test.NewApp()

	var subCalls, doneCalls int
	showTestPrompt(app, time.Now(),
		func(string) error { subCalls++; return nil },
		func() { doneCalls++ })

	test.Tap(buttonNamed(t, lastWindow(app), "Add"))

	if subCalls != 0 {
		t.Fatalf("onSubmit ran for an empty entry (calls=%d)", subCalls)
	}
	if doneCalls != 1 {
		t.Fatalf("onDone calls=%d, want 1", doneCalls)
	}
}

// TestReminderWindow_DismissDiscards: Dismiss never writes, always finishes.
func TestReminderWindow_DismissDiscards(t *testing.T) {
	app := test.NewApp()

	var subCalls, doneCalls int
	showTestPrompt(app, time.Now(),
		func(string) error { subCalls++; return nil },
		func() { doneCalls++ })

	w := lastWindow(app)
	entryIn(t, w).SetText("changed my mind")
	test.Tap(buttonNamed(t, w, "Dismiss"))

	if subCalls != 0 {
		t.Fatalf("Dismiss wrote an entry (calls=%d)", subCalls)
	}
	if doneCalls != 1 {
		t.Fatalf("onDone calls=%d, want 1", doneCalls)
	}
}

// TestReminderWindow_WriteFailureKeepsWindow: if the write fails the window
// stays up (text not lost) and the guard is not released.
func TestReminderWindow_WriteFailureKeepsWindow(t *testing.T) {
	app := test.NewApp()

	doneCalls := 0
	showTestPrompt(app, time.Now(),
		func(string) error { return errors.New("disk full") },
		func() { doneCalls++ })

	w := lastWindow(app)
	entryIn(t, w).SetText("today was fine")
	test.Tap(buttonNamed(t, w, "Add"))

	if doneCalls != 0 {
		t.Fatalf("onDone ran despite a write failure (calls=%d)", doneCalls)
	}
	// A second Add after the error still retries rather than being wedged.
	// (the error dialog is an overlay on the same window; the entry keeps its text)
	if entryIn(t, w).Text != "today was fine" {
		t.Fatalf("entry text lost after write failure: %q", entryIn(t, w).Text)
	}
}

// TestReminderWindow_ShiftEnterInsertsNewline: Shift+Enter must not submit.
func TestReminderWindow_ShiftEnterInsertsNewline(t *testing.T) {
	app := test.NewApp()

	subCalls := 0
	showTestPrompt(app, time.Now(),
		func(string) error { subCalls++; return nil },
		func() {})

	e := entryIn(t, lastWindow(app))
	e.SetText("line one")
	e.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	e.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})

	if subCalls != 0 {
		t.Fatalf("Shift+Enter submitted (calls=%d), want 0", subCalls)
	}
}

// TestEntryWindow_ReusedNotRecreated: firing the prompt twice must drive the
// one window, not open a second. Creating/destroying a focused top-level
// window every reminder is what tripped the GLFW Wayland assert.
func TestEntryWindow_ReusedNotRecreated(t *testing.T) {
	app := test.NewApp()
	ew := newEntryWindow(app)
	before := len(app.Driver().AllWindows())

	var got string
	var subCalls, doneCalls int
	onSubmit := func(s string) error { got = s; subCalls++; return nil }
	onDone := func() { doneCalls++ }

	ew.show(time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC), onSubmit, onDone)
	entryIn(t, ew.win).SetText("morning")
	entryIn(t, ew.win).TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	ew.show(time.Date(2026, 1, 2, 17, 30, 0, 0, time.UTC), onSubmit, onDone)
	entryIn(t, ew.win).SetText("evening")
	test.Tap(buttonNamed(t, ew.win, "Add"))

	if now := len(app.Driver().AllWindows()); now != before {
		t.Fatalf("window count changed %d→%d; prompt was recreated", before, now)
	}
	if subCalls != 2 || doneCalls != 2 {
		t.Fatalf("second prompt inert: onSubmit=%d onDone=%d, want 2 and 2", subCalls, doneCalls)
	}
	if got != "evening" {
		t.Fatalf("second entry not written: got %q", got)
	}
}

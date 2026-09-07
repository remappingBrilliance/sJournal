package ui

import (
	"fyne.io/fyne/v2"

	"sjournal/internal/crypto"
	"sjournal/internal/store"
)

// RunStartup implements the spec's startup password flow: prompt for a
// password, check it against the last block of the most-recently-modified
// note file (if any exist), and offer Continue/Quit/Re-enter on mismatch.
// onReady is called with whichever password should become the session
// password; onQuit is called if the user chooses to quit.
func RunStartup(win fyne.Window, notesDir string, codec *crypto.Codec, onReady func(password string), onQuit func()) {
	var prompt func()
	prompt = func() {
		ShowPasswordEntry(win, "sJournal", "Enter your password:", "Unlock", func(pw string) {
			checkStartupPassword(win, notesDir, codec, pw, onReady, onQuit, prompt)
		}, nil)
	}
	prompt()
}

func checkStartupPassword(win fyne.Window, notesDir string, codec *crypto.Codec, pw string, onReady func(string), onQuit func(), reprompt func()) {
	path, ok, err := store.MostRecentFile(notesDir)
	if err != nil {
		ShowError(win, err)
		onQuit()
		return
	}
	if !ok {
		// No note files exist yet: nothing to check the password against.
		onReady(pw)
		return
	}

	lines, err := store.ReadLines(path)
	if err != nil {
		ShowError(win, err)
		onQuit()
		return
	}
	if len(lines) == 0 {
		onReady(pw)
		return
	}

	last := lines[len(lines)-1]
	if _, err := codec.Decrypt(pw, last); err == nil {
		onReady(pw)
		return
	}

	ShowChoice(win, "Incorrect password",
		"That password did not decrypt the most recent entry.",
		[]string{"Quit", "Continue anyway", "Re-enter"},
		func(idx int) {
			switch idx {
			case 0:
				onQuit()
			case 1:
				onReady(pw)
			case 2:
				reprompt()
			}
		})
}

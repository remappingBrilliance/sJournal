package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ShowPasswordEntry shows a modal popup with a password entry, a submit
// button (submitLabel), and — if onCancel is non-nil — a Cancel button.
// Enter in the entry field also submits.
func ShowPasswordEntry(win fyne.Window, title, message, submitLabel string, onSubmit func(password string), onCancel func()) {
	entry := widget.NewPasswordEntry()
	entry.SetPlaceHolder("Password")

	var popup *widget.PopUp

	submit := widget.NewButton(submitLabel, nil)
	submit.Importance = widget.HighImportance

	buttons := []fyne.CanvasObject{layout.NewSpacer()}
	if onCancel != nil {
		cancel := widget.NewButton("Cancel", func() {
			popup.Hide()
			onCancel()
		})
		buttons = append(buttons, cancel)
	}
	buttons = append(buttons, submit)

	submit.OnTapped = func() {
		pw := entry.Text
		popup.Hide()
		onSubmit(pw)
	}
	entry.OnSubmitted = func(string) { submit.OnTapped() }

	content := container.NewVBox(
		widget.NewLabel(message),
		entry,
		container.NewHBox(buttons...),
	)
	card := widget.NewCard(title, "", content)

	popup = widget.NewModalPopUp(card, win.Canvas())
	popup.Show()
	win.Canvas().Focus(entry)
}

// ShowChoice shows a modal popup with a message and one button per label in
// order; onChoice is called with the index of whichever was clicked.
func ShowChoice(win fyne.Window, title, message string, labels []string, onChoice func(index int)) {
	var popup *widget.PopUp

	buttons := []fyne.CanvasObject{layout.NewSpacer()}
	for i, label := range labels {
		i := i
		buttons = append(buttons, widget.NewButton(label, func() {
			popup.Hide()
			onChoice(i)
		}))
	}

	content := container.NewVBox(
		widget.NewLabel(message),
		container.NewHBox(buttons...),
	)
	card := widget.NewCard(title, "", content)

	popup = widget.NewModalPopUp(card, win.Canvas())
	popup.Show()
}

// ShowTextEntry shows a modal popup with a multi-line text entry and a
// single submit button. Enter submits; Shift+Enter inserts a newline (the
// same convention as the main Enter-Mode boxes).
func ShowTextEntry(win fyne.Window, title, message, submitLabel string, onSubmit func(text string)) {
	var popup *widget.PopUp
	var entry *EnterModeEntry

	submitted := false
	submit := func() {
		if submitted {
			return
		}
		submitted = true
		text := entry.Text
		popup.Hide()
		onSubmit(text)
	}

	entry = NewEnterModeEntry(func(string) { submit() })
	entry.SetPlaceHolder("Enter to add · Shift+Enter for a new line")

	submitBtn := widget.NewButton(submitLabel, submit)
	submitBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		widget.NewLabel(message),
		entry,
		container.NewHBox(layout.NewSpacer(), submitBtn),
	)
	card := widget.NewCard(title, "", content)

	popup = widget.NewModalPopUp(card, win.Canvas())
	popup.Resize(fyne.NewSize(420, 280))
	popup.Show()
	win.Canvas().Focus(entry)
}

func ShowInfo(win fyne.Window, title, message string) {
	dialog.ShowInformation(title, message, win)
}

func ShowError(win fyne.Window, err error) {
	dialog.ShowError(err, win)
}

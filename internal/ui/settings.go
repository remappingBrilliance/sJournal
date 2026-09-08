package ui

import (
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"sjournal/internal/config"
)

// ShowSettings opens the settings window over everything stored in
// config.yaml.
func ShowSettings(app fyne.App, parent fyne.Window, cfg *config.Config) {
	w := app.NewWindow("Settings")

	enabled := widget.NewCheck("Enable Journal reminders", nil)
	enabled.SetChecked(cfg.RemindMe.Enabled)

	start := widget.NewEntry()
	start.SetPlaceHolder("09:00")
	start.SetText(cfg.RemindMe.StartTime)

	end := widget.NewEntry()
	end.SetPlaceHolder("17:00")
	end.SetText(cfg.RemindMe.EndTime)

	interval := widget.NewEntry()
	interval.SetText(strconv.Itoa(cfg.RemindMe.Interval))

	testAlert := widget.NewButton("Test alert now", func() { testReminderAlert(parent) })

	// The start/end/interval fields do nothing unless "Enable Journal
	// reminders" (below) is ticked, so grey them out until it is.
	syncReminderFields := func() {
		for _, f := range []fyne.Disableable{start, end, interval} {
			if enabled.Checked {
				f.Enable()
			} else {
				f.Disable()
			}
		}
	}
	enabled.OnChanged = func(bool) { syncReminderFields() }
	syncReminderFields()

	timestampDefault := widget.NewCheck("Timestamp Journal entries by default", nil)
	timestampDefault.SetChecked(cfg.TimestampDefault)

	notesFolder := widget.NewEntry()
	notesFolder.SetText(cfg.NotesFolder)

	singleInstance := widget.NewCheck("Enforce single instance", nil)
	singleInstance.SetChecked(cfg.SingleInstance)

	requirePasswordForEdit := widget.NewCheck("Require password each time to switch to Edit Mode", nil)
	requirePasswordForEdit.SetChecked(cfg.RequirePasswordForEdit)

	minimizeToTray := widget.NewCheck("Close button minimizes to the system tray", nil)
	minimizeToTray.SetChecked(cfg.MinimizeToTray)

	form := widget.NewForm(
		widget.NewFormItem("Start time (HH:MM)", start),
		widget.NewFormItem("End time (HH:MM)", end),
		widget.NewFormItem("Interval (minutes)", interval),
		widget.NewFormItem("", testAlert),
		widget.NewFormItem("", enabled),
		widget.NewFormItem("", timestampDefault),
		widget.NewFormItem("", singleInstance),
		widget.NewFormItem("", requirePasswordForEdit),
		widget.NewFormItem("", minimizeToTray),
		widget.NewFormItem("Notes folder", notesFolder),
	)
	form.SubmitText = "Save"
	form.CancelText = "Cancel"
	form.OnCancel = func() { w.Close() }
	form.OnSubmit = func() {
		// Only these two settings are read once at startup; everything else
		// is picked up live.
		var restartNeeded []string
		if notesFolder.Text != cfg.NotesFolder {
			restartNeeded = append(restartNeeded, "notes folder")
		}
		if singleInstance.Checked != cfg.SingleInstance {
			restartNeeded = append(restartNeeded, "single-instance enforcement")
		}

		rm := cfg.GetRemindMe()
		rm.Enabled = enabled.Checked
		rm.StartTime = start.Text
		rm.EndTime = end.Text
		if n, err := strconv.Atoi(interval.Text); err == nil && n > 0 {
			rm.Interval = n
		}
		cfg.SetRemindMe(rm)
		cfg.TimestampDefault = timestampDefault.Checked
		cfg.NotesFolder = notesFolder.Text
		cfg.SingleInstance = singleInstance.Checked
		cfg.RequirePasswordForEdit = requirePasswordForEdit.Checked
		cfg.MinimizeToTray = minimizeToTray.Checked

		if err := cfg.Save(); err != nil {
			ShowError(parent, err)
			return
		}
		switch len(restartNeeded) {
		case 0:
			ShowInfo(parent, "Settings saved", "Your changes have been applied.")
		case 1:
			ShowInfo(parent, "Settings saved",
				"Saved. The "+restartNeeded[0]+" change takes effect after restart.")
		default:
			ShowInfo(parent, "Settings saved",
				"Saved. The "+strings.Join(restartNeeded, " and ")+" changes take effect after restart.")
		}
		w.Close()
	}

	w.SetContent(form)
	w.Resize(fyne.NewSize(440, 460))
	w.Show()
}

// testReminderAlert fires the exact chime + desktop notification + popup a
// real Journal reminder produces. It waits a few seconds first: GNOME drops
// the banner for a notification whose app is the focused window, so a
// realistic test needs sJournal in the background.
func testReminderAlert(win fyne.Window) {
	ShowInfo(win, "Test alert",
		"Firing a test reminder in 5 seconds. Switch away from sJournal or send it to the tray now — GNOME only shows a reminder banner when sJournal isn't the focused window.")
	time.AfterFunc(5*time.Second, func() {
		fyne.Do(func() {
			Chime()
			Alert("sJournal", "Test alert — this is what a Journal reminder looks like.")
			// Real reminders raise the window; mirror that here so the popup
			// is visible even if sJournal was sent to the tray.
			if showMainWindow != nil {
				showMainWindow()
			} else {
				win.Show()
				win.RequestFocus()
			}
			ShowChoice(win, "Test alert",
				"This popup is the in-app reminder. You should also have heard a chime, and — if sJournal was in the background — seen a desktop notification banner (click it to raise the window).",
				[]string{"OK"}, func(int) { DismissNotifications() })
		})
	})
}

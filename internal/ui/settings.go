package ui

import (
	"strconv"

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

	timestampDefault := widget.NewCheck("Timestamp Journal entries by default", nil)
	timestampDefault.SetChecked(cfg.TimestampDefault)

	notesFolder := widget.NewEntry()
	notesFolder.SetText(cfg.NotesFolder)

	singleInstance := widget.NewCheck("Enforce single instance", nil)
	singleInstance.SetChecked(cfg.SingleInstance)

	requirePasswordForEdit := widget.NewCheck("Require password each time to switch to Edit Mode", nil)
	requirePasswordForEdit.SetChecked(cfg.RequirePasswordForEdit)

	form := widget.NewForm(
		widget.NewFormItem("", enabled),
		widget.NewFormItem("Start time (HH:MM)", start),
		widget.NewFormItem("End time (HH:MM)", end),
		widget.NewFormItem("Interval (minutes)", interval),
		widget.NewFormItem("", timestampDefault),
		widget.NewFormItem("Notes folder", notesFolder),
		widget.NewFormItem("", singleInstance),
		widget.NewFormItem("", requirePasswordForEdit),
	)
	form.SubmitText = "Save"
	form.CancelText = "Cancel"
	form.OnCancel = func() { w.Close() }
	form.OnSubmit = func() {
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

		if err := cfg.Save(); err != nil {
			ShowError(parent, err)
			return
		}
		ShowInfo(parent, "Settings saved", "Notes folder and single-instance changes take effect after restart.")
		w.Close()
	}

	w.SetContent(form)
	w.Resize(fyne.NewSize(420, 380))
	w.Show()
}

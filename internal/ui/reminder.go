package ui

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"

	"sjournal/internal/store"
)

// Reminder implements the Journal tab's "remind me" feature: chimes and
// prompts for a journal entry at each interval within the configured
// start/end window.
type Reminder struct {
	win     fyne.Window
	sess    *Session
	journal *FileTab

	stop      chan struct{}
	lastFired string
}

func NewReminder(win fyne.Window, sess *Session, journal *FileTab) *Reminder {
	return &Reminder{win: win, sess: sess, journal: journal, stop: make(chan struct{})}
}

func (r *Reminder) Start() { go r.loop() }
func (r *Reminder) Stop()  { close(r.stop) }

func (r *Reminder) loop() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case now := <-ticker.C:
			r.check(now)
		}
	}
}

func (r *Reminder) check(now time.Time) {
	rm := r.sess.Config.GetRemindMe()
	if !rm.Enabled || rm.Interval <= 0 {
		return
	}
	start, err1 := time.Parse("15:04", rm.StartTime)
	end, err2 := time.Parse("15:04", rm.EndTime)
	if err1 != nil || err2 != nil {
		return
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), start.Hour(), start.Minute(), 0, 0, now.Location())
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), end.Hour(), end.Minute(), 0, 0, now.Location())
	if now.Before(todayStart) || now.After(todayEnd) {
		return
	}

	minutesSinceStart := int(now.Sub(todayStart).Minutes())
	if minutesSinceStart%rm.Interval != 0 {
		return
	}

	bucket := now.Format("2006-01-02 15:04")
	if bucket == r.lastFired {
		return
	}
	r.lastFired = bucket

	fireAt := now
	fyne.Do(func() { r.fire(fireAt) })
}

func (r *Reminder) fire(at time.Time) {
	fyne.CurrentApp().SendNotification(&fyne.Notification{
		Title:   "sJournal",
		Content: "Time for a journal entry",
	})

	if r.journal.editMode {
		ShowChoice(r.win, "Journal reminder", "Journal entry time — switch to Enter Mode?",
			[]string{"Ignore", "OK"}, func(idx int) {
				if idx == 1 {
					r.journal.JumpToEnd()
				}
			})
		return
	}

	ShowChoice(r.win, "Journal reminder", "Time to add a journal entry.", []string{"OK"}, func(int) {
		r.promptEntry(at)
	})
}

func (r *Reminder) promptEntry(at time.Time) {
	ShowTextEntry(r.win, "Journal entry", fmt.Sprintf("Entry for %s:", at.Format("15:04")), "Add", func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		plain := fmt.Sprintf("[%s] %s", at.Format("15:04"), text)
		line, err := r.sess.Codec.Encrypt(r.sess.Password, plain)
		if err != nil {
			ShowError(r.win, err)
			return
		}
		path := r.journal.path(store.TodayJournalName())
		if err := store.AppendLine(path, line); err != nil {
			ShowError(r.win, err)
			return
		}
		if r.journal.currentFile == store.TodayJournalName() {
			r.journal.refreshFileList()
		}
	})
}

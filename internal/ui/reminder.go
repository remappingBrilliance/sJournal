package ui

import (
	"fmt"
	"log"
	"strings"
	"sync/atomic"
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
	lastState string      // dedups the diagnostic log line
	active    atomic.Bool // a reminder prompt is on screen, unanswered
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
		r.logState("Journal reminders are off (Settings → Enable Journal reminders)")
		return
	}
	start, err1 := time.Parse("15:04", rm.StartTime)
	end, err2 := time.Parse("15:04", rm.EndTime)
	if err1 != nil || err2 != nil {
		r.logState(fmt.Sprintf("bad reminder time (start=%q end=%q)", rm.StartTime, rm.EndTime))
		return
	}

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), start.Hour(), start.Minute(), 0, 0, now.Location())
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), end.Hour(), end.Minute(), 0, 0, now.Location())
	if now.Before(todayStart) || now.After(todayEnd) {
		r.logState(fmt.Sprintf("outside reminder window %s–%s", rm.StartTime, rm.EndTime))
		return
	}
	r.logState(fmt.Sprintf("reminders armed: every %dm within %s–%s", rm.Interval, rm.StartTime, rm.EndTime))

	minutesSinceStart := int(now.Sub(todayStart).Minutes())
	if minutesSinceStart%rm.Interval != 0 {
		return
	}

	bucket := now.Format("2006-01-02 15:04")
	if bucket == r.lastFired {
		return
	}
	r.lastFired = bucket // consume this slot even if we skip below

	// If the user still hasn't dealt with the previous reminder's prompt,
	// don't stack another one — just drop this interval.
	if r.active.Load() {
		r.logState("previous reminder prompt still open — skipping this interval")
		return
	}

	fireAt := now
	log.Printf("sjournal: reminder firing at %s", fireAt.Format("15:04"))
	r.active.Store(true)
	fyne.Do(func() { r.fire(fireAt) })
}

// logState prints msg to stderr only when it differs from the last thing
// logged, so a 20-second poll doesn't spam the terminal.
func (r *Reminder) logState(msg string) {
	if msg != r.lastState {
		r.lastState = msg
		log.Println("sjournal:", msg)
	}
}

func (r *Reminder) fire(at time.Time) {
	// Remember whether the window was parked in the tray so we can put it
	// back there once the reminder has been dealt with.
	fromTray := mainWindowStashed != nil && mainWindowStashed()

	Chime()
	Alert("sJournal", "Time for a journal entry")

	// Bring the window to the front so the prompt below is visible.
	if showMainWindow != nil {
		showMainWindow()
	} else {
		r.win.Show()
		r.win.RequestFocus()
	}

	// done tidies up after the user has finished with the reminder: clear the
	// "prompt open" guard, withdraw the desktop notification, and if the
	// window came from the tray, send it back.
	done := func(returnToTray bool) {
		r.active.Store(false)
		DismissNotifications()
		if returnToTray && fromTray && stashMainWindow != nil {
			stashMainWindow()
		}
	}

	if r.journal.editMode {
		ShowChoice(r.win, "Journal reminder", "Journal entry time — switch to Enter Mode?",
			[]string{"Ignore", "OK"}, func(idx int) {
				if idx == 1 {
					r.journal.JumpToEnd() // user wants to write — keep the window up
					done(false)
					return
				}
				done(true)
			})
		return
	}

	ShowChoice(r.win, "Journal reminder", "Time to add a journal entry.", []string{"OK"}, func(int) {
		r.promptEntry(at, func() { done(true) })
	})
}

func (r *Reminder) promptEntry(at time.Time, done func()) {
	ShowTextEntry(r.win, "Journal entry", fmt.Sprintf("Entry for %s:", at.Format("15:04")), "Add", func(text string) {
		// The prompt has closed — always release the guard, even if the
		// write below fails, so reminders don't stay suppressed forever.
		r.active.Store(false)

		if strings.TrimSpace(text) == "" {
			done()
			return
		}
		plain := fmt.Sprintf("[%s] %s", at.Format("15:04"), text)
		line, err := r.sess.Codec.Encrypt(r.sess.Password, plain)
		if err != nil {
			ShowError(r.win, err) // leave the window up so the error is seen
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
		done()
	})
}

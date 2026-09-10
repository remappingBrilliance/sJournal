package ui

import (
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"

	"sjournal/internal/store"
)

// Reminder implements the Journal tab's "remind me" feature: chimes and
// prompts for a journal entry at each interval within the configured
// start/end window.
type Reminder struct {
	app     fyne.App
	sess    *Session
	journal *FileTab

	stop      chan struct{}
	lastFired string
	lastState string      // dedups the diagnostic log line
	active    atomic.Bool // a reminder prompt is on screen, unanswered

	prompt *entryWindow // the reusable standalone prompt window (lazily built)
}

func NewReminder(app fyne.App, sess *Session, journal *FileTab) *Reminder {
	return &Reminder{app: app, sess: sess, journal: journal, stop: make(chan struct{})}
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
	Chime()
	Alert("sJournal", "Time for a journal entry")

	// The prompt is its own top-level window, deliberately separate from the
	// main window: it must not steal the user away from whatever tab or Edit
	// Mode session they're in, so the reminder never touches the main window
	// (no raise, no un-stash from the tray). It is built once and reused —
	// see entryWindow for why we never open a fresh one per fire.
	if r.prompt == nil {
		r.prompt = newEntryWindow(r.app)
	}
	r.prompt.show(at, func(text string) error {
		return r.writeEntry(at, text)
	}, func() {
		r.active.Store(false)
		DismissNotifications()
	})
}

// writeEntry encrypts and appends the reminder's journal line, then nudges
// the Journal tab so it stays consistent with the file that was written
// behind its back (midnight roll-over; a live Edit Mode session).
func (r *Reminder) writeEntry(at time.Time, text string) error {
	plain := fmt.Sprintf("[%s] %s", at.Format("15:04"), text)
	line, err := r.sess.Codec.Encrypt(r.sess.Password, plain)
	if err != nil {
		return err
	}
	today := store.TodayJournalName()
	if err := store.AppendLine(r.journal.path(today), line); err != nil {
		return err
	}
	r.journal.AbsorbReminderEntry(plain)
	return nil
}

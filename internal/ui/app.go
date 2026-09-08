package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// App wires the three FileTabs into the main window, handling the tab-switch
// unsaved-changes guard, the Settings/Quit menu, and the reminder scheduler.
type App struct {
	win  fyne.Window
	sess *Session

	journal *FileTab
	todo    *FileTab
	notes   *FileTab
	tabs    *container.AppTabs

	reminder *Reminder

	prevIndex int
	reverting bool

	hiddenToTray  bool
	trayHintShown bool

	onQuit func()
}

// Window-control hooks, set by BuildApp. They exist as package vars because
// the tray (wired in main() before BuildApp) and the reminder both need to
// drive the main window through the App's tray-state bookkeeping.
var (
	showMainWindow    func()      // show + focus, clearing "stashed in tray"
	stashMainWindow   func()      // hide back to the tray, marking it stashed
	mainWindowStashed func() bool // is the window currently in the tray?
)

func BuildApp(app fyne.App, win fyne.Window, sess *Session) *App {
	a := &App{win: win, sess: sess}

	a.journal = newFileTab(win, sess, kindJournal)
	a.todo = newFileTab(win, sess, kindTodo)
	a.notes = newFileTab(win, sess, kindNotes)

	a.tabs = container.NewAppTabs(
		container.NewTabItem("Journal", a.journal.Container()),
		container.NewTabItem("Todo List", a.todo.Container()),
		container.NewTabItem("Notes", a.notes.Container()),
	)
	a.tabs.OnSelected = func(*container.TabItem) { a.onTabSelected() }

	win.SetContent(a.tabs)
	win.Resize(fyne.NewSize(760, 600))
	win.SetMainMenu(a.buildMenu(app))
	win.SetCloseIntercept(func() { a.onCloseRequested() })

	// The tray itself is registered in main() before app.Run(); here we just
	// point its "Quit" item at the unsaved-changes guard.
	trayQuit = a.attemptQuit
	showMainWindow = a.surface
	stashMainWindow = a.stashToTray
	mainWindowStashed = func() bool { return a.hiddenToTray }
	OnNotificationActivated = a.surface
	a.watchRaiseSignal()

	a.reminder = NewReminder(win, sess, a.journal)
	a.reminder.Start()

	return a
}

// surface shows and focuses the main window, clearing the "stashed in the
// tray" state.
func (a *App) surface() {
	a.hiddenToTray = false
	a.win.Show()
	a.win.RequestFocus()
}

// stashToTray hides the main window and records that it's parked in the
// tray. Also withdraws any outstanding sJournal desktop notifications so a
// handled reminder banner doesn't linger.
func (a *App) stashToTray() {
	a.hiddenToTray = true
	a.win.Hide()
	DismissNotifications()
}

// onCloseRequested runs when the window's close button is pressed. With
// MinimizeToTray enabled and a tray available, it hides the window instead
// of quitting; otherwise it goes through the normal unsaved-changes quit.
func (a *App) onCloseRequested() {
	if a.sess.Config.MinimizeToTray && a.sess.TrayEnabled {
		a.stashToTray()
		if !a.trayHintShown {
			a.trayHintShown = true
			Notify("sJournal", "Still running in the system tray — click the tray icon to reopen or quit.")
		}
		return
	}
	a.attemptQuit()
}

// SetOnQuit registers a callback (e.g. releasing the single-instance lock)
// run just before the app actually exits.
func (a *App) SetOnQuit(fn func()) { a.onQuit = fn }

func (a *App) buildMenu(app fyne.App) *fyne.MainMenu {
	settingsItem := fyne.NewMenuItem("Settings...", func() {
		ShowSettings(app, a.win, a.sess.Config)
	})

	items := []*fyne.MenuItem{settingsItem, fyne.NewMenuItemSeparator()}
	if a.sess.Config.MinimizeToTray && a.sess.TrayEnabled {
		hideItem := fyne.NewMenuItem("Hide to Tray", func() { a.win.Hide() })
		items = append(items, hideItem)
	}
	quitItem := fyne.NewMenuItem("Quit", func() { a.attemptQuit() })
	quitItem.IsQuit = true
	items = append(items, quitItem)

	return fyne.NewMainMenu(fyne.NewMenu("File", items...))
}

func (a *App) tabAt(i int) *FileTab {
	switch i {
	case 0:
		return a.journal
	case 1:
		return a.todo
	default:
		return a.notes
	}
}

func (a *App) onTabSelected() {
	if a.reverting {
		return
	}
	newIndex := a.tabs.SelectedIndex()
	if newIndex == a.prevIndex {
		return
	}

	oldTab := a.tabAt(a.prevIndex)
	if !oldTab.IsDirty() {
		a.prevIndex = newIndex
		return
	}

	prevIndex := a.prevIndex
	a.reverting = true
	a.tabs.SelectIndex(prevIndex)
	a.reverting = false

	oldTab.guardUnsaved(func() {
		a.prevIndex = newIndex
		a.reverting = true
		a.tabs.SelectIndex(newIndex)
		a.reverting = false
	}, nil)
}

func (a *App) attemptQuit() {
	active := a.tabAt(a.tabs.SelectedIndex())
	if active.IsDirty() {
		active.guardUnsaved(func() { a.doQuit() }, nil)
		return
	}
	a.doQuit()
}

func (a *App) doQuit() {
	if a.onQuit != nil {
		a.onQuit()
	}
	fyne.CurrentApp().Quit()
}

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

	onQuit func()
}

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
	win.SetCloseIntercept(func() { a.attemptQuit() })

	a.reminder = NewReminder(win, sess, a.journal)
	a.reminder.Start()

	return a
}

// SetOnQuit registers a callback (e.g. releasing the single-instance lock)
// run just before the app actually exits.
func (a *App) SetOnQuit(fn func()) { a.onQuit = fn }

func (a *App) buildMenu(app fyne.App) *fyne.MainMenu {
	settingsItem := fyne.NewMenuItem("Settings...", func() {
		ShowSettings(app, a.win, a.sess.Config)
	})
	quitItem := fyne.NewMenuItem("Quit", func() { a.attemptQuit() })
	return fyne.NewMainMenu(fyne.NewMenu("File", settingsItem, quitItem))
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

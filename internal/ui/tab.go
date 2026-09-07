package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"sjournal/internal/store"
)

type tabKind int

const (
	kindJournal tabKind = iota
	kindTodo
	kindNotes
)

// FileTab implements the shared Journal/Todo/Notes tab behavior: a file
// dropdown, optional New File button, Edit/Enter mode toggle, and (for
// Journal) the timestamp checkbox.
type FileTab struct {
	win  fyne.Window
	sess *Session
	kind tabKind

	editMode    bool
	dirty       bool
	currentFile string

	fileSelect     *widget.Select
	modeBtn        *widget.Button
	timestampCheck *widget.Check

	center     *fyne.Container
	enterEntry *EnterModeEntry
	editor     *BlockEditor

	root *fyne.Container
}

func newFileTab(win fyne.Window, sess *Session, kind tabKind) *FileTab {
	t := &FileTab{win: win, sess: sess, kind: kind}

	t.fileSelect = widget.NewSelect(nil, func(name string) {
		if name == "" || name == t.currentFile {
			return
		}
		prevSelection := t.currentFile
		t.guardUnsaved(
			func() { t.openFile(name) },
			func() { t.fileSelect.SetSelected(prevSelection) },
		)
	})

	var top []fyne.CanvasObject
	top = append(top, t.fileSelect)

	if kind != kindJournal {
		newFileBtn := widget.NewButton("New File", func() {
			t.guardUnsaved(func() { t.createNewFile() }, nil)
		})
		top = append(top, newFileBtn)
	}

	if kind == kindJournal {
		t.timestampCheck = widget.NewCheck("Timestamp each entry", func(checked bool) {
			t.sess.Config.TimestampDefault = checked
			_ = t.sess.Config.Save()
		})
		t.timestampCheck.SetChecked(sess.Config.TimestampDefault)
		top = append(top, t.timestampCheck)
	}

	t.modeBtn = widget.NewButton("", func() { t.toggleMode() })
	top = append(top, layout.NewSpacer(), t.modeBtn)

	t.center = container.NewStack()
	t.root = container.NewBorder(container.NewHBox(top...), nil, nil, nil, t.center)

	name, err := t.initialFile()
	if err != nil {
		ShowError(t.win, err)
	}
	t.currentFile = name
	t.refreshFileList()
	t.showEnterMode()

	return t
}

func (t *FileTab) Container() fyne.CanvasObject { return t.root }

func (t *FileTab) path(name string) string {
	return filepath.Join(t.sess.NotesDir, name)
}

func (t *FileTab) prefix() string {
	if t.kind == kindNotes {
		return "NOTES"
	}
	return "TODO"
}

func (t *FileTab) listFiles() ([]string, error) {
	switch t.kind {
	case kindJournal:
		return store.ListJournalFiles(t.sess.NotesDir)
	default:
		return store.ListNumbered(t.sess.NotesDir, t.prefix())
	}
}

func (t *FileTab) nextFileName() (string, error) {
	return store.NextNumberedName(t.sess.NotesDir, t.prefix())
}

func (t *FileTab) initialFile() (string, error) {
	if t.kind == kindJournal {
		return store.TodayJournalName(), nil
	}
	files, err := t.listFiles()
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		return files[0], nil
	}
	return t.nextFileName()
}

func (t *FileTab) refreshFileList() {
	files, err := t.listFiles()
	if err != nil {
		ShowError(t.win, err)
		return
	}
	options := files
	if t.currentFile != "" && !contains(files, t.currentFile) {
		options = append([]string{t.currentFile}, files...)
	}
	t.fileSelect.Options = options
	t.fileSelect.SetSelected(t.currentFile)
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (t *FileTab) openFile(name string) {
	t.currentFile = name
	t.refreshFileList()
	if t.editMode {
		t.showEditMode()
	} else {
		t.showEnterMode()
	}
}

func (t *FileTab) createNewFile() {
	name, err := t.nextFileName()
	if err != nil {
		ShowError(t.win, err)
		return
	}
	t.openFile(name)
}

// guardUnsaved runs proceed immediately unless the tab is in Edit Mode with
// unsaved changes, in which case it prompts Save/Discard/Cancel first.
// onCancel (may be nil) runs if the user cancels, e.g. to revert a dropdown
// selection that already changed visually.
func (t *FileTab) guardUnsaved(proceed func(), onCancel func()) {
	if !t.editMode || !t.dirty {
		proceed()
		return
	}
	ShowChoice(t.win, "Unsaved changes", "This file has unsaved changes in Edit Mode.",
		[]string{"Cancel", "Discard", "Save"}, func(idx int) {
			switch idx {
			case 0:
				if onCancel != nil {
					onCancel()
				}
			case 1:
				t.dirty = false
				proceed()
			case 2:
				if t.saveEditMode() {
					proceed()
				}
			}
		})
}

func (t *FileTab) toggleMode() {
	if t.editMode {
		if !t.saveEditMode() {
			return
		}
		t.showEnterMode()
		return
	}
	if t.sess.Config.RequirePasswordForEdit {
		ShowPasswordEntry(t.win, "Confirm password", "Re-enter your password to switch to Edit Mode:", "Unlock", func(pw string) {
			if pw != t.sess.Password {
				ShowInfo(t.win, "Incorrect password", "That password does not match.")
				return
			}
			t.showEditMode()
		}, func() {})
		return
	}
	t.showEditMode()
}

func (t *FileTab) showEnterMode() {
	t.editMode = false
	t.dirty = false
	var entry *EnterModeEntry
	entry = NewEnterModeEntry(func(text string) {
		entry.SetText("")
		t.handleEnter(text)
	})
	t.enterEntry = entry
	t.center.Objects = []fyne.CanvasObject{entry}
	t.center.Refresh()
	t.modeBtn.SetText("Switch to Edit Mode")
	safeFocus(t.win, entry)
}

// safeFocus focuses obj only if it's actually attached to a canvas, which
// isn't true yet the first time a tab builds its initial content (before
// the window's SetContent has run) — Canvas.Focus logs a Fyne error and
// no-ops otherwise.
func safeFocus(win fyne.Window, obj fyne.CanvasObject) {
	if fyne.CurrentApp().Driver().CanvasForObject(obj) == nil {
		return
	}
	if f, ok := obj.(fyne.Focusable); ok {
		win.Canvas().Focus(f)
	}
}

// deferFocus focuses obj once it's actually attached to the canvas. A
// widget added to a container this same tick isn't attached yet —
// Fyne registers it in the canvas's object cache during the next
// layout/paint pass, not synchronously on Refresh() — so unlike safeFocus
// (a one-shot best-effort check), this retries for a short while.
func deferFocus(win fyne.Window, obj fyne.CanvasObject) {
	var attempt func(remaining int)
	attempt = func(remaining int) {
		if fyne.CurrentApp().Driver().CanvasForObject(obj) != nil {
			safeFocus(win, obj)
			return
		}
		if remaining <= 0 {
			return
		}
		time.AfterFunc(16*time.Millisecond, func() {
			fyne.Do(func() { attempt(remaining - 1) })
		})
	}
	attempt(10)
}

func (t *FileTab) showEditMode() {
	blocks, err := store.LoadDocument(t.path(t.currentFile), t.sess.Codec, []string{t.sess.Password})
	if err != nil {
		ShowError(t.win, err)
		return
	}
	t.editMode = true
	t.dirty = false
	editor := NewBlockEditor(t.win, blocks, t.kind == kindTodo, t.sess.Codec, func() { t.dirty = true })
	t.editor = editor
	saveBtn := widget.NewButton("Save", func() { t.saveEditMode() })
	panel := container.NewBorder(nil, saveBtn, nil, nil, editor)
	t.center.Objects = []fyne.CanvasObject{panel}
	t.center.Refresh()
	t.modeBtn.SetText("Switch to Enter Mode")
}

func (t *FileTab) saveEditMode() bool {
	if t.editor == nil {
		return true
	}
	lines, err := t.editor.Lines(t.sess.Password)
	if err != nil {
		ShowError(t.win, err)
		return false
	}
	if err := store.WriteLines(t.path(t.currentFile), lines); err != nil {
		ShowError(t.win, err)
		return false
	}
	t.dirty = false
	t.refreshFileList()
	return true
}

func (t *FileTab) handleEnter(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	plain := text
	if t.kind == kindJournal && t.timestampCheck != nil && t.timestampCheck.Checked {
		plain = fmt.Sprintf("[%s] %s", time.Now().Format("15:04"), text)
	}
	if t.kind == kindTodo {
		plain = fmt.Sprintf("- [ ] %s", text)
	}

	line, err := t.sess.Codec.Encrypt(t.sess.Password, plain)
	if err != nil {
		ShowError(t.win, err)
		return
	}
	if err := store.AppendLine(t.path(t.currentFile), line); err != nil {
		ShowError(t.win, err)
		return
	}
	t.refreshFileList()
}

// IsDirty reports whether this tab currently has unsaved Edit Mode changes.
func (t *FileTab) IsDirty() bool { return t.editMode && t.dirty }

// JumpToEnd switches to Enter Mode (used when a reminder popup's OK is
// pressed while this tab was in Edit Mode).
func (t *FileTab) JumpToEnd() {
	t.guardUnsaved(func() { t.showEnterMode() }, nil)
}

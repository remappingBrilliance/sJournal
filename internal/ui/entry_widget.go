package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

var _ desktop.Keyable = (*EnterModeEntry)(nil)

// EnterModeEntry is a multi-line entry where plain Enter fires OnEnter
// instead of inserting a newline, and Shift+Enter inserts a newline as
// normal. Note: Fyne's driver deliberately never synthesizes a
// desktop.CustomShortcut for a bare Shift modifier (see
// internal/driver/glfw/window.go's triggersShortcut), so Shift+Enter cannot
// be detected via the shortcut system — instead this tracks Shift up/down
// itself via desktop.Keyable, the same mechanism widget.Entry uses
// internally for Shift+Arrow text selection.
type EnterModeEntry struct {
	widget.Entry

	OnEnter func(text string)

	shiftHeld bool
}

func NewEnterModeEntry(onEnter func(text string)) *EnterModeEntry {
	e := &EnterModeEntry{OnEnter: onEnter}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

func (e *EnterModeEntry) KeyDown(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shiftHeld = true
	}
	e.Entry.KeyDown(key)
}

func (e *EnterModeEntry) KeyUp(key *fyne.KeyEvent) {
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shiftHeld = false
	}
	e.Entry.KeyUp(key)
}

// TypedKey intercepts plain Return/Enter to fire OnEnter instead of
// inserting a newline. With Shift held, it falls through to the embedded
// Entry's default multiline behavior, which inserts a newline.
func (e *EnterModeEntry) TypedKey(key *fyne.KeyEvent) {
	if (key.Name == fyne.KeyReturn || key.Name == fyne.KeyEnter) && !e.shiftHeld {
		if e.OnEnter != nil {
			e.OnEnter(e.Text)
		}
		return
	}
	e.Entry.TypedKey(key)
}

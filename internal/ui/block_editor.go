package ui

import (
	"fmt"
	"regexp"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"sjournal/internal/crypto"
	"sjournal/internal/store"
)

var todoLineRe = regexp.MustCompile(`(?s)^- \[([ xX])\] (.*)$`)

// lockedBlock is a small widget for a still-encrypted block: shown greyed
// out, right-click offers a one-off password to unlock it for this Edit
// Mode session.
type lockedBlock struct {
	widget.BaseWidget
	label    *widget.Label
	onUnlock func()
}

func newLockedBlock(onUnlock func()) *lockedBlock {
	l := &lockedBlock{
		label:    widget.NewLabel("\U0001F512 encrypted — right-click to unlock"),
		onUnlock: onUnlock,
	}
	l.label.Importance = widget.LowImportance
	l.ExtendBaseWidget(l)
	return l
}

func (l *lockedBlock) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(l.label)
}

func (l *lockedBlock) Tapped(*fyne.PointEvent) {}

func (l *lockedBlock) TappedSecondary(*fyne.PointEvent) {
	if l.onUnlock != nil {
		l.onUnlock()
	}
}

// BlockEditor renders a file's blocks for Edit Mode.
//
// In free-text mode (Journal/Notes), consecutive decrypted blocks are
// collapsed into a single shared editable text area on entry — matching how
// they'll be re-encrypted as one block on save anyway — so the file reads
// and edits as continuous prose rather than a stack of separate boxes.
// Still-encrypted blocks (a different/old password) remain their own
// greyed, right-clickable placeholder, splitting the text into segments
// around them.
//
// In Todo mode, each block stays its own checkbox+text row (so checking one
// item doesn't touch the others), plus one always-present blank row at the
// end for adding a new item. Enter in any row commits it and moves focus to
// the next row; Enter in the trailing blank row (with text) turns it into a
// new item and a fresh blank row appears below it.
type BlockEditor struct {
	widget.BaseWidget

	win      fyne.Window
	blocks   []*store.Block
	todoMode bool
	codec    *crypto.Codec
	box      *fyne.Container
	onChange func()

	// rowEntries holds the Todo-mode row entries in visual order (including
	// the trailing blank row), used to move focus to "the next row" on
	// Enter. Rebuilt whenever the row structure changes.
	rowEntries []*EnterModeEntry
}

func NewBlockEditor(win fyne.Window, blocks []*store.Block, todoMode bool, codec *crypto.Codec, onChange func()) *BlockEditor {
	e := &BlockEditor{
		win:      win,
		blocks:   blocks,
		todoMode: todoMode,
		codec:    codec,
		box:      container.NewVBox(),
		onChange: onChange,
	}
	if !todoMode {
		e.blocks = collapseDecryptedRuns(blocks)
	}
	e.ExtendBaseWidget(e)
	e.rebuild()
	return e
}

// collapseDecryptedRuns merges consecutive decrypted blocks into a single
// in-memory block (joined by newlines), leaving still-encrypted blocks
// as their own entries in place. This is exactly what Consolidate would do
// at save time anyway; doing it up front means the free-text editor shows
// one continuous box per run instead of one box per originally-typed entry.
func collapseDecryptedRuns(blocks []*store.Block) []*store.Block {
	var out []*store.Block
	var run []*store.Block

	flush := func() {
		if len(run) == 0 {
			return
		}
		if len(run) == 1 {
			out = append(out, run[0])
		} else {
			texts := make([]string, len(run))
			for i, b := range run {
				texts[i] = b.Plaintext
			}
			out = append(out, &store.Block{Plaintext: strings.Join(texts, "\n"), Decrypted: true})
		}
		run = nil
	}

	for _, b := range blocks {
		if b.Decrypted {
			run = append(run, b)
			continue
		}
		flush()
		out = append(out, b)
	}
	flush()
	return out
}

func (e *BlockEditor) changed() {
	if e.onChange != nil {
		e.onChange()
	}
}

func (e *BlockEditor) CreateRenderer() fyne.WidgetRenderer {
	scroll := container.NewVScroll(e.box)
	return widget.NewSimpleRenderer(scroll)
}

func (e *BlockEditor) rebuild() {
	e.box.Objects = nil
	if e.todoMode {
		e.rebuildTodo()
	} else {
		e.rebuildFreeText()
	}
	e.box.Refresh()
}

func (e *BlockEditor) rebuildFreeText() {
	for _, b := range e.blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted {
			e.box.Add(newLockedBlock(func() { e.promptUnlock(b) }))
			continue
		}
		e.box.Add(e.freeTextRow(b))
	}
}

func (e *BlockEditor) freeTextRow(b *store.Block) fyne.CanvasObject {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(b.Plaintext)
	entry.OnChanged = func(t string) {
		b.Plaintext = t
		e.changed()
	}
	return entry
}

func (e *BlockEditor) rebuildTodo() {
	e.rowEntries = nil
	for _, b := range e.blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted {
			e.box.Add(newLockedBlock(func() { e.promptUnlock(b) }))
			continue
		}
		e.box.Add(e.todoRow(b))
	}
	e.box.Add(e.newItemRow())
}

func (e *BlockEditor) todoRow(b *store.Block) fyne.CanvasObject {
	rowIndex := len(e.rowEntries)

	checked := false
	text := b.Plaintext
	if m := todoLineRe.FindStringSubmatch(b.Plaintext); m != nil {
		checked = strings.EqualFold(m[1], "x")
		text = m[2]
	}

	var entry *EnterModeEntry
	entry = NewEnterModeEntry(func(string) { e.advanceFocus(rowIndex) })
	entry.Wrapping = fyne.TextWrapWord
	entry.SetText(text)

	check := widget.NewCheck("", nil)
	check.SetChecked(checked)

	sync := func() {
		mark := " "
		if check.Checked {
			mark = "x"
		}
		if strings.TrimSpace(entry.Text) == "" {
			b.Removed = true
			e.rebuild()
			e.changed()
			return
		}
		b.Plaintext = fmt.Sprintf("- [%s] %s", mark, entry.Text)
		e.changed()
	}
	check.OnChanged = func(bool) { sync() }
	entry.OnChanged = func(string) { sync() }

	e.rowEntries = append(e.rowEntries, entry)
	return container.NewBorder(nil, nil, check, nil, entry)
}

// newItemRow is the always-present blank row at the bottom of a Todo list.
// Typing text and pressing Enter here turns it into a real new item and a
// fresh blank row takes its place.
func (e *BlockEditor) newItemRow() fyne.CanvasObject {
	var entry *EnterModeEntry
	entry = NewEnterModeEntry(func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		e.blocks = append(e.blocks, &store.Block{
			Plaintext: fmt.Sprintf("- [ ] %s", text),
			Decrypted: true,
		})
		e.changed()
		e.rebuild()
		if len(e.rowEntries) > 0 {
			safeFocus(e.win, e.rowEntries[len(e.rowEntries)-1])
		}
	})
	entry.Wrapping = fyne.TextWrapWord
	entry.SetPlaceHolder("New item…")

	check := widget.NewCheck("", nil)
	check.Disable()

	e.rowEntries = append(e.rowEntries, entry)
	return container.NewBorder(nil, nil, check, nil, entry)
}

// advanceFocus moves focus to the row after rowIndex (the next item, or the
// trailing blank "new item" row), if any.
func (e *BlockEditor) advanceFocus(rowIndex int) {
	next := rowIndex + 1
	if next < len(e.rowEntries) {
		safeFocus(e.win, e.rowEntries[next])
	}
}

func (e *BlockEditor) promptUnlock(b *store.Block) {
	ShowPasswordEntry(e.win, "Unlock block", "Password for this block only:", "Unlock", func(pw string) {
		if store.TryUnlock(b, pw, e.codec) {
			e.rebuild()
			e.changed()
		} else {
			ShowInfo(e.win, "Incorrect password", "That password did not unlock this block.")
		}
	}, nil)
}

// Lines returns the consolidated on-disk block lines for this editor's
// current state (dropping any items removed via empty-text editing) ready
// to write with store.WriteLines.
func (e *BlockEditor) Lines(sessionPassword string) ([]string, error) {
	return store.Consolidate(e.blocks, sessionPassword, e.codec)
}

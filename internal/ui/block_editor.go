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

var todoLineRe = regexp.MustCompile(`^- \[([ xX])\] (.*)$`)

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

// todoItem is one checklist line parsed out of a decrypted block's
// markdown. Several items can share one underlying block (chunk) — e.g. a
// chunk that already held multiple items before this editor ever loaded
// it — in which case editing any one of them re-serializes all of that
// chunk's surviving items back into it as a group; new items typed into
// the trailing blank row each get their own fresh chunk.
type todoItem struct {
	blockIdx int
	checked  bool
	text     string
	removed  bool
}

// parseTodoLines splits a block's decrypted plaintext into checklist items.
// A line matching "- [ ] "/"- [x] " starts a new item; any line that
// doesn't (e.g. a Shift+Enter continuation line, or legacy freeform text)
// is appended to the current item so multi-line items and non-conforming
// content both round-trip instead of being silently dropped.
func parseTodoLines(plaintext string) []struct {
	checked bool
	text    string
} {
	var items []struct {
		checked bool
		text    string
	}
	for _, line := range strings.Split(plaintext, "\n") {
		if m := todoLineRe.FindStringSubmatch(line); m != nil {
			items = append(items, struct {
				checked bool
				text    string
			}{checked: strings.EqualFold(m[1], "x"), text: m[2]})
			continue
		}
		if len(items) == 0 {
			items = append(items, struct {
				checked bool
				text    string
			}{checked: false, text: line})
			continue
		}
		items[len(items)-1].text += "\n" + line
	}
	return items
}

// BlockEditor renders a file's blocks for Edit Mode.
//
// In free-text mode (Journal/Notes), consecutive decrypted blocks are
// collapsed into a single shared editable text area on entry — matching how
// they'll be re-encrypted as one block on save anyway — so the file reads
// and edits as continuous prose rather than a stack of separate boxes, and
// that area fills the whole Edit Mode panel.
//
// In Todo mode, each block's decrypted markdown is parsed into one row per
// checklist item (a block may hold more than one), plus one always-present
// blank row at the end for adding a new item. Enter in any row commits it
// and moves focus to the next row; Enter in the trailing blank row (with
// text) turns it into a new item — its own new chunk — and a fresh blank
// row appears below it. Saving re-encrypts each chunk on its own (see
// store.EncryptIndividually) rather than merging them, so items stay
// independently editable/decryptable across sessions.
type BlockEditor struct {
	widget.BaseWidget

	win      fyne.Window
	blocks   []*store.Block
	todoMode bool
	codec    *crypto.Codec
	box      *fyne.Container
	onChange func()

	// items and rowEntries are Todo-mode-only, rebuilt fresh from blocks
	// every rebuild(). rowEntries holds the row entries in visual order
	// (including the trailing blank row), used to move focus to "the next
	// row" on Enter.
	items      []*todoItem
	rowEntries []*EnterModeEntry
}

func NewBlockEditor(win fyne.Window, blocks []*store.Block, todoMode bool, codec *crypto.Codec, onChange func()) *BlockEditor {
	e := &BlockEditor{
		win:      win,
		blocks:   blocks,
		todoMode: todoMode,
		codec:    codec,
		onChange: onChange,
	}
	if todoMode {
		e.box = container.NewVBox()
	} else {
		e.blocks = collapseDecryptedRuns(blocks)
		// NewStack (unlike VBox) stretches its content to fill all
		// available space, which is what a continuous prose editor needs.
		e.box = container.NewStack()
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
	if e.todoMode {
		return widget.NewSimpleRenderer(container.NewVScroll(e.box))
	}
	return widget.NewSimpleRenderer(e.box)
}

func (e *BlockEditor) rebuild() {
	if e.todoMode {
		e.rebuildTodo()
		return
	}
	e.rebuildFreeText()
}

func (e *BlockEditor) rebuildFreeText() {
	var segments []fyne.CanvasObject
	for _, b := range e.blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted {
			b := b
			segments = append(segments, newLockedBlock(func() { e.promptUnlock(b) }))
			continue
		}
		segments = append(segments, e.freeTextRow(b))
	}
	if len(segments) == 0 {
		// Brand new, never-written-to file: still show one empty editable
		// block rather than a blank panel with nothing to type into.
		nb := &store.Block{Decrypted: true}
		e.blocks = append(e.blocks, nb)
		segments = append(segments, e.freeTextRow(nb))
	}

	var content fyne.CanvasObject
	if len(segments) == 1 {
		content = segments[0]
	} else {
		// Only the last segment stretches to fill remaining space; earlier
		// segments (rare — only happen around a locked block) sit above it
		// at their natural size.
		top := container.NewVBox(segments[:len(segments)-1]...)
		content = container.NewBorder(top, nil, nil, nil, segments[len(segments)-1])
	}
	e.box.Objects = []fyne.CanvasObject{content}
	e.box.Refresh()
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
	e.items = nil
	e.rowEntries = nil
	e.box.Objects = nil

	for bi, b := range e.blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted {
			b := b
			e.box.Add(newLockedBlock(func() { e.promptUnlock(b) }))
			continue
		}
		for _, line := range parseTodoLines(b.Plaintext) {
			item := &todoItem{blockIdx: bi, checked: line.checked, text: line.text}
			e.items = append(e.items, item)
			e.box.Add(e.todoItemRow(item))
		}
	}
	e.box.Add(e.newItemRow())
	e.box.Refresh()
}

// todoRowLines returns how many rows a todo item's entry should reserve —
// one per newline in its text (Shift+Enter continuation lines) — so a
// single-line item gets a single-line box instead of Entry's fixed 3-row
// multiline default.
func todoRowLines(text string) int {
	return strings.Count(text, "\n") + 1
}

func (e *BlockEditor) todoItemRow(item *todoItem) fyne.CanvasObject {
	rowIndex := len(e.rowEntries)

	var entry *EnterModeEntry
	entry = NewEnterModeEntry(func(string) { e.advanceFocus(rowIndex) })
	// Word-wrap forces Entry to wrap itself in an internal scroll container
	// (see widget/entry.go) that swallows mouse-wheel events even when
	// there's nothing to scroll, which is what was blocking the outer Todo
	// list's own scrollbar. Todo items are short lines, not prose, so we
	// don't need wrapping — only Shift+Enter's explicit newlines.
	entry.Wrapping = fyne.TextWrapOff
	entry.SetText(item.text)
	entry.SetMinRowsVisible(todoRowLines(item.text))

	check := widget.NewCheck("", nil)
	check.SetChecked(item.checked)

	sync := func() {
		item.checked = check.Checked
		item.text = entry.Text
		entry.SetMinRowsVisible(todoRowLines(entry.Text))
		e.box.Refresh()
		if strings.TrimSpace(item.text) == "" {
			item.removed = true
			e.syncBlock(item.blockIdx)
			e.rebuild()
			e.changed()
			return
		}
		e.syncBlock(item.blockIdx)
		e.changed()
	}
	check.OnChanged = func(bool) { sync() }
	entry.OnChanged = func(string) { sync() }

	e.rowEntries = append(e.rowEntries, entry)
	return container.NewBorder(nil, nil, check, nil, entry)
}

// syncBlock re-serializes every surviving item belonging to blockIdx back
// into that block's Plaintext (preserving multi-item chunks as a group),
// or marks the block Removed if none survive.
func (e *BlockEditor) syncBlock(blockIdx int) {
	var lines []string
	for _, it := range e.items {
		if it.blockIdx != blockIdx || it.removed {
			continue
		}
		mark := " "
		if it.checked {
			mark = "x"
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", mark, it.text))
	}
	if len(lines) == 0 {
		e.blocks[blockIdx].Removed = true
		return
	}
	e.blocks[blockIdx].Plaintext = strings.Join(lines, "\n")
}

// newItemRow is the always-present blank row at the bottom of a Todo list.
// Typing text and pressing Enter here turns it into a real new item (its
// own new chunk) and a fresh blank row takes its place.
func (e *BlockEditor) newItemRow() fyne.CanvasObject {
	entry := NewEnterModeEntry(func(text string) {
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
			deferFocus(e.win, e.rowEntries[len(e.rowEntries)-1])
		}
	})
	entry.Wrapping = fyne.TextWrapOff
	entry.SetPlaceHolder("New item…")
	entry.SetMinRowsVisible(1)
	entry.OnChanged = func(t string) {
		entry.SetMinRowsVisible(todoRowLines(t))
		e.box.Refresh()
	}

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

// Lines returns the on-disk block lines for this editor's current state,
// ready to write with store.WriteLines. Todo mode never merges chunks
// together (see store.EncryptIndividually); free-text mode consolidates as
// usual.
func (e *BlockEditor) Lines(sessionPassword string) ([]string, error) {
	if e.todoMode {
		return store.EncryptIndividually(e.blocks, sessionPassword, e.codec)
	}
	return store.Consolidate(e.blocks, sessionPassword, e.codec)
}

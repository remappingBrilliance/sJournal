package ui

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"sjournal/internal/crypto"
	"sjournal/internal/store"
)

// todoLineRe matches a checklist line once its leading indentation has been
// stripped (the indent is read separately, via store.IndentDepth).
var todoLineRe = regexp.MustCompile(`^- \[([ xX])\] (.*)$`)

// completedDateRe matches the trailing "<!-- c:YYYY-MM-DD -->" marker Edit
// Mode appends to a checked item's on-disk text so the completion date
// round-trips without cluttering the editable text itself.
var completedDateRe = regexp.MustCompile(`[ \t]+<!--[ \t]*c:(\d{4}-\d{2}-\d{2})[ \t]*-->[ \t]*$`)

// splitCompletedDate strips a trailing completed-date marker from text,
// returning the cleaned text and the date (empty if there was no marker).
func splitCompletedDate(text string) (string, string) {
	if m := completedDateRe.FindStringSubmatch(text); m != nil {
		return text[:len(text)-len(m[0])], m[1]
	}
	return text, ""
}

// todoMaxDepth caps how far a checklist item can be nested.
const todoMaxDepth = 8

// todoIndentPx is the on-screen width of one nesting level in Edit Mode.
const todoIndentPx = 24

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

// todoItem is one checklist item — its own block on disk. depth is its
// nesting level (0 = top). text may carry embedded newlines from Shift+Enter
// continuation lines; the "[ ]"/"[x]" marker applies to the whole item.
// completedDate (YYYY-MM-DD, store.JournalDateLayout) is set the moment the
// item is checked and cleared the moment it's unchecked; it round-trips as a
// trailing "<!-- c:... -->" marker on the item's on-disk line.
type todoItem struct {
	depth         int
	checked       bool
	text          string
	completedDate string
}

// todoRow is one visible row of the Todo editor: exactly one of item (a
// checklist item) or block (a heading block, or a still-encrypted block
// shown greyed-out) is set.
type todoRow struct {
	item  *todoItem
	block *store.Block
}

// parseTodoItems splits a decrypted block's plaintext into checklist items,
// reading each item's nesting depth from its leading indentation. A line
// that isn't a "- [ ] "/"- [x] " line (a Shift+Enter continuation, or
// legacy freeform text) is appended to the current item so multi-line items
// and non-conforming content both round-trip instead of being dropped.
func parseTodoItems(plaintext string) []*todoItem {
	var items []*todoItem
	for _, line := range strings.Split(plaintext, "\n") {
		body := strings.TrimLeft(line, " \t")
		if m := todoLineRe.FindStringSubmatch(body); m != nil {
			items = append(items, &todoItem{
				depth:   min(store.IndentDepth(line), todoMaxDepth),
				checked: strings.EqualFold(m[1], "x"),
				text:    m[2],
			})
			continue
		}
		if len(items) == 0 {
			items = append(items, &todoItem{depth: min(store.IndentDepth(line), todoMaxDepth), text: body})
			continue
		}
		items[len(items)-1].text += "\n" + body
	}
	for _, it := range items {
		it.text, it.completedDate = splitCompletedDate(it.text)
	}
	return items
}

// formatTodoItem renders an item back to its on-disk line(s): the indent for
// its depth, the "- [x] " marker, then the text (continuation lines indented
// to sit under the marker).
func formatTodoItem(it *todoItem) string {
	mark := " "
	if it.checked {
		mark = "x"
	}
	indent := strings.Repeat(store.TodoIndentUnit, it.depth)
	lines := strings.Split(it.text, "\n")
	lines[0] = fmt.Sprintf("%s- [%s] %s", indent, mark, lines[0])
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + "  " + lines[i]
	}
	out := strings.Join(lines, "\n")
	if it.checked && it.completedDate != "" {
		out += fmt.Sprintf(" <!-- c:%s -->", it.completedDate)
	}
	return out
}

// BlockEditor renders a file's blocks for Edit Mode.
//
// In free-text mode (Journal/Notes), consecutive decrypted blocks are
// collapsed into a single shared editable text area on entry — matching how
// they'll be re-encrypted as one block on save anyway — so the file reads
// and edits as continuous prose rather than a stack of separate boxes, and
// that area fills the whole Edit Mode panel.
//
// In Todo mode, each checklist item is its own block. A decrypted block that
// holds several "- [ ] " lines is split into one item each on load, so items
// are independently editable and every save writes one item per block (see
// store.EncryptIndividually — no chunk merging). Items nest: leading
// indentation on disk ⇄ a depth per item. Tab / Shift+Tab at the start of an
// item indents / outdents it; Enter at the end of an item opens a new
// sibling below at the same depth. A lone "# Title" block is shown as the
// list heading, not an item. Still-encrypted blocks stay greyed-out in place.
type BlockEditor struct {
	widget.BaseWidget

	win      fyne.Window
	blocks   []*store.Block // free-text mode only
	todoMode bool
	codec    *crypto.Codec
	box      *fyne.Container
	onChange func()

	// Todo-mode only: rows is the ordered visible model; itemEntry maps each
	// live item to its entry widget so focus can be restored after a rebuild;
	// newItemEntry is the trailing "New item…" field; newItemDepth is that
	// field's pending nesting level (Tab/Shift+Tab before the item exists).
	// hideCompleted/hideCompletedCheck back the "Hide completed" toggle above
	// the list — checked items stay in rows (and are still saved) either way,
	// this only affects what rebuildTodo renders.
	rows               []*todoRow
	itemEntry          map[*todoItem]*EnterModeEntry
	itemCheck          map[*todoItem]*widget.Check
	newItemEntry       *EnterModeEntry
	newItemDepth       int
	hideCompleted      bool
	hideCompletedCheck *widget.Check
}

func NewBlockEditor(win fyne.Window, blocks []*store.Block, todoMode bool, codec *crypto.Codec, onChange func()) *BlockEditor {
	e := &BlockEditor{
		win:      win,
		todoMode: todoMode,
		codec:    codec,
		onChange: onChange,
	}
	if todoMode {
		e.box = container.NewVBox()
		e.rows = buildTodoRows(blocks)
		e.hideCompletedCheck = widget.NewCheck("Hide completed", func(checked bool) {
			e.hideCompleted = checked
			e.rebuildTodo()
		})
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

// buildTodoRows expands a file's blocks into the editor's row model: heading
// and still-encrypted blocks pass through as-is; every other decrypted block
// is split into one row per checklist item.
func buildTodoRows(blocks []*store.Block) []*todoRow {
	var rows []*todoRow
	for _, b := range blocks {
		if b.Removed {
			continue
		}
		if !b.Decrypted || store.IsHeadingLine(b.Plaintext) {
			rows = append(rows, &todoRow{block: b})
			continue
		}
		for _, it := range parseTodoItems(b.Plaintext) {
			rows = append(rows, &todoRow{item: it})
		}
	}
	return rows
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

// AppendExternalBlock folds a line written to the file outside this editor
// (a reminder window's entry, while Edit Mode was open on the same file)
// into the in-memory model, so saving Edit Mode — a full-file rewrite —
// doesn't drop it. Free-text (Journal/Notes) only — the reminder never
// writes to a Todo file. It extends the last decrypted run so the file still
// reads as one block; in-progress edits to existing blocks are preserved
// (OnChanged mirrors them synchronously), though the rebuild moves focus.
func (e *BlockEditor) AppendExternalBlock(plaintext string) {
	for i := len(e.blocks) - 1; i >= 0; i-- {
		b := e.blocks[i]
		if b.Removed || !b.Decrypted {
			continue
		}
		if b.Plaintext == "" {
			b.Plaintext = plaintext
		} else {
			b.Plaintext += "\n" + plaintext
		}
		e.rebuild()
		e.changed()
		return
	}
	e.blocks = append(e.blocks, &store.Block{Plaintext: plaintext, Decrypted: true})
	e.rebuild()
	e.changed()
}

func (e *BlockEditor) changed() {
	if e.onChange != nil {
		e.onChange()
	}
}

func (e *BlockEditor) CreateRenderer() fyne.WidgetRenderer {
	if e.todoMode {
		return widget.NewSimpleRenderer(container.NewBorder(
			e.hideCompletedCheck, nil, nil, nil, container.NewVScroll(e.box)))
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
	e.normalizeTodoRows()
	e.itemEntry = map[*todoItem]*EnterModeEntry{}
	e.itemCheck = map[*todoItem]*widget.Check{}
	e.box.Objects = nil

	for _, r := range e.rows {
		switch {
		case r.item != nil:
			if e.hideCompleted && r.item.checked {
				continue
			}
			e.box.Add(e.todoItemRow(r.item))
		case !r.block.Decrypted:
			b := r.block
			e.box.Add(newLockedBlock(func() { e.promptUnlock(b) }))
		default:
			e.box.Add(e.headingRow(r.block))
		}
	}
	e.box.Add(e.newItemRow())
	e.box.Refresh()
}

// normalizeTodoRows re-expands any row whose block became a decrypted,
// non-heading block since the last rebuild (a locked block the user just
// unlocked) into its checklist items.
func (e *BlockEditor) normalizeTodoRows() {
	var out []*todoRow
	for _, r := range e.rows {
		if r.item == nil && r.block != nil && r.block.Decrypted &&
			!r.block.Removed && !store.IsHeadingLine(r.block.Plaintext) {
			for _, it := range parseTodoItems(r.block.Plaintext) {
				out = append(out, &todoRow{item: it})
			}
			continue
		}
		if r.block != nil && r.block.Removed {
			continue
		}
		out = append(out, r)
	}
	e.rows = out
}

// todoRowLines returns how many rows a todo item's entry should reserve —
// one per newline in its text (Shift+Enter continuation lines) — so a
// single-line item gets a single-line box instead of Entry's fixed 3-row
// multiline default.
func todoRowLines(text string) int {
	return strings.Count(text, "\n") + 1
}

func (e *BlockEditor) todoItemRow(item *todoItem) fyne.CanvasObject {
	var entry *EnterModeEntry
	entry = NewEnterModeEntry(func(string) { e.openSiblingAfter(item) })
	// Word-wrap forces Entry to wrap itself in an internal scroll container
	// (see widget/entry.go) that swallows mouse-wheel events even when
	// there's nothing to scroll, which is what was blocking the outer Todo
	// list's own scrollbar. Todo items are short lines, not prose, so we
	// don't need wrapping — only Shift+Enter's explicit newlines.
	entry.Wrapping = fyne.TextWrapOff
	entry.SetText(item.text)
	entry.SetMinRowsVisible(todoRowLines(item.text))
	// Tab / Shift+Tab at the very start of the item nests / un-nests it;
	// elsewhere it's swallowed (a literal tab in an item helps nobody).
	entry.OnTab = func(shift bool) bool {
		if entry.CursorRow == 0 && entry.CursorColumn == 0 {
			e.shiftDepth(item, shift)
		}
		return true
	}

	check := widget.NewCheck("", nil)
	check.SetChecked(item.checked)

	// Checking/unchecking stamps or clears the completion date; it never
	// needs to touch item.text, so it's kept separate from the entry's
	// OnChanged (below) which fires on every keystroke and must not rebuild.
	// A full rebuild (rather than box.Refresh()) is needed either way: the
	// completed-date tag beside the entry only exists on checked rows, so it
	// has to be added/removed, not just repainted.
	check.OnChanged = func(checked bool) {
		item.checked = checked
		if checked {
			if item.completedDate == "" {
				item.completedDate = time.Now().Format(store.JournalDateLayout)
			}
		} else {
			item.completedDate = ""
		}
		e.changed()
		e.rebuildTodo()
	}
	entry.OnChanged = func(string) {
		item.text = entry.Text
		entry.SetMinRowsVisible(todoRowLines(entry.Text))
		e.box.Refresh()
		if strings.TrimSpace(item.text) == "" {
			e.removeItem(item)
			e.changed()
			return
		}
		e.changed()
	}

	e.itemEntry[item] = entry
	e.itemCheck[item] = check

	indent := canvas.NewRectangle(color.Transparent)
	indent.SetMinSize(fyne.NewSize(float32(item.depth)*todoIndentPx, 1))

	var tag fyne.CanvasObject
	if item.checked && item.completedDate != "" {
		tag = e.completedDateTag(item.completedDate)
	}
	return container.NewBorder(nil, nil, container.NewHBox(indent, check), tag, entry)
}

// completedDateTag is the small grey "completed on" marker shown to the
// right of a checked item's entry. It's always visible (unlike a
// hover-only hint, which turned out to be unreliable — widget.Check already
// implements desktop.Hoverable for its own hover highlight, and the hit-test
// walk lets the deepest matching object win, so a hoverTooltip wrapped
// around the checkbox itself never received the mouse events). Hovering the
// tag itself — a plain widget.Label, which isn't Hoverable — still shows the
// same date in a small popup.
func (e *BlockEditor) completedDateTag(date string) fyne.CanvasObject {
	label := widget.NewLabel(date)
	label.Importance = widget.LowImportance
	return newHoverTooltip(e.win, label, func() string { return "Completed " + date })
}

// hoverTooltip wraps a CanvasObject and, while the pointer is over it, shows
// a small popup with whatever textFunc currently returns (nothing is shown
// when it returns ""). Used to surface a Todo item's completion date without
// giving it a permanently-visible label.
type hoverTooltip struct {
	widget.BaseWidget
	content  fyne.CanvasObject
	textFunc func() string
	win      fyne.Window
	popup    *widget.PopUp
}

func newHoverTooltip(win fyne.Window, content fyne.CanvasObject, textFunc func() string) *hoverTooltip {
	h := &hoverTooltip{content: content, textFunc: textFunc, win: win}
	h.ExtendBaseWidget(h)
	return h
}

func (h *hoverTooltip) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(h.content)
}

func (h *hoverTooltip) MouseIn(ev *desktop.MouseEvent) {
	text := h.textFunc()
	if text == "" {
		return
	}
	h.popup = widget.NewPopUp(widget.NewLabel(text), h.win.Canvas())
	h.popup.ShowAtPosition(ev.AbsolutePosition.Add(fyne.NewPos(12, 12)))
}

func (h *hoverTooltip) MouseMoved(*desktop.MouseEvent) {}

func (h *hoverTooltip) MouseOut() {
	if h.popup != nil {
		h.popup.Hide()
		h.popup = nil
	}
}

// rowIndexOfItem returns item's position in e.rows, or -1.
func (e *BlockEditor) rowIndexOfItem(item *todoItem) int {
	for i, r := range e.rows {
		if r.item == item {
			return i
		}
	}
	return -1
}

// shiftDepth indents (shift=false) or outdents (shift=true) item by one
// level, clamped to [0, one deeper than the row above], and keeps focus on it.
func (e *BlockEditor) shiftDepth(item *todoItem, outdent bool) {
	old := item.depth
	if outdent {
		if item.depth > 0 {
			item.depth--
		}
	} else if item.depth < e.maxDepthFor(item) {
		item.depth++
	}
	if item.depth == old {
		return
	}
	e.changed()
	e.rebuild()
	e.focusItem(item)
}

// focusItem moves focus to item's entry. Unlike deferFocus it doesn't wait
// for a paint pass: the focus manager walks the live widget tree, so an
// entry added to e.box this same tick is already reachable. It only needs
// the editor itself to be on a canvas — which isn't true during the very
// first build (before the window's SetContent), so fall back to deferFocus
// then.
func (e *BlockEditor) focusItem(item *todoItem) {
	entry := e.itemEntry[item]
	if entry == nil {
		return
	}
	if fyne.CurrentApp().Driver().CanvasForObject(e) == nil {
		deferFocus(e.win, entry)
		return
	}
	e.win.Canvas().Focus(entry)
}

// maxDepthFor is how deep item is allowed to nest: one past the nearest
// item row above it (so a list can't sprout an orphan indent), and never
// past todoMaxDepth.
func (e *BlockEditor) maxDepthFor(item *todoItem) int {
	limit := 0
	idx := e.rowIndexOfItem(item)
	for i := idx - 1; i >= 0; i-- {
		if prev := e.rows[i].item; prev != nil {
			limit = prev.depth + 1
			break
		}
	}
	return min(limit, todoMaxDepth)
}

// openSiblingAfter inserts a fresh empty item just below item at the same
// depth and moves focus to it — Enter at the end of a checklist line.
func (e *BlockEditor) openSiblingAfter(item *todoItem) {
	if strings.TrimSpace(item.text) == "" {
		return // don't spawn empties off an empty row
	}
	idx := e.rowIndexOfItem(item)
	if idx < 0 {
		return
	}
	ni := &todoItem{depth: item.depth}
	rows := make([]*todoRow, 0, len(e.rows)+1)
	rows = append(rows, e.rows[:idx+1]...)
	rows = append(rows, &todoRow{item: ni})
	rows = append(rows, e.rows[idx+1:]...)
	e.rows = rows
	e.changed()
	e.rebuild()
	e.focusItem(ni)
}

// removeItem drops item's row and focuses the nearest item row above it.
func (e *BlockEditor) removeItem(item *todoItem) {
	idx := e.rowIndexOfItem(item)
	if idx < 0 {
		return
	}
	e.rows = append(e.rows[:idx], e.rows[idx+1:]...)
	delete(e.itemEntry, item)
	delete(e.itemCheck, item)
	e.rebuild()
	for i := idx - 1; i >= 0; i-- {
		if prev := e.rows[i].item; prev != nil {
			e.focusItem(prev)
			return
		}
	}
}

// headingRow renders a Todo file's heading block ("# Title") as a bold,
// single-line editable field with no checkbox. Clearing it drops the block.
func (e *BlockEditor) headingRow(b *store.Block) fyne.CanvasObject {
	entry := NewEnterModeEntry(nil)
	entry.Wrapping = fyne.TextWrapOff
	entry.TextStyle = fyne.TextStyle{Bold: true}
	entry.SetText(b.Plaintext)
	entry.SetMinRowsVisible(1)
	entry.OnChanged = func(text string) {
		b.Plaintext = text
		if strings.TrimSpace(text) == "" {
			b.Removed = true
			e.rebuild()
		}
		e.changed()
	}
	return entry
}

// newItemRow is the always-present blank row at the bottom of a Todo list.
// Enter here adds a new top-level item (or, for "# Title", a heading);
// leading whitespace nests it, and a fresh blank row takes its place.
func (e *BlockEditor) newItemRow() fyne.CanvasObject {
	entry := NewEnterModeEntry(func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		var r *todoRow
		if store.IsHeadingLine(text) {
			r = &todoRow{block: &store.Block{Plaintext: strings.TrimSpace(text), Decrypted: true}}
		} else {
			depth := max(e.newItemDepth, min(store.IndentDepth(text), todoMaxDepth))
			r = &todoRow{item: &todoItem{
				depth: depth,
				text:  strings.TrimLeft(text, " \t"),
			}}
		}
		e.rows = append(e.rows, r)
		e.newItemDepth = 0
		e.changed()
		e.rebuild()
		// Keep focus on this box so items can be added one after another.
		if e.newItemEntry != nil && fyne.CurrentApp().Driver().CanvasForObject(e) != nil {
			e.win.Canvas().Focus(e.newItemEntry)
		}
	})
	entry.Wrapping = fyne.TextWrapOff
	entry.SetPlaceHolder("New item… (or “# Heading”)")
	entry.SetMinRowsVisible(1)
	// Tab / Shift+Tab at the start nests / un-nests the item about to be
	// created — same rule as an existing item (see todoItemRow's OnTab).
	// Without this it fell through to the embedded Entry's default handling
	// and just inserted a literal tab character instead of indenting.
	entry.OnTab = func(shift bool) bool {
		if entry.CursorRow == 0 && entry.CursorColumn == 0 {
			e.shiftNewItemDepth(shift)
		}
		return true
	}
	e.newItemEntry = entry
	entry.OnChanged = func(t string) {
		entry.SetMinRowsVisible(todoRowLines(t))
		e.box.Refresh()
	}

	check := widget.NewCheck("", nil)
	check.Disable()

	indent := canvas.NewRectangle(color.Transparent)
	indent.SetMinSize(fyne.NewSize(float32(e.newItemDepth)*todoIndentPx, 1))
	return container.NewBorder(nil, nil, container.NewHBox(indent, check), nil, entry)
}

// shiftNewItemDepth indents (shift=false) or outdents (shift=true) the
// pending "new item" row by one level — mirrors shiftDepth, but the item
// doesn't exist in e.rows yet, so the clamp is against the last item
// currently in the list instead of the row above a real todoItem.
func (e *BlockEditor) shiftNewItemDepth(outdent bool) {
	old := e.newItemDepth
	if outdent {
		if e.newItemDepth > 0 {
			e.newItemDepth--
		}
	} else if e.newItemDepth < e.maxNewItemDepth() {
		e.newItemDepth++
	}
	if e.newItemDepth == old {
		return
	}
	e.rebuild()
	if e.newItemEntry != nil && fyne.CurrentApp().Driver().CanvasForObject(e) != nil {
		e.win.Canvas().Focus(e.newItemEntry)
	}
}

// maxNewItemDepth is how deep the pending new item may nest: one past the
// last item currently in the list (mirrors maxDepthFor), and never past
// todoMaxDepth.
func (e *BlockEditor) maxNewItemDepth() int {
	for i := len(e.rows) - 1; i >= 0; i-- {
		if prev := e.rows[i].item; prev != nil {
			return min(prev.depth+1, todoMaxDepth)
		}
	}
	return 0
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
// ready to write with store.WriteLines. Todo mode writes one block per
// checklist item / heading (see store.EncryptIndividually — no merging);
// free-text mode consolidates as usual.
func (e *BlockEditor) Lines(sessionPassword string) ([]string, error) {
	if e.todoMode {
		return store.EncryptIndividually(e.todoBlocks(), sessionPassword, e.codec)
	}
	return store.Consolidate(e.blocks, sessionPassword, e.codec)
}

// todoBlocks flattens the row model back to blocks for saving: one block per
// non-empty item (indented "- [x] " line), heading blocks and still-locked
// blocks passed through unchanged.
func (e *BlockEditor) todoBlocks() []*store.Block {
	var bs []*store.Block
	for _, r := range e.rows {
		if r.item != nil {
			if strings.TrimSpace(r.item.text) == "" {
				continue
			}
			bs = append(bs, &store.Block{Plaintext: formatTodoItem(r.item), Decrypted: true})
			continue
		}
		if r.block != nil && !r.block.Removed {
			bs = append(bs, r.block)
		}
	}
	return bs
}

// todoItems returns the current checklist items in order (test/debug helper).
func (e *BlockEditor) todoItems() []*todoItem {
	var items []*todoItem
	for _, r := range e.rows {
		if r.item != nil {
			items = append(items, r.item)
		}
	}
	return items
}

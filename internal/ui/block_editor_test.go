package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"sjournal/internal/crypto"
	"sjournal/internal/store"
)

func newTodoEditor(t *testing.T, codec *crypto.Codec, pw string, plaintexts ...string) (*BlockEditor, fyne.Window) {
	t.Helper()
	test.NewApp()
	w := test.NewWindow(widget.NewLabel("bg"))
	t.Cleanup(w.Close)

	blocks := make([]*store.Block, len(plaintexts))
	for i, p := range plaintexts {
		raw, _ := codec.Encrypt(pw, p)
		blocks[i] = &store.Block{Raw: raw, Plaintext: p, Decrypted: true}
	}
	ed := NewBlockEditor(w, blocks, true, codec, func() {})
	w.SetContent(ed)
	w.Resize(fyne.NewSize(500, 500))
	return ed, w
}

// decrypts every line of ed.Lines back to plaintext.
func savedPlaintext(t *testing.T, ed *BlockEditor, codec *crypto.Codec, pw string) []string {
	t.Helper()
	lines, err := ed.Lines(pw)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		pt, err := codec.Decrypt(pw, l)
		if err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		out[i] = pt
	}
	return out
}

// TestBlockEditor_TodoHeadingRoundTrips: a "# Title" block in a Todo file is
// shown as a heading (not a checklist item) and written back verbatim.
func TestBlockEditor_TodoHeadingRoundTrips(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "# My List", "- [ ] buy milk")

	if items := ed.todoItems(); len(items) != 1 || items[0].text != "buy milk" {
		t.Fatalf("heading leaked into items: %+v", items)
	}
	got := savedPlaintext(t, ed, codec, pw)
	if len(got) != 2 || got[0] != "# My List" || got[1] != "- [ ] buy milk" {
		t.Fatalf("round-trip changed the file: %q", got)
	}
}

// TestBlockEditor_TodoNestingRoundTrips: indented checklist lines load with
// the right depth and are written back with 4-space-per-level indentation.
func TestBlockEditor_TodoNestingRoundTrips(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw,
		"- [ ] Main\n   - [ ] Sub Item\n        - [ ] Sub-sub item.",
	)

	items := ed.todoItems()
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d: %+v", len(items), items)
	}
	for i, want := range []int{0, 1, 2} {
		if items[i].depth != want {
			t.Errorf("item %d depth = %d, want %d", i, items[i].depth, want)
		}
	}

	// One multi-item block in → one block per item out, normalised indent.
	got := savedPlaintext(t, ed, codec, pw)
	want := []string{"- [ ] Main", "    - [ ] Sub Item", "        - [ ] Sub-sub item."}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("saved:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestBlockEditor_TabIndents: Tab at the start of an item indents it (capped
// at one past the row above), Shift+Tab outdents it (capped at 0).
func TestBlockEditor_TabIndents(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [ ] Main", "- [ ] Second")

	main := ed.todoItems()[0]
	second := ed.todoItems()[1]

	// indent "Second" under "Main"
	e := ed.itemEntry[second]
	e.CursorRow, e.CursorColumn = 0, 0
	if !e.OnTab(false) {
		t.Fatal("OnTab(false) at col 0 should consume the key")
	}
	second = ed.todoItems()[1]
	if second.depth != 1 {
		t.Fatalf("after Tab, depth = %d, want 1", second.depth)
	}

	// can't go deeper than main+1
	e = ed.itemEntry[second]
	e.CursorRow, e.CursorColumn = 0, 0
	e.OnTab(false)
	if ed.todoItems()[1].depth != 1 {
		t.Fatalf("Tab past parent+1 should be a no-op, depth = %d", ed.todoItems()[1].depth)
	}

	// Shift+Tab back to 0, then clamp
	e = ed.itemEntry[ed.todoItems()[1]]
	e.CursorRow, e.CursorColumn = 0, 0
	e.OnTab(true)
	e = ed.itemEntry[ed.todoItems()[1]]
	e.CursorRow, e.CursorColumn = 0, 0
	e.OnTab(true)
	if ed.todoItems()[1].depth != 0 {
		t.Fatalf("Shift+Tab should clamp at 0, depth = %d", ed.todoItems()[1].depth)
	}

	// Tab away from column 0 changes nothing (and never inserts a literal tab).
	e = ed.itemEntry[main]
	e.CursorRow, e.CursorColumn = 0, 3
	e.OnTab(false)
	if ed.todoItems()[0].depth != 0 || strings.Contains(ed.todoItems()[0].text, "\t") {
		t.Fatalf("Tab mid-item changed something: %+v", ed.todoItems()[0])
	}
}

// TestBlockEditor_EnterOpensSiblingAtSameDepth: Enter at the end of an item
// inserts a new empty item right below it at the same indent.
func TestBlockEditor_EnterOpensSiblingAtSameDepth(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, w := newTodoEditor(t, codec, pw, "- [ ] Main\n    - [ ] Sub")

	sub := ed.todoItems()[1]
	ed.itemEntry[sub].TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	items := ed.todoItems()
	if len(items) != 3 {
		t.Fatalf("Enter should have inserted a row: %d items", len(items))
	}
	if items[2].depth != 1 || strings.TrimSpace(items[2].text) != "" {
		t.Fatalf("new sibling = %+v, want empty at depth 1", items[2])
	}
	// Focus must land on the new item's entry — synchronously, no paint wait.
	if got := w.Canvas().Focused(); got != fyne.Focusable(ed.itemEntry[items[2]]) {
		t.Fatalf("focus did not move to the new item (focused=%p, want=%p)", got, ed.itemEntry[items[2]])
	}

	// The empty row is dropped on save.
	got := savedPlaintext(t, ed, codec, pw)
	if len(got) != 2 {
		t.Fatalf("empty sibling not dropped on save: %q", got)
	}

	// Enter on an empty item does nothing.
	ed.itemEntry[ed.todoItems()[2]].TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if len(ed.todoItems()) != 3 {
		t.Fatalf("Enter on an empty item spawned another: %d", len(ed.todoItems()))
	}
}

// TestBlockEditor_TodoMultiItemBlockSplits: a legacy block holding several
// checklist lines is split so each item becomes its own block on save.
func TestBlockEditor_TodoMultiItemBlockSplits(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [ ] one\n- [x] two\n- [ ] three")

	if got := savedPlaintext(t, ed, codec, pw); len(got) != 3 {
		t.Fatalf("want 3 blocks after split, got %d: %q", len(got), got)
	}
}

// TestBlockEditor_CheckStampsCompletedDate: ticking a checkbox writes a
// "<!-- c:YYYY-MM-DD -->" marker, and the item's editable text stays clean.
func TestBlockEditor_CheckStampsCompletedDate(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [ ] buy milk")

	item := ed.todoItems()[0]
	check := ed.itemCheck[item]
	check.SetChecked(true)

	if item.text != "buy milk" {
		t.Fatalf("checking should not touch item text, got %q", item.text)
	}
	today := time.Now().Format(store.JournalDateLayout)
	if item.completedDate != today {
		t.Fatalf("completedDate = %q, want %q", item.completedDate, today)
	}

	got := savedPlaintext(t, ed, codec, pw)
	want := fmt.Sprintf("- [x] buy milk <!-- c:%s -->", today)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("saved = %q, want [%q]", got, want)
	}

	// Unticking clears the marker.
	check.SetChecked(false)
	if item.completedDate != "" {
		t.Fatalf("unchecking should clear completedDate, got %q", item.completedDate)
	}
	got = savedPlaintext(t, ed, codec, pw)
	if len(got) != 1 || got[0] != "- [ ] buy milk" {
		t.Fatalf("saved after uncheck = %q, want [\"- [ ] buy milk\"]", got)
	}
}

// TestBlockEditor_CompletedDateRoundTrips: a marker already on disk loads
// into completedDate and is written back unchanged without further edits.
func TestBlockEditor_CompletedDateRoundTrips(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [x] buy milk <!-- c:2026-01-02 -->")

	item := ed.todoItems()[0]
	if item.text != "buy milk" || item.completedDate != "2026-01-02" {
		t.Fatalf("item = %+v, want text=%q completedDate=%q", item, "buy milk", "2026-01-02")
	}
	got := savedPlaintext(t, ed, codec, pw)
	if len(got) != 1 || got[0] != "- [x] buy milk <!-- c:2026-01-02 -->" {
		t.Fatalf("saved = %q, unchanged round-trip expected", got)
	}
}

// TestBlockEditor_HideCompletedTogglesVisibility: the "Hide completed" check
// removes checked items from the rendered rows without dropping them from
// the saved file.
func TestBlockEditor_HideCompletedTogglesVisibility(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [ ] one\n- [x] two")

	done := ed.todoItems()[1]
	if ed.itemEntry[done] == nil {
		t.Fatal("completed item should be visible before hiding")
	}

	ed.hideCompletedCheck.SetChecked(true)
	if ed.itemEntry[done] != nil {
		t.Fatal("completed item should be hidden from rendered rows")
	}
	if len(ed.todoItems()) != 2 {
		t.Fatalf("hiding must not drop the item from the model: %d items", len(ed.todoItems()))
	}
	if got := savedPlaintext(t, ed, codec, pw); len(got) != 2 {
		t.Fatalf("hidden completed item must still save: %q", got)
	}

	ed.hideCompletedCheck.SetChecked(false)
	if ed.itemEntry[done] == nil {
		t.Fatal("unhiding should restore the completed item's row")
	}
}

// TestBlockEditor_CompletedDateTagVisibility: the always-visible grey date
// tag appears the moment an item is checked and disappears the moment it's
// unchecked — this is the always-on complement to the hover tooltip (hover
// alone turned out to be unreliable, since widget.Check already implements
// desktop.Hoverable for its own hover highlight and shadows a wrapper placed
// directly around it).
func TestBlockEditor_CompletedDateTagVisibility(t *testing.T) {
	codec := crypto.NewCodec()
	const pw = "pw"
	ed, _ := newTodoEditor(t, codec, pw, "- [ ] buy milk")

	item := ed.todoItems()[0]
	if findLabelText(ed.box, item.completedDate) {
		t.Fatal("unchecked item should show no date tag")
	}

	ed.itemCheck[item].SetChecked(true)
	item = ed.todoItems()[0] // rebuild replaced the row; item pointer itself is stable
	if !findLabelText(ed.box, item.completedDate) {
		t.Fatalf("checked item should show a %q tag", item.completedDate)
	}

	ed.itemCheck[item].SetChecked(false)
	if findLabelText(ed.box, time.Now().Format(store.JournalDateLayout)) {
		t.Fatal("unchecking should remove the date tag")
	}
}

// findLabelText reports whether o (or anything nested inside it) is a
// *widget.Label with exactly this text.
func findLabelText(o fyne.CanvasObject, text string) bool {
	if l, ok := o.(*widget.Label); ok && l.Text == text {
		return true
	}
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if findLabelText(child, text) {
				return true
			}
		}
		return false
	}
	if w, ok := o.(fyne.Widget); ok {
		for _, child := range test.WidgetRenderer(w).Objects() {
			if findLabelText(child, text) {
				return true
			}
		}
	}
	return false
}

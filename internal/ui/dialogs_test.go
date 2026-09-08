package ui

import (
	"reflect"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

// walk visits o and, via reflection, any child it exposes through a
// `Content` / `Objects` field — enough to reach into a PopUp > Card >
// Container tree in tests without importing Fyne internals.
func walk(o fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	fn(o)
	if c, ok := o.(*fyne.Container); ok {
		for _, ch := range c.Objects {
			walk(ch, fn)
		}
		return
	}
	v := reflect.Indirect(reflect.ValueOf(o))
	if v.Kind() != reflect.Struct {
		return
	}
	if f := v.FieldByName("Objects"); f.IsValid() {
		if objs, ok := f.Interface().([]fyne.CanvasObject); ok {
			for _, ch := range objs {
				walk(ch, fn)
			}
		}
	}
	if f := v.FieldByName("Content"); f.IsValid() && f.CanInterface() {
		if ch, ok := f.Interface().(fyne.CanvasObject); ok {
			walk(ch, fn)
		}
	}
}

func findInOverlays[T fyne.CanvasObject](t *testing.T, c fyne.Canvas) T {
	t.Helper()
	var found T
	var ok bool
	for _, ov := range c.Overlays().List() {
		walk(ov, func(o fyne.CanvasObject) {
			if v, is := o.(T); is && !ok {
				found, ok = v, true
			}
		})
	}
	if !ok {
		t.Fatalf("no %T in overlays", found)
	}
	return found
}

// TestShowTextEntry_EnterMatchesButton is the regression for the reminder
// entry popup: pressing Enter must do exactly what clicking the submit
// button does — call onSubmit once with the typed text — so the reminder's
// follow-up (dismiss notification, return window to tray) runs either way.
func TestShowTextEntry_EnterMatchesButton(t *testing.T) {
	run := func(useEnter bool) (string, int) {
		test.NewApp()
		w := test.NewWindow(widget.NewLabel("bg"))
		defer w.Close()

		var got string
		var calls int
		ShowTextEntry(w, "Journal entry", "Entry:", "Add", func(s string) {
			got = s
			calls++
		})

		entry := findInOverlays[*EnterModeEntry](t, w.Canvas())
		entry.SetText("today was fine")

		if useEnter {
			entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		} else {
			test.Tap(findInOverlays[*widget.Button](t, w.Canvas()))
		}
		return got, calls
	}

	gotEnter, callsEnter := run(true)
	gotButton, callsButton := run(false)

	if callsEnter != 1 || callsButton != 1 {
		t.Fatalf("onSubmit calls: enter=%d button=%d, want 1 and 1", callsEnter, callsButton)
	}
	if gotEnter != "today was fine" || gotButton != "today was fine" {
		t.Fatalf("onSubmit text: enter=%q button=%q", gotEnter, gotButton)
	}
}

// TestShowTextEntry_ShiftEnterInsertsNewline: Shift+Enter must not submit.
func TestShowTextEntry_ShiftEnterInsertsNewline(t *testing.T) {
	test.NewApp()
	w := test.NewWindow(widget.NewLabel("bg"))
	defer w.Close()

	calls := 0
	ShowTextEntry(w, "Journal entry", "Entry:", "Add", func(string) { calls++ })

	entry := findInOverlays[*EnterModeEntry](t, w.Canvas())
	entry.SetText("line one")
	entry.KeyDown(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	entry.KeyUp(&fyne.KeyEvent{Name: desktop.KeyShiftLeft})

	if calls != 0 {
		t.Fatalf("Shift+Enter submitted (calls=%d), want 0", calls)
	}
}

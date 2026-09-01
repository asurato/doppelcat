package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asurato/doppelcat/internal/document"
	"github.com/asurato/doppelcat/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func dirtyUI(t *testing.T) (*UI, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "document.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := document.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	u := New(path, snapshot)
	u.startEdit()
	u.editor.SetText("new", true)
	return u, path
}

func pressModalKey(t *testing.T, u *UI, event *tcell.EventKey) {
	t.Helper()
	modal, ok := u.pages.GetPage("modal").(*tview.Modal)
	if !ok {
		t.Fatal("modal is not displayed")
	}
	modal.InputHandler()(event, func(tview.Primitive) {})
}

func TestLeaveEditDialogKeyboardShortcuts(t *testing.T) {
	t.Run("save and close", func(t *testing.T) {
		u, path := dirtyUI(t)
		u.leaveEdit()
		pressModalKey(t, u, tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone))

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "new" || u.model.Dirty || u.model.Mode == model.Edit || u.modal {
			t.Fatalf("save shortcut did not save and close: content=%q dirty=%v mode=%v modal=%v", content, u.model.Dirty, u.model.Mode, u.modal)
		}
	})

	t.Run("continue editing", func(t *testing.T) {
		for name, event := range map[string]*tcell.EventKey{
			"c":      tcell.NewEventKey(tcell.KeyRune, 'C', tcell.ModShift),
			"escape": tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		} {
			t.Run(name, func(t *testing.T) {
				u, _ := dirtyUI(t)
				u.leaveEdit()
				pressModalKey(t, u, event)
				if u.model.Mode != model.Edit || !u.model.Dirty || u.modal {
					t.Fatalf("continue shortcut changed edits: dirty=%v mode=%v modal=%v", u.model.Dirty, u.model.Mode, u.modal)
				}
			})
		}
	})

	t.Run("discard", func(t *testing.T) {
		u, path := dirtyUI(t)
		u.leaveEdit()
		pressModalKey(t, u, tcell.NewEventKey(tcell.KeyRune, 'd', tcell.ModNone))

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "old" || u.model.Dirty || u.model.Mode != model.Normal || u.modal {
			t.Fatalf("discard shortcut did not discard and close: content=%q dirty=%v mode=%v modal=%v", content, u.model.Dirty, u.model.Mode, u.modal)
		}
	})
}

func TestPlatformCursorKeyOnMacOS(t *testing.T) {
	u := &UI{editor: tview.NewTextArea()}
	tests := []struct {
		name string
		in   tcell.Key
		want tcell.Key
	}{
		{"Ctrl+A", tcell.KeyCtrlA, tcell.KeyHome},
		{"Ctrl+B", tcell.KeyCtrlB, tcell.KeyLeft},
		{"Ctrl+E", tcell.KeyCtrlE, tcell.KeyEnd},
		{"Ctrl+F", tcell.KeyCtrlF, tcell.KeyRight},
		{"Ctrl+N", tcell.KeyCtrlN, tcell.KeyDown},
		{"Ctrl+P", tcell.KeyCtrlP, tcell.KeyUp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, handled := u.platformEditorKey(tcell.NewEventKey(tt.in, 0, tcell.ModCtrl), "darwin")
			if !handled {
				t.Fatalf("platformCursorKey(%s) was not handled", tt.name)
			}
			if got == nil || got.Key() != tt.want || got.Modifiers() != tcell.ModNone {
				t.Fatalf("platformCursorKey(%s) = %v, want key %v without modifiers", tt.name, got, tt.want)
			}
		})
	}
}

func TestPlatformCursorKeyOnLinux(t *testing.T) {
	u := &UI{editor: tview.NewTextArea()}
	tests := []struct {
		in   tcell.Key
		want tcell.Key
	}{
		{tcell.KeyCtrlA, tcell.KeyHome},
		{tcell.KeyCtrlB, tcell.KeyLeft},
		{tcell.KeyCtrlE, tcell.KeyEnd},
		{tcell.KeyCtrlF, tcell.KeyRight},
		{tcell.KeyCtrlN, tcell.KeyDown},
		{tcell.KeyCtrlP, tcell.KeyUp},
	}

	for _, tt := range tests {
		got, handled := u.platformEditorKey(tcell.NewEventKey(tt.in, 0, tcell.ModCtrl), "linux")
		if !handled || got == nil || got.Key() != tt.want || got.Modifiers() != tcell.ModNone {
			t.Fatalf("platformEditorKey(%v, linux) = (%v, %v), want key %v without modifiers", tt.in, got, handled, tt.want)
		}
	}
}

func TestPlatformCursorKeyOnWindowsDoesNothing(t *testing.T) {
	u := &UI{editor: tview.NewTextArea()}
	for _, key := range []tcell.Key{tcell.KeyCtrlA, tcell.KeyCtrlB, tcell.KeyCtrlE, tcell.KeyCtrlF, tcell.KeyCtrlL, tcell.KeyCtrlN, tcell.KeyCtrlP} {
		got, handled := u.platformEditorKey(tcell.NewEventKey(key, 0, tcell.ModCtrl), "windows")
		if !handled || got != nil {
			t.Fatalf("platformCursorKey(%v, windows) = (%v, %v), want (nil, true)", key, got, handled)
		}
	}
}

func TestPlatformCursorKeyLeavesOtherEditorBindingsAlone(t *testing.T) {
	u := &UI{editor: tview.NewTextArea()}
	event := tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModCtrl)
	got, handled := u.platformEditorKey(event, "windows")
	if handled || got != event {
		t.Fatalf("platformCursorKey(Ctrl+D) = (%v, %v), want original event and false", got, handled)
	}
}

func TestPlatformCursorKeyCentersEditorOnMacOS(t *testing.T) {
	editor := tview.NewTextArea()
	editor.SetRect(0, 0, 80, 10)
	editor.SetText(strings.Repeat("line\n", 20), true)
	u := &UI{editor: editor}

	got, handled := u.platformEditorKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl), "darwin")
	if !handled || got != nil {
		t.Fatalf("platformCursorKey(Ctrl+L, darwin) = (%v, %v), want (nil, true)", got, handled)
	}
	rowOffset, _ := editor.GetOffset()
	if rowOffset != 15 {
		t.Fatalf("Ctrl+L row offset = %d, want 15", rowOffset)
	}
}

func TestPlatformCursorKeyCentersEditorOnLinux(t *testing.T) {
	editor := tview.NewTextArea()
	editor.SetRect(0, 0, 80, 10)
	editor.SetText(strings.Repeat("line\n", 20), true)
	u := &UI{editor: editor}

	got, handled := u.platformEditorKey(tcell.NewEventKey(tcell.KeyCtrlL, 0, tcell.ModCtrl), "linux")
	if !handled || got != nil {
		t.Fatalf("platformEditorKey(Ctrl+L, linux) = (%v, %v), want (nil, true)", got, handled)
	}
	rowOffset, _ := editor.GetOffset()
	if rowOffset != 15 {
		t.Fatalf("Ctrl+L row offset = %d, want 15", rowOffset)
	}
}

func TestStartEditKeepsViewerLine(t *testing.T) {
	text := numberedLines(30)
	snapshot, err := document.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	u := New("test.txt", snapshot)
	u.editor.SetRect(0, 0, 80, 10)
	u.viewer.Move(14)

	u.startEdit()

	if got := u.editorLine(); got != 15 {
		t.Fatalf("editor line = %d, want 15", got)
	}
	rowOffset, _ := u.editor.GetOffset()
	if rowOffset != 14 {
		t.Fatalf("editor row offset = %d, want 14", rowOffset)
	}
}

func TestLeaveEditKeepsEditorLine(t *testing.T) {
	text := numberedLines(30)
	snapshot, err := document.Decode([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	u := New("test.txt", snapshot)
	u.startEdit()
	u.seekEditorLine(18)

	u.leaveEdit()

	if got := u.viewer.TopLine(); got != 18 {
		t.Fatalf("viewer top line = %d, want 18", got)
	}
}

func TestFooterShowsModePathAndContextualGuide(t *testing.T) {
	snapshot, err := document.Decode([]byte("text"))
	if err != nil {
		t.Fatal(err)
	}
	u := New("test.txt", snapshot)
	u.transient = ""
	u.refresh()

	if got, want := u.status.GetText(true), " VIEW  test.txt\n d Diff  e Edit  q Quit"; got != want {
		t.Fatalf("view footer = %q, want %q", got, want)
	}

	u.model.Mode = model.Diff
	u.refresh()
	if got, want := u.status.GetText(true), " DIFF  test.txt\n d View  e Edit  q Quit"; got != want {
		t.Fatalf("diff footer = %q, want %q", got, want)
	}

	u.startEdit()
	if got, want := u.status.GetText(true), " EDIT  test.txt\n Ctrl+S Save  Esc View  Ctrl+Q Quit"; got != want {
		t.Fatalf("edit footer = %q, want %q", got, want)
	}
}

func TestFooterMessageReplacesGuide(t *testing.T) {
	snapshot, err := document.Decode([]byte("text"))
	if err != nil {
		t.Fatal(err)
	}
	u := New("test.txt", snapshot)
	u.transient = "clipboard unavailable"
	u.refresh()

	if got, want := u.status.GetText(true), " VIEW  test.txt\n clipboard unavailable"; got != want {
		t.Fatalf("message footer = %q, want %q", got, want)
	}
}

func numberedLines(count int) string {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString("line\n")
	}
	return b.String()
}

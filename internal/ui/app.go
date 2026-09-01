package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/asurato/doppelcat/internal/document"
	"github.com/asurato/doppelcat/internal/model"
	watcher "github.com/asurato/doppelcat/internal/watch"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type UI struct {
	app       *tview.Application
	pages     *tview.Pages
	content   *tview.Pages
	body      *tview.Flex
	viewer    *Viewer
	editor    *tview.TextArea
	status    *tview.TextView
	model     *model.Model
	watcher   *watcher.Watcher
	cancel    context.CancelFunc
	transient string
	modal     bool
}

const DefaultDebounce = 200 * time.Millisecond

func Run(path string, initial document.Snapshot) error {
	return RunWithDebounce(path, initial, DefaultDebounce)
}

func RunWithDebounce(path string, initial document.Snapshot, debounce time.Duration) error {
	u := NewWithDebounce(path, initial, debounce)
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	if err := u.watcher.Start(ctx); err != nil {
		return err
	}
	go func() {
		for ev := range u.watcher.Events() {
			e := ev
			u.app.QueueUpdateDraw(func() { u.onWatch(e) })
		}
	}()
	err := u.app.Run()
	cancel()
	u.watcher.Close()
	return err
}

func New(path string, initial document.Snapshot) *UI {
	return NewWithDebounce(path, initial, DefaultDebounce)
}

func NewWithDebounce(path string, initial document.Snapshot, debounce time.Duration) *UI {
	u := &UI{app: tview.NewApplication(), viewer: NewViewer(), editor: tview.NewTextArea(), status: tview.NewTextView(), model: model.New(path, initial), watcher: watcher.New(path, debounce, 250*time.Millisecond)}
	u.viewer.SetBorder(true).SetTitle(" doppelcat ")
	u.editor.SetBorder(true).SetTitle(" EDIT ")
	u.editor.SetText(initial.Text, false)
	u.editor.SetWrap(true)
	u.editor.SetWordWrap(false)
	u.status.SetDynamicColors(true).SetWrap(false)
	u.content = tview.NewPages().AddPage("viewer", u.viewer, true, true).AddPage("editor", u.editor, true, false)
	u.body = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(u.content, 0, 1, true).AddItem(u.status, 2, 0, false)
	u.pages = tview.NewPages().AddPage("main", u.body, true, true)
	u.app.SetRoot(u.pages, true).EnablePaste(true)
	u.viewer.SetNormal(initial.Text)
	u.app.SetFocus(u.viewer)
	cb := newClipboard(func(s string) { u.transient = s; u.refresh() })
	u.editor.SetClipboard(cb.copy, cb.paste)
	u.editor.SetChangedFunc(func() { u.model.SetBuffer(u.editor.GetText()); u.refresh() })
	u.app.SetInputCapture(u.capture)
	u.refresh()
	return u
}

func (u *UI) capture(e *tcell.EventKey) *tcell.EventKey {
	if e.Key() == tcell.KeyCtrlQ {
		if u.modal {
			u.pages.RemovePage("modal")
			u.modal = false
		}
		u.confirmQuit()
		return nil
	}
	if u.modal {
		return e
	}
	if u.model.Mode == model.Edit {
		if event, handled := u.platformEditorKey(e, runtime.GOOS); handled {
			return event
		}
		switch e.Key() {
		case tcell.KeyCtrlS:
			u.save()
			return nil
		case tcell.KeyEscape:
			u.leaveEdit()
			return nil
		case tcell.KeyCtrlC:
			return tcell.NewEventKey(tcell.KeyCtrlQ, 0, e.Modifiers())
		}
		return e
	}
	switch {
	case e.Rune() == 'q':
		u.app.Stop()
		return nil
	case e.Rune() == 'd':
		if len(u.model.DiffLines) == 0 {
			return nil
		}
		if u.model.Mode == model.Diff {
			u.model.Mode = model.Normal
		} else {
			u.model.Mode = model.Diff
		}
		u.showViewer()
		return nil
	case e.Rune() == 'e':
		u.startEdit()
		return nil
	case e.Rune() == 'c' && u.model.Conflict:
		u.showConflict()
		return nil
	case e.Key() == tcell.KeyHome && e.Modifiers()&tcell.ModCtrl != 0:
		u.viewer.Home()
		return nil
	case e.Key() == tcell.KeyEnd && e.Modifiers()&tcell.ModCtrl != 0:
		u.viewer.End()
		return nil
	}
	return e
}

func (u *UI) platformEditorKey(e *tcell.EventKey, goos string) (*tcell.EventKey, bool) {
	if e.Key() == tcell.KeyCtrlL {
		if goos == "darwin" || goos == "linux" {
			u.centerEditorCursor()
			return nil, true
		}
		if goos == "windows" {
			return nil, true
		}
		return e, false
	}

	keys := map[tcell.Key]tcell.Key{
		tcell.KeyCtrlA: tcell.KeyHome,
		tcell.KeyCtrlB: tcell.KeyLeft,
		tcell.KeyCtrlE: tcell.KeyEnd,
		tcell.KeyCtrlF: tcell.KeyRight,
		tcell.KeyCtrlN: tcell.KeyDown,
		tcell.KeyCtrlP: tcell.KeyUp,
	}
	target, ok := keys[e.Key()]
	if !ok {
		return e, false
	}
	if goos == "darwin" || goos == "linux" {
		return tcell.NewEventKey(target, 0, tcell.ModNone), true
	}
	if goos == "windows" {
		return nil, true
	}
	return e, false
}

func (u *UI) centerEditorCursor() {
	fromRow, _, toRow, _ := u.editor.GetCursor()
	_, _, _, height := u.editor.GetInnerRect()
	rowOffset := (fromRow+toRow)/2 - height/2
	if rowOffset < 0 {
		rowOffset = 0
	}
	_, columnOffset := u.editor.GetOffset()
	u.editor.SetOffset(rowOffset, columnOffset)
}

func (u *UI) startEdit() {
	if u.model.Availability != model.Available {
		return
	}
	line := u.viewer.TopLine()
	u.model.EnterEdit()
	u.editor.SetText(u.model.Buffer, false)
	u.seekEditorLine(line)
	u.content.SwitchToPage("editor")
	u.app.SetFocus(u.editor)
	u.refresh()
}
func (u *UI) showViewer() {
	u.showViewerAt(0)
}
func (u *UI) showViewerAt(line int) {
	u.content.SwitchToPage("viewer")
	if u.model.Mode == model.Diff {
		u.viewer.SetDiff(u.model.DiffLines)
	} else {
		u.viewer.SetNormal(u.model.Current.Text)
	}
	if line > 0 {
		u.viewer.seek(line)
	}
	u.app.SetFocus(u.viewer)
	u.refresh()
}
func (u *UI) leaveEdit() {
	line := u.editorLine()
	if u.model.Dirty {
		u.choiceWithKeys("Unsaved changes", map[rune]int{'s': 0, 'c': 1, 'd': 2}, 1,
			"Save and close (s)", func() { u.save() },
			"Continue editing (c / Esc)", func() {},
			"Discard (d)", func() { u.model.SetBuffer(u.model.Current.Text); u.model.Mode = model.Normal; u.showViewerAt(line) },
		)
		return
	}
	u.model.Mode = model.Normal
	u.showViewerAt(line)
}
func (u *UI) save() {
	line := u.editorLine()
	u.model.SetBuffer(u.editor.GetText())
	if err := u.model.Save(); err != nil {
		u.transient = "save failed: " + err.Error()
		if errors.Is(err, document.ErrChanged) {
			u.transient = "save blocked: external change detected"
		}
		u.refresh()
		return
	}
	u.transient = ""
	u.showViewerAt(line)
}

func (u *UI) seekEditorLine(line int) {
	text := u.editor.GetText()
	index := 0
	for current := 1; current < line; current++ {
		next := strings.IndexByte(text[index:], '\n')
		if next < 0 {
			index = len(text)
			break
		}
		index += next + 1
	}
	u.editor.Select(index, index)
	row, _, _, _ := u.editor.GetCursor()
	logicalRow := strings.Count(text[:index], "\n")
	if row < logicalRow {
		row = logicalRow
	}
	u.editor.SetOffset(row, 0)
}

func (u *UI) editorLine() int {
	text := u.editor.GetText()
	_, index, _ := u.editor.GetSelection()
	if index > len(text) {
		index = len(text)
	}
	return strings.Count(text[:index], "\n") + 1
}

func (u *UI) onWatch(ev watcher.Result) {
	wasEditing := u.model.Mode == model.Edit
	beforeTop := u.viewer.TopLine()
	u.model.ApplyExternal(ev.Snapshot, ev.Err)
	if u.model.Conflict {
		u.refresh()
		u.showConflict()
		return
	}
	if u.model.Availability == model.Unavailable {
		u.viewer.SetNormal("")
		u.content.SwitchToPage("viewer")
		u.app.SetFocus(u.viewer)
		u.refresh()
		return
	}
	if u.model.Mode != model.Edit {
		u.model.Mode = model.Diff
		u.viewer.SetDiff(u.model.DiffLines)
		for u.viewer.TopLine() < beforeTop && u.viewer.top < len(u.viewer.lines)-1 {
			u.viewer.top++
		}
	}
	if wasEditing && u.model.Mode != model.Edit {
		u.showViewer()
		return
	}
	u.refresh()
}

func (u *UI) showConflict() {
	if u.modal {
		return
	}
	u.choice("External change conflicts with unsaved edits", "Keep local edits", func() {
		if u.model.Mode == model.Edit && u.model.Availability == model.Available {
			u.content.SwitchToPage("editor")
			u.app.SetFocus(u.editor)
		}
	}, "Reload external", func() { u.confirm("Discard local edits?", func() { u.model.ReloadExternal(); u.showViewer() }) }, "Overwrite external", func() {
		u.confirm("Overwrite the external version?", func() {
			if err := u.model.Overwrite(); err != nil {
				u.transient = err.Error()
			}
			u.showViewer()
		})
	})
}

func (u *UI) confirmQuit() {
	if !u.model.Dirty {
		u.app.Stop()
		return
	}
	u.choice("Unsaved changes", "Save and quit", func() {
		u.model.SetBuffer(u.editor.GetText())
		if err := u.model.Save(); err != nil {
			u.transient = "save failed: " + err.Error()
			u.refresh()
			return
		}
		u.app.Stop()
	}, "Discard and quit", func() { u.app.Stop() }, "Cancel", func() {})
}

func (u *UI) confirm(text string, yes func()) { u.choice(text, "Confirm", yes, "Cancel", func() {}) }
func (u *UI) choice(text string, args ...any) {
	u.choiceWithKeys(text, nil, -1, args...)
}

func (u *UI) choiceWithKeys(text string, keys map[rune]int, escapeIndex int, args ...any) {
	m := tview.NewModal().SetText(text)
	labels := make([]string, 0, len(args)/2)
	callbacks := make([]func(), 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		labels = append(labels, args[i].(string))
		callbacks = append(callbacks, args[i+1].(func()))
	}
	activate := func(index int) {
		if index < 0 || index >= len(callbacks) {
			return
		}
		u.pages.RemovePage("modal")
		u.modal = false
		callbacks[index]()
		u.refresh()
	}
	m.AddButtons(labels).SetDoneFunc(func(_ int, label string) {
		for i, l := range labels {
			if l == label {
				activate(i)
				break
			}
		}
	})
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		index := -1
		if event.Key() == tcell.KeyEscape {
			index = escapeIndex
		} else if event.Key() == tcell.KeyRune {
			if i, ok := keys[unicode.ToLower(event.Rune())]; ok {
				index = i
			}
		}
		if index >= 0 {
			activate(index)
			return nil
		}
		return event
	})
	u.modal = true
	u.pages.AddPage("modal", m, true, true)
	u.app.SetFocus(m)
}

func (u *UI) refresh() {
	modeName := map[model.Mode]string{model.Normal: "VIEW", model.Diff: "DIFF", model.Edit: "EDIT"}[u.model.Mode]
	guide := "d Diff  e Edit  q Quit"
	if u.model.Mode == model.Diff {
		guide = "d View  e Edit  q Quit"
	} else if u.model.Mode == model.Edit {
		guide = "Ctrl+S Save  Esc View  Ctrl+Q Quit"
	}
	message := u.transient
	if u.model.StatusError != nil {
		message = u.model.StatusError.Error()
	}
	if message != "" {
		guide = message
	}
	u.status.SetText(fmt.Sprintf(" %s  %s\n %s", modeName, tview.Escape(filepath.Clean(u.model.Path)), tview.Escape(guide)))
}

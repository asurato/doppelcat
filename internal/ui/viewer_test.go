package ui

import (
	"strings"
	"testing"

	"github.com/asurato/doppelcat/internal/diff"
	"github.com/asurato/doppelcat/internal/document"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestViewerDrawsDiffMarks(t *testing.T) {
	s := tcell.NewSimulationScreen("UTF-8")
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	defer s.Fini()
	s.SetSize(20, 4)
	v := NewViewer()
	v.SetRect(0, 0, 20, 4)
	lines, _ := diff.Lines("old\nsame\n", "new\nsame\n")
	v.SetDiff(lines)
	v.Draw(s)
	s.Show()
	cells, w, h := s.GetContents()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := cells[y*w+x].Runes
			if len(r) > 0 {
				b.WriteRune(r[0])
			}
		}
		b.WriteByte('\n')
	}
	text := b.String()
	if !strings.Contains(text, "-     1 old") || !strings.Contains(text, "+     1 new") {
		t.Fatalf("screen:\n%s", text)
	}
}

func TestWrapUsesDisplayWidth(t *testing.T) {
	got := wrap("日本語abc", 4)
	if len(got) < 2 || got[0] != "日本" {
		t.Fatalf("%q", got)
	}
}

func TestViewerKeepsLatestLogicalAnchor(t *testing.T) {
	v := NewViewer()
	v.SetNormal("a\nb\nc\n")
	v.Move(1)
	lines, _ := diff.Lines("a\nb\nc\n", "a\nc\n")
	v.SetDiff(lines)
	if got := v.TopLine(); got != 2 {
		t.Fatalf("top logical line=%d", got)
	}
}

func TestViewerFocusDoesNotDelegateToItself(t *testing.T) {
	v := NewViewer()
	v.Focus(func(tview.Primitive) { t.Fatal("Viewer must not delegate focus to itself") })
	if !v.HasFocus() {
		t.Fatal("viewer did not receive focus")
	}
}

func TestNewInitializesViewerFocus(t *testing.T) {
	s, err := document.Decode([]byte("test\n"))
	if err != nil {
		t.Fatal(err)
	}
	u := New("test.txt", s)
	if !u.viewer.HasFocus() {
		t.Fatal("viewer did not receive initial focus")
	}
}

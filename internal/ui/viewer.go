package ui

import (
	"fmt"
	"strings"

	"github.com/asurato/doppelcat/internal/diff"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"
)

type viewLine struct {
	mark    byte
	number  int
	logical int
	text    string
	color   tcell.Color
}

type Viewer struct {
	*tview.Box
	lines []viewLine
	top   int
}

func NewViewer() *Viewer { return &Viewer{Box: tview.NewBox()} }

func (v *Viewer) SetNormal(text string) {
	line := v.TopLine()
	parts := diff.Split(text)
	v.lines = make([]viewLine, len(parts))
	for i, s := range parts {
		v.lines[i] = viewLine{' ', i + 1, i + 1, s, tcell.ColorDefault}
	}
	v.seek(line)
}

func (v *Viewer) SetDiff(lines []diff.Line) {
	line := v.TopLine()
	v.lines = make([]viewLine, len(lines))
	for i, l := range lines {
		n, c := l.NewNo, tcell.ColorDefault
		if l.Kind == diff.Delete {
			n, c = l.OldNo, tcell.ColorRed
		} else if l.Kind == diff.Insert {
			c = tcell.ColorGreen
		}
		logical := l.NewNo
		if l.Kind == diff.Delete {
			logical = l.Anchor
		}
		v.lines[i] = viewLine{byte(l.Kind), n, logical, l.Text, c}
	}
	v.seek(line)
}

func (v *Viewer) seek(line int) {
	if line <= 0 {
		v.clamp()
		return
	}
	v.top = 0
	for i, l := range v.lines {
		if l.logical >= line {
			v.top = i
			break
		}
		v.top = i
	}
	v.clamp()
}

func (v *Viewer) TopLine() int {
	if len(v.lines) == 0 {
		return 0
	}
	return v.lines[v.top].logical
}
func (v *Viewer) Move(delta int) { v.top += delta; v.clamp() }
func (v *Viewer) Home()          { v.top = 0 }
func (v *Viewer) End() {
	_, _, _, h := v.GetInnerRect()
	v.top = len(v.lines) - h
	if v.top < 0 {
		v.top = 0
	}
}
func (v *Viewer) clamp() {
	if v.top < 0 {
		v.top = 0
	}
	if v.top >= len(v.lines) && len(v.lines) > 0 {
		v.top = len(v.lines) - 1
	}
	if len(v.lines) == 0 {
		v.top = 0
	}
}

func (v *Viewer) Draw(screen tcell.Screen) {
	v.Box.DrawForSubclass(screen, v)
	x, y, w, h := v.GetInnerRect()
	if w <= 0 || h <= 0 {
		return
	}
	row := 0
	for i := v.top; i < len(v.lines) && row < h; i++ {
		line := v.lines[i]
		prefix := fmt.Sprintf("%c%6d ", line.mark, line.number)
		avail := w - uniseg.StringWidth(prefix)
		if avail < 1 {
			avail = 1
		}
		chunks := wrap(line.text, avail)
		if len(chunks) == 0 {
			chunks = []string{""}
		}
		for j, ch := range chunks {
			if row >= h {
				break
			}
			p := "        "
			if j == 0 {
				p = prefix
			}
			drawText(screen, x, y+row, w, p+ch, line.color)
			row++
		}
	}
}

func wrap(s string, width int) []string {
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	used := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		part := g.Str()
		pw := uniseg.StringWidth(part)
		if used > 0 && used+pw > width {
			out = append(out, b.String())
			b.Reset()
			used = 0
		}
		b.WriteString(part)
		used += pw
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func drawText(screen tcell.Screen, x, y, width int, s string, color tcell.Color) {
	style := tcell.StyleDefault.Foreground(color)
	col := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		runes := []rune(g.Str())
		if len(runes) == 0 {
			continue
		}
		rw := uniseg.StringWidth(g.Str())
		if col+rw > width {
			break
		}
		screen.SetContent(x+col, y, runes[0], runes[1:], style)
		col += rw
	}
}

func (v *Viewer) Focus(delegate func(p tview.Primitive)) { delegate(v) }
func (v *Viewer) HasFocus() bool                         { return v.Box.HasFocus() }
func (v *Viewer) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return v.WrapInputHandler(func(e *tcell.EventKey, setFocus func(tview.Primitive)) {
		switch e.Key() {
		case tcell.KeyUp:
			v.Move(-1)
		case tcell.KeyDown:
			v.Move(1)
		case tcell.KeyHome:
			v.Home()
		case tcell.KeyEnd:
			v.End()
		}
	})
}

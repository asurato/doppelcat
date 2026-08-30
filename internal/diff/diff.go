package diff

import "strings"

type Kind byte

const (
	Equal  Kind = ' '
	Delete Kind = '-'
	Insert Kind = '+'
)

type Line struct {
	Kind         Kind
	Text         string
	OldNo, NewNo int
	Anchor       int
}

type Options struct{ MaxDistance, MaxTraceCells int }

var DefaultOptions = Options{MaxDistance: 2000, MaxTraceCells: 2_000_000}

func Lines(oldText, newText string) ([]Line, bool) {
	return LinesWith(oldText, newText, DefaultOptions)
}

func Split(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	if parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

type edit struct {
	kind Kind
	text string
}

func LinesWith(oldText, newText string, opt Options) ([]Line, bool) {
	a, b := Split(oldText), Split(newText)
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		p++
	}
	s := 0
	for s < len(a)-p && s < len(b)-p && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	edits, ok := myers(a[p:len(a)-s], b[p:len(b)-s], opt)
	if !ok {
		edits = make([]edit, 0, len(a)+len(b)-2*p-2*s)
		for _, v := range a[p : len(a)-s] {
			edits = append(edits, edit{Delete, v})
		}
		for _, v := range b[p : len(b)-s] {
			edits = append(edits, edit{Insert, v})
		}
	}
	all := make([]edit, 0, p+len(edits)+s)
	for _, v := range a[:p] {
		all = append(all, edit{Equal, v})
	}
	all = append(all, edits...)
	for _, v := range a[len(a)-s:] {
		all = append(all, edit{Equal, v})
	}
	return number(all), ok
}

func myers(a, b []string, opt Options) ([]edit, bool) {
	n, m := len(a), len(b)
	if n == 0 {
		r := make([]edit, m)
		for i, v := range b {
			r[i] = edit{Insert, v}
		}
		return r, true
	}
	if m == 0 {
		r := make([]edit, n)
		for i, v := range a {
			r[i] = edit{Delete, v}
		}
		return r, true
	}
	max := n + m
	if opt.MaxDistance > 0 && max > opt.MaxDistance {
		max = opt.MaxDistance
	}
	off := max + 1
	v := make([]int, 2*max+3)
	for i := range v {
		v[i] = -1
	}
	v[off+1] = 0
	trace := make([][]int, 0, max+1)
	cells := 0
	for d := 0; d <= max; d++ {
		if opt.MaxTraceCells > 0 && cells+2*d+1 > opt.MaxTraceCells {
			return nil, false
		}
		for k := -d; k <= d; k += 2 {
			idx := off + k
			x := 0
			if k == -d || (k != d && v[idx-1] < v[idx+1]) {
				x = v[idx+1]
			} else {
				x = v[idx-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}
			v[idx] = x
			if x >= n && y >= m {
				cp := append([]int(nil), v...)
				trace = append(trace, cp)
				return backtrack(a, b, trace, off, d), true
			}
		}
		trace = append(trace, append([]int(nil), v...))
		cells += 2*d + 1
	}
	return nil, false
}

func backtrack(a, b []string, trace [][]int, off, distance int) []edit {
	x, y := len(a), len(b)
	rev := make([]edit, 0, x+y)
	for d := distance; d > 0; d-- {
		prev := trace[d-1]
		k := x - y
		prevK := k - 1
		if k == -d || (k != d && prev[off+k-1] < prev[off+k+1]) {
			prevK = k + 1
		}
		prevX := prev[off+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			rev = append(rev, edit{Equal, a[x-1]})
			x--
			y--
		}
		if x == prevX {
			rev = append(rev, edit{Insert, b[y-1]})
			y--
		} else {
			rev = append(rev, edit{Delete, a[x-1]})
			x--
		}
	}
	for x > 0 && y > 0 {
		rev = append(rev, edit{Equal, a[x-1]})
		x--
		y--
	}
	for x > 0 {
		rev = append(rev, edit{Delete, a[x-1]})
		x--
	}
	for y > 0 {
		rev = append(rev, edit{Insert, b[y-1]})
		y--
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func number(edits []edit) []Line {
	lines := make([]Line, 0, len(edits))
	oldNo, newNo := 1, 1
	for _, e := range edits {
		l := Line{Kind: e.kind, Text: e.text, Anchor: newNo}
		switch e.kind {
		case Equal:
			l.OldNo, l.NewNo = oldNo, newNo
			oldNo++
			newNo++
		case Delete:
			l.OldNo = oldNo
			oldNo++
		case Insert:
			l.NewNo = newNo
			newNo++
		}
		lines = append(lines, l)
	}
	return lines
}

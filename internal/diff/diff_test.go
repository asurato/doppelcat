package diff

import "testing"

func TestLines(t *testing.T) {
	got, ok := Lines("a\nb\nc\n", "a\nx\nc\nd\n")
	if !ok {
		t.Fatal("fallback")
	}
	want := " a-b+x c+d"
	out := ""
	for _, l := range got {
		out += string(l.Kind) + l.Text
	}
	if out != want {
		t.Fatalf("%q", out)
	}
	if got[2].Anchor != 2 {
		t.Fatalf("delete anchor %d", got[2].Anchor)
	}
}
func TestFallback(t *testing.T) {
	got, ok := LinesWith("a\nb\nc", "x\ny\nz", Options{MaxDistance: 1, MaxTraceCells: 10})
	if ok {
		t.Fatal("expected fallback")
	}
	if len(got) != 6 {
		t.Fatalf("len=%d", len(got))
	}
}

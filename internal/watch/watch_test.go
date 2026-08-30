package watch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitResult(t *testing.T, ch <-chan Result) Result {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("watch timeout")
		return Result{}
	}
}

func TestWriteDeleteRecreate(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "doc.txt")
	if err := os.WriteFile(p, []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	w := New(p, 40*time.Millisecond, 40*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := os.WriteFile(p, []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r := waitResult(t, w.Events())
	if r.Snapshot == nil || r.Snapshot.Text != "two\n" {
		t.Fatalf("write: %#v", r)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	r = waitResult(t, w.Events())
	if !r.Missing {
		t.Fatalf("delete: %#v", r)
	}
	if err := os.WriteFile(p, []byte("three\n"), 0600); err != nil {
		t.Fatal(err)
	}
	r = waitResult(t, w.Events())
	if r.Snapshot == nil || r.Snapshot.Text != "three\n" {
		t.Fatalf("recreate: %#v", r)
	}
}

func TestConsecutiveWritesAreDebounced(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "doc.txt")
	_ = os.WriteFile(p, []byte("zero"), 0600)
	w := New(p, 100*time.Millisecond, 25*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	for _, s := range []string{"one", "two", "final"} {
		_ = os.WriteFile(p, []byte(s), 0600)
		time.Sleep(20 * time.Millisecond)
	}
	r := waitResult(t, w.Events())
	if r.Snapshot == nil || r.Snapshot.Text != "final" {
		t.Fatalf("%#v", r)
	}
}

func TestSameContentAfterRecreateIsEmitted(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "doc.txt")
	_ = os.WriteFile(p, []byte("same"), 0600)
	w := New(p, 40*time.Millisecond, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	_ = os.Remove(p)
	if r := waitResult(t, w.Events()); !r.Missing {
		t.Fatalf("%#v", r)
	}
	_ = os.WriteFile(p, []byte("same"), 0600)
	r := waitResult(t, w.Events())
	if r.Snapshot == nil || r.Snapshot.Text != "same" {
		t.Fatalf("%#v", r)
	}
}

func TestAtomicReplacement(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "doc.txt")
	_ = os.WriteFile(p, []byte("old"), 0600)
	w := New(p, 40*time.Millisecond, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	tmp := filepath.Join(d, "replacement.tmp")
	_ = os.WriteFile(tmp, []byte("replacement"), 0600)
	if err := os.Rename(tmp, p); err != nil {
		t.Skipf("atomic replacement unsupported: %v", err)
	}
	r := waitResult(t, w.Events())
	if r.Snapshot == nil || r.Snapshot.Text != "replacement" {
		t.Fatalf("%#v", r)
	}
}

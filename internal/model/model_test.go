package model

import (
	"os"
	"path/filepath"
	"testing"

	"doppelcat/internal/document"
)

func load(t *testing.T, p string) document.Snapshot {
	t.Helper()
	s, e := document.Load(p)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestExternalAndConflict(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x")
	os.WriteFile(p, []byte("a\n"), 0600)
	m := New(p, load(t, p))
	os.WriteFile(p, []byte("b\n"), 0600)
	s := load(t, p)
	m.ApplyExternal(&s, nil)
	if m.Mode != Diff || m.Current.Text != "b\n" {
		t.Fatal("external not accepted")
	}
	m.EnterEdit()
	m.SetBuffer("local\n")
	os.WriteFile(p, []byte("remote\n"), 0600)
	r := load(t, p)
	m.ApplyExternal(&r, nil)
	if !m.Conflict || m.Buffer != "local\n" {
		t.Fatal("local edit lost")
	}
	if err := m.Overwrite(); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "local\n" {
		t.Fatalf("%q", got)
	}
}

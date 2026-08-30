package document

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	cases := [][]byte{{}, []byte("a\n"), []byte("a\r\nb\r\n"), []byte("a\r\nb\nc\r"), append([]byte{0xef, 0xbb, 0xbf}, []byte("日本語🙂\r\n")...)}
	for _, in := range cases {
		s, err := Decode(in)
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.Encode(s.Text)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, in) {
			t.Fatalf("round trip: %q != %q", got, in)
		}
	}
}

func TestMixedNewlinesFollowUnchangedLines(t *testing.T) {
	s, err := Decode([]byte("a\r\nb\nc\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Encode("new\na\nb\nc\n")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\r\na\r\nb\nc\r\n" {
		t.Fatalf("%q", got)
	}
	got, _ = s.Encode("a\nb\nc")
	if string(got) != "a\r\nb\nc\r\n" {
		t.Fatalf("trailing newline not preserved: %q", got)
	}
}
func TestRejectInvalid(t *testing.T) {
	for _, in := range [][]byte{{0xff}, {'a', 0, 'b'}} {
		if _, err := Decode(in); err == nil {
			t.Fatalf("accepted %v", in)
		}
	}
}
func TestSaveDetectsConflictAndPreservesSymlink(t *testing.T) {
	d := t.TempDir()
	real := filepath.Join(d, "real.txt")
	link := filepath.Join(d, "link.txt")
	if err := os.WriteFile(real, []byte("one\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	s, err := Load(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Save(link, "two\n", s); err != nil {
		t.Fatal(err)
	}
	if i, err := os.Lstat(link); err != nil || i.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link replaced: %v", err)
	}
	if err = os.WriteFile(real, []byte("external\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if _, err = Save(link, "local\n", s); !errors.Is(err, ErrChanged) {
		t.Fatalf("want conflict, got %v", err)
	}
}
func TestSaveFailureKeepsOriginal(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.txt")
	os.WriteFile(p, []byte("old"), 0600)
	s, _ := Load(p)
	_, err := SaveWith(p, "new", s, SaveOps{CreateTemp: os.CreateTemp, Rename: func(string, string) error { return errors.New("boom") }})
	if err == nil {
		t.Fatal("expected error")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "old" {
		t.Fatalf("got %q", got)
	}
}

func TestWriteAndSyncFailuresKeepOriginal(t *testing.T) {
	for _, stage := range []string{"write", "sync"} {
		t.Run(stage, func(t *testing.T) {
			d := t.TempDir()
			p := filepath.Join(d, "x.txt")
			_ = os.WriteFile(p, []byte("old"), 0600)
			s, _ := Load(p)
			ops := SaveOps{}
			if stage == "write" {
				ops.Write = func(*os.File, []byte) (int, error) { return 0, errors.New("write failed") }
			}
			if stage == "sync" {
				ops.Sync = func(*os.File) error { return errors.New("sync failed") }
			}
			if _, err := SaveWith(p, "new", s, ops); err == nil {
				t.Fatal("expected error")
			}
			got, _ := os.ReadFile(p)
			if string(got) != "old" {
				t.Fatalf("got %q", got)
			}
		})
	}
}

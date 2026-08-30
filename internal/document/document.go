package document

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/asurato/doppelcat/internal/diff"
)

var (
	ErrInvalidUTF8 = errors.New("file is not valid UTF-8")
	ErrBinary      = errors.New("file contains NUL bytes")
	ErrChanged     = errors.New("file changed since it was loaded")
)

type Snapshot struct {
	Path     string
	Text     string
	Hash     [sha256.Size]byte
	BOM      bool
	Breaks   []string
	Mode     os.FileMode
	Size     int64
	ModNanos int64
}

func Load(path string) (Snapshot, error) {
	resolved, err := resolveTarget(path)
	if err != nil {
		return Snapshot{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.Mode().IsRegular() {
		return Snapshot{}, fmt.Errorf("not a regular file: %s", path)
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		return Snapshot{}, err
	}
	s, err := Decode(b)
	if err != nil {
		return Snapshot{}, err
	}
	s.Path, s.Mode, s.Size, s.ModNanos = resolved, info.Mode(), info.Size(), info.ModTime().UnixNano()
	return s, nil
}

func Decode(data []byte) (Snapshot, error) {
	s := Snapshot{Hash: sha256.Sum256(data), Size: int64(len(data))}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		s.BOM = true
		data = data[3:]
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return Snapshot{}, ErrBinary
	}
	if !utf8.Valid(data) {
		return Snapshot{}, ErrInvalidUTF8
	}
	var text strings.Builder
	text.Grow(len(data))
	for i := 0; i < len(data); {
		switch data[i] {
		case '\r':
			if i+1 < len(data) && data[i+1] == '\n' {
				s.Breaks = append(s.Breaks, "\r\n")
				i += 2
			} else {
				s.Breaks = append(s.Breaks, "\r")
				i++
			}
			text.WriteByte('\n')
		case '\n':
			s.Breaks = append(s.Breaks, "\n")
			text.WriteByte('\n')
			i++
		default:
			text.WriteByte(data[i])
			i++
		}
	}
	s.Text = text.String()
	return s, nil
}

func (s Snapshot) Encode(text string) ([]byte, error) {
	if strings.IndexByte(text, 0) >= 0 {
		return nil, ErrBinary
	}
	if !utf8.ValidString(text) {
		return nil, ErrInvalidUTF8
	}
	wantTrailing := strings.HasSuffix(s.Text, "\n")
	text = strings.TrimSuffix(text, "\n")
	if wantTrailing {
		text += "\n"
	}
	majority := "\n"
	counts := map[string]int{}
	for _, br := range s.Breaks {
		counts[br]++
		if counts[br] > counts[majority] {
			majority = br
		}
	}
	var out bytes.Buffer
	if s.BOM {
		out.Write([]byte{0xef, 0xbb, 0xbf})
	}
	breaks := make([]string, 0, strings.Count(text, "\n"))
	for _, line := range mappedLines(s.Text, text) {
		if !line.hasBreak {
			continue
		}
		br := majority
		if line.oldIndex >= 0 && line.oldIndex < len(s.Breaks) {
			br = s.Breaks[line.oldIndex]
		}
		breaks = append(breaks, br)
	}
	breakIndex := 0
	for _, r := range text {
		if r == '\n' {
			out.WriteString(breaks[breakIndex])
			breakIndex++
		} else {
			out.WriteRune(r)
		}
	}
	return out.Bytes(), nil
}

type mappedLine struct {
	oldIndex int
	hasBreak bool
}

func mappedLines(oldText, newText string) []mappedLine {
	lines, _ := diff.Lines(oldText, newText)
	newCount := len(diff.Split(newText))
	result := make([]mappedLine, 0, newCount)
	oldIndex := 0
	for _, line := range lines {
		switch line.Kind {
		case diff.Equal:
			result = append(result, mappedLine{oldIndex: oldIndex})
			oldIndex++
		case diff.Delete:
			oldIndex++
		case diff.Insert:
			result = append(result, mappedLine{oldIndex: -1})
		}
	}
	for len(result) < newCount {
		result = append(result, mappedLine{oldIndex: -1})
	}
	breakCount := strings.Count(newText, "\n")
	for i := range result {
		result[i].hasBreak = i < breakCount
	}
	return result
}

func CurrentHash(path string) ([sha256.Size]byte, error) {
	resolved, err := resolveTarget(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	return sha256.Sum256(b), nil
}

type SaveOps struct {
	CreateTemp func(string, string) (*os.File, error)
	Chmod      func(*os.File, os.FileMode) error
	Write      func(*os.File, []byte) (int, error)
	Sync       func(*os.File) error
	Close      func(*os.File) error
	Rename     func(string, string) error
	Remove     func(string) error
}

func Save(path, text string, base Snapshot) (Snapshot, error) {
	return SaveWith(path, text, base, SaveOps{})
}

func SaveWith(path, text string, base Snapshot, ops SaveOps) (Snapshot, error) {
	if ops.CreateTemp == nil {
		ops.CreateTemp = os.CreateTemp
	}
	if ops.Chmod == nil {
		ops.Chmod = func(f *os.File, mode os.FileMode) error { return f.Chmod(mode) }
	}
	if ops.Write == nil {
		ops.Write = func(f *os.File, b []byte) (int, error) { return f.Write(b) }
	}
	if ops.Sync == nil {
		ops.Sync = func(f *os.File) error { return f.Sync() }
	}
	if ops.Close == nil {
		ops.Close = func(f *os.File) error { return f.Close() }
	}
	if ops.Rename == nil {
		ops.Rename = os.Rename
	}
	if ops.Remove == nil {
		ops.Remove = os.Remove
	}
	resolved, err := resolveTarget(path)
	if err != nil {
		return Snapshot{}, err
	}
	actual, err := CurrentHash(resolved)
	if err != nil {
		return Snapshot{}, err
	}
	if actual != base.Hash {
		return Snapshot{}, ErrChanged
	}
	data, err := base.Encode(text)
	if err != nil {
		return Snapshot{}, err
	}
	tmp, err := ops.CreateTemp(filepath.Dir(resolved), ".doppelcat-*")
	if err != nil {
		return Snapshot{}, err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = ops.Remove(tmpName)
		}
	}()
	if err = ops.Chmod(tmp, base.Mode.Perm()); err == nil {
		_, err = ops.Write(tmp, data)
	}
	if err == nil {
		err = ops.Sync(tmp)
	}
	closeErr := ops.Close(tmp)
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return Snapshot{}, err
	}
	if err = ops.Rename(tmpName, resolved); err != nil {
		return Snapshot{}, err
	}
	ok = true
	return Load(path)
}

func resolveTarget(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

package model

import (
	"errors"
	"os"

	"github.com/asurato/doppelcat/internal/diff"
	"github.com/asurato/doppelcat/internal/document"
)

type Mode int

const (
	Normal Mode = iota
	Diff
	Edit
)

type Availability int

const (
	Available Availability = iota
	Unavailable
)

type Model struct {
	Path              string
	Mode              Mode
	Availability      Availability
	Previous, Current document.Snapshot
	DiffLines         []diff.Line
	Buffer            string
	Dirty             bool
	Conflict          bool
	Pending           *document.Snapshot
	StatusError       error
}

func New(path string, s document.Snapshot) *Model {
	return &Model{Path: path, Current: s, Buffer: s.Text}
}

func (m *Model) EnterEdit() {
	if m.Availability == Available {
		m.Mode = Edit
		if !m.Dirty {
			m.Buffer = m.Current.Text
		}
	}
}
func (m *Model) SetBuffer(s string) { m.Buffer = s; m.Dirty = s != m.Current.Text }

func (m *Model) ApplyExternal(s *document.Snapshot, err error) {
	if err != nil || s == nil {
		m.Availability = Unavailable
		m.StatusError = err
		return
	}
	m.StatusError = nil
	if s.Hash == m.Current.Hash && m.Availability == Available {
		return
	}
	wasUnavailable := m.Availability == Unavailable
	m.Availability = Available
	if m.Dirty {
		cp := *s
		m.Pending = &cp
		m.Conflict = true
		if wasUnavailable {
			m.StatusError = errors.New("file reappeared with external content")
		}
		return
	}
	m.accept(*s)
}

func (m *Model) accept(s document.Snapshot) {
	m.Previous = m.Current
	m.Current = s
	m.Buffer = s.Text
	m.Dirty = false
	m.Conflict = false
	m.Pending = nil
	m.DiffLines, _ = diff.Lines(m.Previous.Text, m.Current.Text)
	m.Mode = Diff
}

func (m *Model) ReloadExternal() bool {
	if m.Pending == nil {
		return false
	}
	s := *m.Pending
	m.accept(s)
	return true
}

func (m *Model) Save() error {
	if m.Availability != Available {
		return errors.New("file is unavailable")
	}
	if m.Conflict {
		return document.ErrChanged
	}
	before := m.Current
	saved, err := document.Save(m.Path, m.Buffer, m.Current)
	if err != nil {
		return err
	}
	m.Previous = before
	m.Current = saved
	m.Buffer = saved.Text
	m.Dirty = false
	m.Conflict = false
	m.Pending = nil
	m.DiffLines, _ = diff.Lines(before.Text, saved.Text)
	m.Mode = Diff
	return nil
}

func (m *Model) Overwrite() error {
	if m.Pending == nil || m.Availability != Available {
		return errors.New("no external version available")
	}
	before := *m.Pending
	saved, err := document.Save(m.Path, m.Buffer, before)
	if err != nil {
		return err
	}
	m.Previous = before
	m.Current = saved
	m.Buffer = saved.Text
	m.Dirty = false
	m.Conflict = false
	m.Pending = nil
	m.DiffLines, _ = diff.Lines(before.Text, saved.Text)
	m.Mode = Diff
	return nil
}

func (m *Model) Missing() bool {
	return m.Availability == Unavailable && errors.Is(m.StatusError, os.ErrNotExist)
}

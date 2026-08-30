package watch

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/asurato/doppelcat/internal/document"
	"github.com/fsnotify/fsnotify"
)

type Result struct {
	Snapshot *document.Snapshot
	Err      error
	Missing  bool
}

type Watcher struct {
	path           string
	debounce, poll time.Duration
	out            chan Result
	cancel         context.CancelFunc
	once           sync.Once
}

func New(path string, debounce, poll time.Duration) *Watcher {
	if debounce <= 0 {
		debounce = 200 * time.Millisecond
	}
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	return &Watcher{path: path, debounce: debounce, poll: poll, out: make(chan Result, 8)}
}

func (w *Watcher) Events() <-chan Result { return w.out }

func (w *Watcher) Start(parent context.Context) error {
	abs, err := filepath.Abs(w.path)
	if err != nil {
		return err
	}
	w.path = abs
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = fw.Add(filepath.Dir(abs)); err != nil {
		_ = fw.Close()
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	go w.run(ctx, fw)
	return nil
}

func (w *Watcher) Close() {
	w.once.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
	})
}

type fingerprint struct {
	exists bool
	size   int64
	mod    int64
	mode   os.FileMode
}

func statFingerprint(path string) fingerprint {
	i, err := os.Stat(path)
	if err != nil {
		return fingerprint{}
	}
	return fingerprint{true, i.Size(), i.ModTime().UnixNano(), i.Mode()}
}

func (w *Watcher) run(ctx context.Context, fw *fsnotify.Watcher) {
	defer close(w.out)
	defer fw.Close()
	poll := time.NewTicker(w.poll)
	defer poll.Stop()
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	lastFP := statFingerprint(w.path)
	var lastHash [sha256.Size]byte
	haveHash := false
	pending := false
	schedule := func() {
		pending = true
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(w.debounce)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-fw.Events:
			if !ok {
				return
			}
			if filepath.Clean(ev.Name) == filepath.Clean(w.path) {
				schedule()
			}
		case _, ok := <-fw.Errors:
			if !ok {
				return
			}
			schedule()
		case <-poll.C:
			fp := statFingerprint(w.path)
			if fp != lastFP {
				lastFP = fp
				schedule()
			}
		case <-timer.C:
			if !pending {
				continue
			}
			pending = false
			snap, err := stableLoad(w.path)
			if err != nil {
				missing := errors.Is(err, os.ErrNotExist)
				haveHash = false
				w.send(ctx, Result{Err: err, Missing: missing})
				if !missing && !errors.Is(err, document.ErrInvalidUTF8) && !errors.Is(err, document.ErrBinary) {
					schedule()
				}
				continue
			}
			lastFP = statFingerprint(w.path)
			if haveHash && snap.Hash == lastHash {
				continue
			}
			lastHash = snap.Hash
			haveHash = true
			w.send(ctx, Result{Snapshot: &snap})
		}
	}
}

func stableLoad(path string) (document.Snapshot, error) {
	var last error
	for i := 0; i < 4; i++ {
		before := statFingerprint(path)
		if !before.exists {
			return document.Snapshot{}, os.ErrNotExist
		}
		s, err := document.Load(path)
		after := statFingerprint(path)
		if err == nil && before == after {
			return s, nil
		}
		last = err
		if last == nil {
			last = errors.New("file changed while reading")
		}
		time.Sleep(25 * time.Millisecond)
	}
	return document.Snapshot{}, last
}

func (w *Watcher) send(ctx context.Context, r Result) {
	select {
	case w.out <- r:
	case <-ctx.Done():
	}
}

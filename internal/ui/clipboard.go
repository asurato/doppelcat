package ui

import (
	"context"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

type clipboardBridge struct {
	mu        sync.RWMutex
	internal  string
	available bool
	warn      func(string)
}

func newClipboard(warn func(string)) *clipboardBridge {
	c := &clipboardBridge{warn: warn}
	if err := clipboard.Init(); err != nil {
		warn("OS clipboard unavailable; using internal clipboard")
	} else {
		c.available = true
	}
	return c
}
func (c *clipboardBridge) copy(s string) {
	c.mu.Lock()
	c.internal = s
	c.mu.Unlock()
	if !c.available {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := clipboard.Write(ctx, clipboard.FmtText, []byte(s)); err != nil {
		c.warn("clipboard write timed out; internal copy kept")
	}
}
func (c *clipboardBridge) paste() string {
	c.mu.RLock()
	fallback := c.internal
	c.mu.RUnlock()
	if !c.available {
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	b, err := clipboard.Read(ctx, clipboard.FmtText)
	if err != nil {
		c.warn("clipboard read timed out; using internal clipboard")
		return fallback
	}
	if len(b) > 0 {
		return string(b)
	}
	return fallback
}

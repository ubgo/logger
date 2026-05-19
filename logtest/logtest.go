// Package logtest provides a capturing sink and assertion helpers so logging
// is testable: assert what was logged, at which level, with which fields —
// and optionally fail a test on any unexpected ERROR.
package logtest

import (
	"strings"
	"sync"
	"testing"

	logger "github.com/ubgo/logger"
)

// Entry is a captured record copied out of the pooled *Record.
type Entry struct {
	Level   logger.Level
	Message string
	Fields  map[string]any
}

// Capture is an in-memory logger.Sink recording every emitted record.
type Capture struct {
	mu      sync.Mutex
	entries []Entry
}

// Emit implements logger.Sink.
func (c *Capture) Emit(r *logger.Record) error {
	e := Entry{Level: r.Level, Message: r.Message, Fields: map[string]any{}}
	for _, f := range r.Fields {
		e.Fields[f.Key] = f.Value()
	}
	c.mu.Lock()
	c.entries = append(c.entries, e)
	c.mu.Unlock()
	return nil
}

// Sync implements logger.Sink.
func (c *Capture) Sync() error { return nil }

// Close implements logger.Sink.
func (c *Capture) Close() error { return nil }

// Entries returns a copy of everything captured so far.
func (c *Capture) Entries() []Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Entry(nil), c.entries...)
}

// New returns a Trace-level logger writing into a fresh Capture. Inline
// (Sync) transport so assertions see records immediately.
func New() (*logger.Logger, *Capture) {
	c := &Capture{}
	l := logger.New(
		logger.WithLevel(logger.LevelTrace),
		logger.WithSink(c),
	)
	return l, c
}

// AssertLogged fails t unless some entry at exactly level has a message
// containing msgSubstr.
func (c *Capture) AssertLogged(t testing.TB, level logger.Level, msgSubstr string) {
	t.Helper()
	for _, e := range c.Entries() {
		if e.Level == level && strings.Contains(e.Message, msgSubstr) {
			return
		}
	}
	t.Fatalf("expected a %s log containing %q; got %d entries: %v",
		level, msgSubstr, len(c.entries), c.Entries())
}

// AssertField fails t unless some entry has field key == value.
func (c *Capture) AssertField(t testing.TB, key string, value any) {
	t.Helper()
	for _, e := range c.Entries() {
		if v, ok := e.Fields[key]; ok && v == value {
			return
		}
	}
	t.Fatalf("expected an entry with field %s=%v; got %v", key, value, c.Entries())
}

// AssertNoErrors fails t if any entry was at ERROR or above — opt-in
// "tests must not log errors" guard.
func (c *Capture) AssertNoErrors(t testing.TB) {
	t.Helper()
	for _, e := range c.Entries() {
		if e.Level >= logger.LevelError {
			t.Fatalf("unexpected %s log in test: %q %v", e.Level, e.Message, e.Fields)
		}
	}
}

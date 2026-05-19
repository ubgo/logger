package logger

import (
	"context"
	"sync"
	"time"
)

// DedupProcessor throttles identical (level + message) records: the first in
// a window passes (annotated with how many were suppressed since the last
// pass), the rest are dropped. Stops one hot error from burying every other
// line, without silently losing the fact that it happened.
type DedupProcessor struct {
	Window time.Duration // default 5s
	mu     sync.Mutex
	seen   map[string]*dedupState
}

type dedupState struct {
	last       time.Time
	suppressed uint64
}

// NewDedupProcessor builds a deduper with the given window.
func NewDedupProcessor(window time.Duration) *DedupProcessor {
	if window <= 0 {
		window = 5 * time.Second
	}
	return &DedupProcessor{Window: window, seen: make(map[string]*dedupState)}
}

// Process implements Processor.
func (d *DedupProcessor) Process(_ context.Context, r *Record) error {
	key := r.Level.String() + "\x00" + r.Message
	now := timeNow()

	d.mu.Lock()
	st := d.seen[key]
	if st == nil {
		st = &dedupState{}
		d.seen[key] = st
	}
	if !st.last.IsZero() && now.Sub(st.last) < d.Window {
		st.suppressed++
		d.mu.Unlock()
		return ErrDrop
	}
	n := st.suppressed
	st.suppressed = 0
	st.last = now
	d.mu.Unlock()

	if n > 0 {
		r.Fields = append(r.Fields, Int("deduped_count", n))
	}
	return nil
}

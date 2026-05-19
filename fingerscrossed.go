package logger

import (
	"context"
	"sync"
)

// FingersCrossed is a Sink decorator implementing debug-on-error buffering
// (Monolog's signature, near-absent in Go): records below ActivationLevel are
// held in a bounded ring and emitted ONLY if a record at/above
// ActivationLevel occurs in the same scope — then the whole buffered trail is
// flushed for full failure context. Successful scopes pay ~nothing and emit
// nothing below the activation level.
//
// Scope it per request with FCScope(ctx); without a scope it falls back to a
// single process-global ring (still correct, less precise).
type FingersCrossed struct {
	inner      Sink
	Activation Level // default LevelError
	BufferSize int   // per-scope ring capacity, default 256
	// PassThrough emits records at/above this level even when not activated
	// (so WARN-and-below stay buffered but you still see, e.g., nothing until
	// error). Default 0 = nothing passes until activation.
	PassThrough Level

	global *fcBuffer
}

type fcBuffer struct {
	mu        sync.Mutex
	ring      []*Record
	start     int
	count     int
	activated bool
	cap       int
}

func newFCBuffer(capacity int) *fcBuffer {
	if capacity < 1 {
		capacity = 256
	}
	return &fcBuffer{ring: make([]*Record, capacity), cap: capacity}
}

func (b *fcBuffer) push(r *Record) {
	idx := (b.start + b.count) % b.cap
	if b.count == b.cap { // full: overwrite oldest
		b.ring[b.start] = r
		b.start = (b.start + 1) % b.cap
	} else {
		b.ring[idx] = r
		b.count++
	}
}

func (b *fcBuffer) drain() []*Record {
	out := make([]*Record, 0, b.count)
	for i := 0; i < b.count; i++ {
		out = append(out, b.ring[(b.start+i)%b.cap])
	}
	b.start, b.count = 0, 0
	return out
}

type fcKey struct{}

// FCScope returns a context carrying a fresh per-scope FingersCrossed buffer.
// Call it at the start of a request; all logs made with this ctx share the
// buffer and flush together on the first error.
func FCScope(ctx context.Context) context.Context {
	return context.WithValue(ctx, fcKey{}, newFCBuffer(256))
}

// NewFingersCrossed wraps inner with debug-on-error buffering.
func NewFingersCrossed(inner Sink) *FingersCrossed {
	return &FingersCrossed{
		inner:      inner,
		Activation: LevelError,
		BufferSize: 256,
		global:     newFCBuffer(256),
	}
}

func (f *FingersCrossed) bufFor(r *Record) *fcBuffer {
	if r.Ctx != nil {
		if b, ok := r.Ctx.Value(fcKey{}).(*fcBuffer); ok {
			return b
		}
	}
	return f.global
}

// Emit implements Sink.
func (f *FingersCrossed) Emit(r *Record) error {
	act := f.Activation
	if act == 0 {
		act = LevelError
	}
	b := f.bufFor(r)
	b.mu.Lock()

	if b.activated {
		b.mu.Unlock()
		return f.inner.Emit(r)
	}
	if r.Level >= act {
		// activation: flush the buffered trail, then this record, latch open
		b.activated = true
		held := b.drain()
		b.mu.Unlock()
		for _, hr := range held {
			_ = f.inner.Emit(hr)
		}
		return f.inner.Emit(r)
	}
	if f.PassThrough != 0 && r.Level >= f.PassThrough {
		b.mu.Unlock()
		return f.inner.Emit(r)
	}
	// below activation, not activated: buffer a copy (record is pooled)
	b.push(r.Clone())
	b.mu.Unlock()
	return nil
}

// Sync implements Sink.
func (f *FingersCrossed) Sync() error { return f.inner.Sync() }

// Close flushes any still-buffered global records (best effort) and closes
// inner. Scoped buffers that never activated are intentionally discarded —
// that is the whole point (no error ⇒ no debug noise).
func (f *FingersCrossed) Close() error {
	return f.inner.Close()
}

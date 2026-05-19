package logger

import (
	"context"
	"sync"
	"time"
)

// Record is one log event flowing through the processor pipeline. It is
// pooled: never retain a *Record past the call that produced it. Copy fields
// out (or call Clone) if you must keep them (e.g. async transport, buffering).
type Record struct {
	Time    time.Time
	Level   Level
	Message string
	// PC is the caller program counter (0 if caller capture is disabled);
	// resolved lazily so the cost is only paid when actually formatted.
	PC uint64
	// EventName, when set, is the stable identity of a named typed event
	// (mulog "events not messages" + OTEL EventName). Use it instead of
	// prose for analytics/AI-friendly logs.
	EventName string
	// Fields are this event's own attributes plus any inherited via With().
	Fields []Field
	// Ctx carries request-scoped values + the active trace span. Never nil
	// in the pipeline (defaults to context.Background()).
	Ctx context.Context

	pooled bool
}

var recordPool = sync.Pool{New: func() any { return &Record{} }}

func newRecord() *Record {
	r := recordPool.Get().(*Record)
	r.pooled = true
	return r
}

func (r *Record) reset() {
	r.Time = time.Time{}
	r.Level = 0
	r.Message = ""
	r.EventName = ""
	r.PC = 0
	r.Fields = r.Fields[:0]
	r.Ctx = nil
}

func (r *Record) release() {
	if !r.pooled {
		return
	}
	r.reset()
	recordPool.Put(r)
}

// Clone returns a heap copy safe to retain after the pipeline returns. The
// async transports call this before handing a record to a worker goroutine.
func (r *Record) Clone() *Record {
	c := &Record{
		Time:      r.Time,
		Level:     r.Level,
		Message:   r.Message,
		EventName: r.EventName,
		PC:        r.PC,
		Ctx:       r.Ctx,
	}
	c.Fields = append(make([]Field, 0, len(r.Fields)), r.Fields...)
	return c
}

// AddField appends a field in place (used by enrich processors).
func (r *Record) AddField(f Field) { r.Fields = append(r.Fields, f) }

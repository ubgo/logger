package logger

import (
	"io"
	"sync"
)

// Sink is a terminal destination. Each sink owns its level + encoder so a
// fan-out can send the same record to console(debug,pretty) and
// file(info,json) simultaneously. A sink that errors must not block or kill
// sibling sinks (see Fanout).
type Sink interface {
	// Emit writes r if r.Level >= the sink's level.
	Emit(r *Record) error
	// Sync flushes buffered data.
	Sync() error
	// Close flushes and releases resources.
	Close() error
}

// WriterSink adapts any io.Writer + Encoder into a Sink with its own level.
type WriterSink struct {
	W      io.Writer
	Enc    Encoder
	MinLvl Leveler
	mu     sync.Mutex
}

// NewWriterSink builds a WriterSink. minLevel may be a *LevelVar for runtime
// control or a constant Level.
func NewWriterSink(w io.Writer, enc Encoder, minLevel Level) *WriterSink {
	return &WriterSink{W: w, Enc: enc, MinLvl: staticLevel(minLevel)}
}

// Emit implements Sink.
func (s *WriterSink) Emit(r *Record) error {
	if r.Level < s.MinLvl.Level() {
		return nil
	}
	buf := getBuffer()
	s.Enc.Encode(buf, r)
	s.mu.Lock()
	_, err := s.W.Write(buf.b)
	s.mu.Unlock()
	putBuffer(buf)
	return err
}

// Sync implements Sink (best-effort flush for Syncer writers).
func (s *WriterSink) Sync() error {
	if sy, ok := s.W.(interface{ Sync() error }); ok {
		return sy.Sync()
	}
	return nil
}

// Close implements Sink.
func (s *WriterSink) Close() error {
	if c, ok := s.W.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Fanout broadcasts a record to N sinks with per-sink failure isolation: one
// sink returning an error never prevents the others from receiving the record.
type Fanout struct {
	sinks   []Sink
	OnError func(Sink, error)
}

// NewFanout groups sinks.
func NewFanout(sinks ...Sink) *Fanout { return &Fanout{sinks: sinks} }

// Emit implements Sink.
func (f *Fanout) Emit(r *Record) error {
	for _, s := range f.sinks {
		if err := s.Emit(r); err != nil && f.OnError != nil {
			f.OnError(s, err)
		}
	}
	return nil
}

// Sync implements Sink.
func (f *Fanout) Sync() error {
	var first error
	for _, s := range f.sinks {
		if err := s.Sync(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Close implements Sink.
func (f *Fanout) Close() error {
	var first error
	for _, s := range f.sinks {
		if err := s.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

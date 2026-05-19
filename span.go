package logger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Span is scoped structured context with an outcome — the tracing-rs "spans
// as context" + Eliot "causal action tree" idea, logging-native (no separate
// tracer). Every log made with the span's ctx inherits its fields and a
// hierarchical span path, so a flat log stream reconstructs the tree of what
// happened and why. End/Fail emit a single duration+outcome record.
type Span struct {
	l        *Logger
	name     string
	id       string
	parentID string
	path     string // Eliot-style task_level, e.g. "1.2.1"
	start    time.Time
	fields   []Field
	level    Level // level for the span.end record (raised to Error on Fail)
	err      error
	ended    bool
}

type spanKey struct{}

type spanState struct {
	id       string
	path     string
	children int
}

func shortID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// StartSpan opens a span. The returned context must be used for all logs that
// should belong to it (and passed to child StartSpan calls to nest). Always
// pair with defer span.End() (or span.Fail).
//
//	ctx, span := log.StartSpan(ctx, "handle_order", logger.String("order", id))
//	defer span.End()
func (l *Logger) StartSpan(ctx context.Context, name string, fields ...Field) (context.Context, *Span) {
	if ctx == nil {
		ctx = context.Background()
	}
	parent, _ := ctx.Value(spanKey{}).(*spanState)
	s := &Span{
		l:      l,
		name:   name,
		id:     shortID(),
		start:  timeNow(),
		fields: fields,
		level:  LevelInfo,
	}
	if parent != nil {
		parent.children++
		s.parentID = parent.id
		s.path = parent.path + "." + itoa(parent.children)
	} else {
		s.path = "1"
	}
	st := &spanState{id: s.id, path: s.path}
	ctx = context.WithValue(ctx, spanKey{}, st)
	// Child logs inherit span identity + the span's bound fields.
	bound := append([]Field{
		String("span", name),
		String("span_id", s.id),
		String("span_path", s.path),
	}, fields...)
	if s.parentID != "" {
		bound = append(bound, String("parent_span_id", s.parentID))
	}
	ctx = ContextWith(ctx, bound...)
	return ctx, s
}

// Fail marks the span as failed; End will emit at ERROR with the error.
func (s *Span) Fail(err error) { s.err = err; s.level = LevelError }

// SetLevel overrides the level of the span.end record (default Info / Error
// if Fail was called).
func (s *Span) SetLevel(l Level) { s.level = l }

// End emits the single span-completion record with duration + outcome. Safe
// to call once; subsequent calls are no-ops.
func (s *Span) End() {
	if s.ended {
		return
	}
	s.ended = true
	dur := timeNow().Sub(s.start)
	fields := make([]Field, 0, len(s.fields)+6)
	fields = append(fields,
		String("span", s.name),
		String("span_id", s.id),
		String("span_path", s.path),
		Dur("duration", dur),
		Bool("ok", s.err == nil),
	)
	if s.parentID != "" {
		fields = append(fields, String("parent_span_id", s.parentID))
	}
	if s.err != nil {
		fields = append(fields, NamedErr("error", s.err))
	}
	fields = append(fields, s.fields...)
	s.l.log(context.Background(), s.level, "span.end", fields)
}

// itoa is a tiny no-import int→string for the span path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

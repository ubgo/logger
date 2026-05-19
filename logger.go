// Package logger is a pluggable, adapter-based, slog-native structured logging
// core: a correct slog.Handler, a single processor pipeline as the only
// extension seam, swappable async transports with explicit backpressure, and
// batteries (rotation, redaction, sampling, OTEL correlation) built in.
//
// See MISSION.md / FEATURES.md in the plan repo for the design rationale.
package logger

import (
	"context"
	"os"
	"runtime"
)

// Logger is the core. It is immutable after construction except via With,
// which returns a child sharing the same pipeline + transport but with extra
// bound fields. Safe for concurrent use.
type Logger struct {
	leveler    Leveler
	processors []Processor
	transport  Transport
	with       []Field // inherited fields, prepended to every record
	addCaller  bool
	callerSkip int
	metrics    *Metrics
}

// Option configures a Logger at construction.
type Option func(*config)

type config struct {
	leveler    Leveler
	processors []Processor
	transport  Transport
	sink       Sink
	addCaller  bool
	callerSkip int
}

// WithLevel sets a constant minimum level.
func WithLevel(l Level) Option { return func(c *config) { c.leveler = staticLevel(l) } }

// WithLeveler sets a dynamic level source (e.g. *LevelVar) for runtime control.
func WithLeveler(lv Leveler) Option { return func(c *config) { c.leveler = lv } }

// WithProcessors sets the pipeline (enrich/redact/sample/...). Order matters.
func WithProcessors(ps ...Processor) Option {
	return func(c *config) { c.processors = ps }
}

// WithSink sets the terminal destination, delivered inline (SyncTransport).
// Use WithTransport for async.
func WithSink(s Sink) Option { return func(c *config) { c.sink = s } }

// WithTransport sets an explicit transport (Sync/Channel/Ring). Overrides
// WithSink.
func WithTransport(t Transport) Option { return func(c *config) { c.transport = t } }

// WithCaller enables file:line capture (lazily resolved by encoders).
func WithCaller(skip int) Option {
	return func(c *config) { c.addCaller = true; c.callerSkip = skip }
}

// New builds a Logger. With no options it logs JSON at Info to stderr inline.
func New(opts ...Option) *Logger {
	c := &config{leveler: staticLevel(LevelInfo)}
	for _, o := range opts {
		o(c)
	}
	if c.transport == nil {
		sink := c.sink
		if sink == nil {
			sink = NewWriterSink(os.Stderr, NewJSONEncoder(), LevelTrace)
		}
		c.transport = NewSyncTransport(sink)
	}
	return &Logger{
		leveler:    c.leveler,
		processors: c.processors,
		transport:  c.transport,
		addCaller:  c.addCaller,
		callerSkip: c.callerSkip,
		metrics:    &Metrics{},
	}
}

// Metrics returns the logger's self-observability counters (emitted/dropped/
// sink-errors/by-level). Child loggers from With share the parent's metrics.
func (l *Logger) Metrics() *Metrics { return l.metrics }

// Enabled reports whether a record at level l would be emitted. Call this to
// guard expensive field construction.
func (l *Logger) Enabled(level Level) bool { return level >= l.leveler.Level() }

// With returns a child logger that prepends fields to every record. Inherited
// fields are copied so the parent is unaffected.
func (l *Logger) With(fields ...Field) *Logger {
	nl := *l
	nl.with = append(append(make([]Field, 0, len(l.with)+len(fields)), l.with...), fields...)
	return &nl
}

// log is the single emission path used by every level method.
func (l *Logger) log(ctx context.Context, level Level, msg string, fields []Field) {
	if level < l.leveler.Level() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r := newRecord()
	r.Time = timeNow()
	r.Level = level
	r.Message = msg
	r.Ctx = ctx
	if len(l.with) > 0 {
		r.Fields = append(r.Fields, l.with...)
	}
	r.Fields = append(r.Fields, fields...)
	if l.addCaller {
		var pcs [1]uintptr
		// skip: runtime.Callers, this fn, level method, user frame
		runtime.Callers(3+l.callerSkip, pcs[:])
		r.PC = uint64(pcs[0])
	}
	if err := runPipeline(l.processors, ctx, r); err != nil {
		if l.metrics != nil {
			l.metrics.incDropped()
		}
		r.release() // dropped or failed: do not emit
		return
	}
	if l.metrics != nil {
		l.metrics.incEmitted(level)
	}
	l.transport.Dispatch(r)
	r.release()
}

// Leveled methods. The Context variants carry trace correlation + scoped KV.

func (l *Logger) Trace(msg string, f ...Field) { l.log(context.Background(), LevelTrace, msg, f) }
func (l *Logger) Debug(msg string, f ...Field) { l.log(context.Background(), LevelDebug, msg, f) }
func (l *Logger) Info(msg string, f ...Field)  { l.log(context.Background(), LevelInfo, msg, f) }
func (l *Logger) Warn(msg string, f ...Field)  { l.log(context.Background(), LevelWarn, msg, f) }
func (l *Logger) Error(msg string, f ...Field) { l.log(context.Background(), LevelError, msg, f) }

func (l *Logger) TraceContext(ctx context.Context, msg string, f ...Field) {
	l.log(ctx, LevelTrace, msg, f)
}
func (l *Logger) DebugContext(ctx context.Context, msg string, f ...Field) {
	l.log(ctx, LevelDebug, msg, f)
}
func (l *Logger) InfoContext(ctx context.Context, msg string, f ...Field) {
	l.log(ctx, LevelInfo, msg, f)
}
func (l *Logger) WarnContext(ctx context.Context, msg string, f ...Field) {
	l.log(ctx, LevelWarn, msg, f)
}
func (l *Logger) ErrorContext(ctx context.Context, msg string, f ...Field) {
	l.log(ctx, LevelError, msg, f)
}

// Log emits at an arbitrary (possibly custom) level.
func (l *Logger) Log(ctx context.Context, level Level, msg string, f ...Field) {
	l.log(ctx, level, msg, f)
}

// Event logs a named typed event with NO message (mulog "events not
// messages"): the event name is the primary index — better for analytics and
// AI consumption than free-form prose. Defaults to Info; use EventAt for a
// level/ctx.
func (l *Logger) Event(name string, f ...Field) {
	l.eventAt(context.Background(), LevelInfo, name, f)
}

// EventAt logs a named event at an explicit level with context.
func (l *Logger) EventAt(ctx context.Context, level Level, name string, f ...Field) {
	l.eventAt(ctx, level, name, f)
}

func (l *Logger) eventAt(ctx context.Context, level Level, name string, fields []Field) {
	if level < l.leveler.Level() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r := newRecord()
	r.Time = timeNow()
	r.Level = level
	r.EventName = name
	r.Ctx = ctx
	if len(l.with) > 0 {
		r.Fields = append(r.Fields, l.with...)
	}
	r.Fields = append(r.Fields, fields...)
	if err := runPipeline(l.processors, ctx, r); err != nil {
		if l.metrics != nil {
			l.metrics.incDropped()
		}
		r.release()
		return
	}
	if l.metrics != nil {
		l.metrics.incEmitted(level)
	}
	l.transport.Dispatch(r)
	r.release()
}

// Sync flushes buffered records (call before exit).
func (l *Logger) Sync() error { return l.transport.Sync() }

// Close drains async transports and closes sinks.
func (l *Logger) Close() error { return l.transport.Close() }

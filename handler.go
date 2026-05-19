package logger

import (
	"context"
	"log/slog"
	"time"
)

// slogHandler is a correct slog.Handler over *Logger. It threads WithGroup /
// WithAttrs state properly (group-prefixed keys, preformatted inherited
// attrs) — the part most third-party handlers get subtly wrong, shipped right
// once here so users never hand-roll it.
type slogHandler struct {
	l       *Logger
	groups  []string // open group names, in order
	prefix  string   // precomputed "g1.g2." for the current group path
	preform []Field  // attrs added via WithAttrs, already prefixed
}

// Handler returns an slog.Handler backed by this Logger, so the whole
// slog ecosystem (samber/slog-*, otelslog) composes on top of us.
func (l *Logger) Handler() slog.Handler { return &slogHandler{l: l} }

// NewSlog returns an *slog.Logger writing through this Logger.
func (l *Logger) NewSlog() *slog.Logger { return slog.New(l.Handler()) }

// slogToLevel maps slog's level scale onto the OTEL SeverityNumber bands.
func slogToLevel(sl slog.Level) Level {
	switch {
	case sl < slog.LevelDebug:
		return LevelTrace
	case sl < slog.LevelInfo:
		return LevelDebug
	case sl < slog.LevelWarn:
		return LevelInfo
	case sl < slog.LevelError:
		return LevelWarn
	default:
		return LevelError
	}
}

// Enabled implements slog.Handler.
func (h *slogHandler) Enabled(_ context.Context, sl slog.Level) bool {
	return h.l.Enabled(slogToLevel(sl))
}

// Handle implements slog.Handler.
func (h *slogHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := make([]Field, 0, len(h.preform)+r.NumAttrs())
	fields = append(fields, h.preform...)
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&fields, h.prefix, a)
		return true
	})
	// Preserve slog's timestamp verbatim — including the zero time, which
	// encoders must omit (slogtest conformance requires no "time" key then).
	h.l.logAt(ctx, r.Time, slogToLevel(r.Level), r.Message, fields)
	return nil
}

// WithAttrs implements slog.Handler: attrs are preformatted with the current
// group prefix and inherited by every subsequent record.
func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	nh := h.clone()
	for _, a := range attrs {
		appendAttr(&nh.preform, nh.prefix, a)
	}
	return nh
}

// WithGroup implements slog.Handler: opens a namespace so later attrs/keys are
// prefixed "name.".
func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	nh := h.clone()
	nh.groups = append(nh.groups, name)
	nh.prefix = h.prefix + name + "."
	return nh
}

func (h *slogHandler) clone() *slogHandler {
	return &slogHandler{
		l:       h.l,
		groups:  append([]string(nil), h.groups...),
		prefix:  h.prefix,
		preform: append([]Field(nil), h.preform...),
	}
}

// appendAttr flattens a slog.Attr (including nested groups) into Fields with
// dotted, prefixed keys.
func appendAttr(dst *[]Field, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve() // honor slog.LogValuer (lazy/redacted)
	if a.Equal(slog.Attr{}) {
		return
	}
	key := prefix + a.Key
	switch a.Value.Kind() {
	case slog.KindGroup:
		gs := a.Value.Group()
		if a.Key == "" { // inline group
			for _, ga := range gs {
				appendAttr(dst, prefix, ga)
			}
			return
		}
		for _, ga := range gs {
			appendAttr(dst, key+".", ga)
		}
	case slog.KindString:
		*dst = append(*dst, String(key, a.Value.String()))
	case slog.KindInt64:
		*dst = append(*dst, Int(key, a.Value.Int64()))
	case slog.KindUint64:
		*dst = append(*dst, Int(key, a.Value.Uint64()))
	case slog.KindFloat64:
		*dst = append(*dst, Float(key, a.Value.Float64()))
	case slog.KindBool:
		*dst = append(*dst, Bool(key, a.Value.Bool()))
	case slog.KindDuration:
		*dst = append(*dst, Dur(key, a.Value.Duration()))
	case slog.KindTime:
		*dst = append(*dst, Time(key, a.Value.Time()))
	default:
		*dst = append(*dst, Any(key, a.Value.Any()))
	}
}

// logAt is log() with an explicit timestamp (slog bridge path).
func (l *Logger) logAt(ctx context.Context, t time.Time, level Level, msg string, fields []Field) {
	if level < l.leveler.Level() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	r := newRecord()
	r.Time = t
	r.Level = level
	r.Message = msg
	r.Ctx = ctx
	if len(l.with) > 0 {
		r.Fields = append(r.Fields, l.with...)
	}
	r.Fields = append(r.Fields, fields...)
	if err := runPipeline(l.processors, ctx, r); err != nil {
		r.release()
		return
	}
	l.transport.Dispatch(r)
	r.release()
}

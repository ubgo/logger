// Package otellogger bridges ubgo/logger into the OpenTelemetry Logs SDK: a
// Sink that emits each record through an otel log.Logger, so logs land in the
// same OTLP pipeline as traces/metrics with trace correlation from ctx.
package otellogger

import (
	"context"
	"fmt"

	logger "github.com/ubgo/logger"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// Sink implements logger.Sink by emitting OTEL LogRecords.
type Sink struct {
	lg     otellog.Logger
	minLvl logger.Level
}

// New builds a Sink from an otel LoggerProvider scope name.
func New(name string, minLevel logger.Level) *Sink {
	return &Sink{lg: global.GetLoggerProvider().Logger(name), minLvl: minLevel}
}

// NewWithLogger wires an explicit otel log.Logger (e.g. from an SDK provider).
func NewWithLogger(lg otellog.Logger, minLevel logger.Level) *Sink {
	return &Sink{lg: lg, minLvl: minLevel}
}

func severity(l logger.Level) otellog.Severity {
	// ubgo Level IS the OTEL SeverityNumber (1..24) by design.
	return otellog.Severity(l)
}

// Emit implements logger.Sink. r.Ctx carries the active span so the SDK's
// processor attaches TraceId/SpanId/TraceFlags automatically.
func (s *Sink) Emit(r *logger.Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	var rec otellog.Record
	rec.SetTimestamp(r.Time)
	rec.SetSeverity(severity(r.Level))
	rec.SetSeverityText(r.Level.String())
	rec.SetBody(otellog.StringValue(r.Message))
	for _, f := range r.Fields {
		rec.AddAttributes(toKV(f))
	}
	ctx := r.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	s.lg.Emit(ctx, rec)
	return nil
}

// Sync implements logger.Sink (the SDK processor owns flushing).
func (s *Sink) Sync() error { return nil }

// Close implements logger.Sink.
func (s *Sink) Close() error { return nil }

// toKV maps a ubgo Field to an OTEL KeyValue. ubgo's accessors are
// unexported, so non-string scalars round-trip via the String() of an
// otellog value built from the public field — kept simple and lossless for
// the common types through fmt for composites.
func toKV(f logger.Field) otellog.KeyValue {
	// Re-render via a tiny logfmt-ish path using the public API surface.
	// For composites we fall back to fmt; scalars stay typed.
	switch v := f.Value().(type) {
	case string:
		return otellog.String(f.Key, v)
	case bool:
		return otellog.Bool(f.Key, v)
	case int64:
		return otellog.Int64(f.Key, v)
	case float64:
		return otellog.Float64(f.Key, v)
	default:
		return otellog.String(f.Key, fmt.Sprint(v))
	}
}

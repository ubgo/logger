package otellogger

import (
	"context"
	"errors"
	"testing"

	logger "github.com/ubgo/logger"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/embedded"
	"go.opentelemetry.io/otel/trace"
)

// recorder is a minimal in-memory otellog.Logger for assertions.
type recorder struct {
	embedded.Logger
	recs []otellog.Record
}

func (r *recorder) Emit(_ context.Context, rec otellog.Record) { r.recs = append(r.recs, rec) }
func (r *recorder) Enabled(context.Context, otellog.EnabledParameters) bool {
	return true
}

func TestOtelSinkEmitsAllLevels(t *testing.T) {
	rec := &recorder{}
	s := NewWithLogger(rec, logger.LevelTrace)
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(s)),
		logger.WithLevel(logger.LevelTrace),
	)
	l.Trace("t")
	l.Debug("d")
	l.Info("i", logger.String("k", "v"), logger.Int("n", 3),
		logger.Bool("b", true), logger.Float("f", 1.5))
	l.Warn("w")
	l.Error("e", logger.Err(errors.New("boom")))
	l.Event("evt.name", logger.String("a", "b"))

	if len(rec.recs) != 6 {
		t.Fatalf("got %d records, want 6", len(rec.recs))
	}
	// severity of the Error record == OTEL SeverityNumber for ubgo Error (17)
	if got := rec.recs[4].Severity(); got != otellog.Severity(logger.LevelError) {
		t.Fatalf("error severity = %v", got)
	}
	if rec.recs[2].Body().AsString() != "i" {
		t.Fatalf("body = %q", rec.recs[2].Body().AsString())
	}
	if rec.recs[2].AttributesLen() != 4 {
		t.Fatalf("attrs = %d, want 4", rec.recs[2].AttributesLen())
	}
}

func TestOtelSinkMinLevelFilter(t *testing.T) {
	rec := &recorder{}
	s := NewWithLogger(rec, logger.LevelError)
	if err := s.Emit(&logger.Record{Level: logger.LevelInfo, Message: "drop"}); err != nil {
		t.Fatal(err)
	}
	if len(rec.recs) != 0 {
		t.Fatal("below-minLevel record should be filtered")
	}
	_ = s.Emit(&logger.Record{Level: logger.LevelError, Message: "keep"})
	if len(rec.recs) != 1 {
		t.Fatal("error record should pass")
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOtelSinkNilCtxRecord(t *testing.T) {
	rec := &recorder{}
	s := NewWithLogger(rec, logger.LevelTrace)
	// Record with nil Ctx exercises the context.Background() fallback.
	if err := s.Emit(&logger.Record{Level: logger.LevelInfo, Message: "x", Ctx: nil}); err != nil {
		t.Fatal(err)
	}
	if len(rec.recs) != 1 {
		t.Fatal("expected one record")
	}
}

func TestNewUsesGlobalProvider(t *testing.T) {
	// global provider is a no-op by default; just exercise the constructor.
	s := New("svc", logger.LevelInfo)
	if s == nil {
		t.Fatal("New returned nil")
	}
	_ = s.Emit(&logger.Record{Level: logger.LevelInfo, Message: "noop"})
}

func TestTraceExtractor(t *testing.T) {
	ex := TraceExtractor()
	// no span in ctx → not ok
	if _, _, ok := ex(context.Background()); ok {
		t.Fatal("expected no trace without a span")
	}
	// valid span context → ids returned
	tid := trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	sid := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	gtid, gsid, ok := ex(ctx)
	if !ok || gtid != tid.String() || gsid != sid.String() {
		t.Fatalf("extractor=%q/%q ok=%v", gtid, gsid, ok)
	}

	// wire it through the core EnrichProcessor end to end
	var buf strBuf
	core := logger.New(
		logger.WithProcessors(logger.NewEnrichProcessor(ex)),
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	core.InfoContext(ctx, "correlated")
	if !contains(buf.String(), tid.String()) {
		t.Fatalf("trace_id not enriched: %s", buf.String())
	}
}

type strBuf struct{ b []byte }

func (s *strBuf) Write(p []byte) (int, error) { s.b = append(s.b, p...); return len(p), nil }
func (s *strBuf) String() string              { return string(s.b) }

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

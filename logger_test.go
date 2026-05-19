package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedTime makes encoder output deterministic for golden assertions.
func fixedTime(t *testing.T) {
	t.Helper()
	orig := timeNow
	timeNow = func() time.Time { return time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { timeNow = orig })
}

func newBufLogger(opts ...Option) (*Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	sink := NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)
	base := []Option{WithSink(sink), WithLevel(LevelInfo)}
	return New(append(base, opts...)...), &buf
}

func TestJSONLevelsAndFields(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	l.Info("hello", String("user", "ada"), Int("n", 42), Bool("ok", true))
	l.Debug("filtered") // below Info, must not appear

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if m["msg"] != "hello" || m["level"] != "info" || m["user"] != "ada" {
		t.Fatalf("unexpected record: %v", m)
	}
	if m["n"].(float64) != 42 || m["ok"] != true {
		t.Fatalf("typed fields wrong: %v", m)
	}
	if strings.Contains(buf.String(), "filtered") {
		t.Fatal("level filter failed: debug line emitted at Info")
	}
}

func TestWithInheritsFields(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	child := l.With(String("req", "r1"))
	child.Info("a")
	l.Info("b") // parent must NOT have req
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if !strings.Contains(lines[0], `"req":"r1"`) {
		t.Fatalf("child missing inherited field: %s", lines[0])
	}
	if strings.Contains(lines[1], "req") {
		t.Fatalf("parent leaked child field: %s", lines[1])
	}
}

func TestSamplerNeverDropsErrors(t *testing.T) {
	fixedTime(t)
	sp := NewSampleProcessor(0, 1_000_000) // aggressively drop info
	l, buf := newBufLogger(WithProcessors(sp), WithLevel(LevelTrace))
	for i := 0; i < 50; i++ {
		l.Info("noise")
	}
	l.Error("boom")
	if !strings.Contains(buf.String(), "boom") {
		t.Fatal("sampler dropped an ERROR — must never happen")
	}
	if sp.Dropped() == 0 {
		t.Fatal("expected sampler to report drops")
	}
}

func TestRedactProcessor(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithProcessors(NewRedactProcessor("password")))
	l.Info("login", String("user", "ada"), String("password", "hunter2"))
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("secret leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "[REDACTED]") {
		t.Fatalf("redaction marker missing: %s", buf.String())
	}
}

func TestSlogHandlerGroupsAndAttrs(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	core := New(WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)), WithLevel(LevelInfo))
	sl := core.NewSlog()
	sl.With(slog.String("svc", "api")).
		WithGroup("http").
		Info("req", slog.Int("status", 200), slog.Group("client", slog.String("ip", "1.2.3.4")))

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if m["svc"] != "api" {
		t.Fatalf("WithAttrs not inherited: %v", m)
	}
	if m["http.status"].(float64) != 200 {
		t.Fatalf("group prefix wrong: %v", m)
	}
	if m["http.client.ip"] != "1.2.3.4" {
		t.Fatalf("nested group prefix wrong: %v", m)
	}
}

func TestChannelTransportConcurrentNoLoss(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	sink := NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)
	tr := NewChannelTransport(sink, 256, Block) // Block = lossless
	l := New(WithTransport(tr), WithLevel(LevelTrace))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				l.Info("x")
			}
		}()
	}
	wg.Wait()
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := strings.Count(buf.String(), `"msg":"x"`); got != 800 {
		t.Fatalf("Block transport lost records: got %d want 800", got)
	}
}

func TestRingTransportDropOldestCounts(t *testing.T) {
	var buf bytes.Buffer
	sink := NewWriterSink(blockingWriter{&buf}, NewJSONEncoder(), LevelTrace)
	tr := NewRingTransport(sink, 4, DropOldest)
	l := New(WithTransport(tr), WithLevel(LevelTrace))
	for i := 0; i < 1000; i++ {
		l.Info("flood")
	}
	_ = l.Close()
	if tr.Dropped() == 0 {
		t.Fatal("expected DropOldest to report drops under flood")
	}
}

type blockingWriter struct{ w *bytes.Buffer }

func (b blockingWriter) Write(p []byte) (int, error) {
	time.Sleep(time.Microsecond) // force the ring to actually fill
	return b.w.Write(p)
}

func TestEnabledGuard(t *testing.T) {
	l, _ := newBufLogger()
	if l.Enabled(LevelDebug) {
		t.Fatal("Debug must be disabled at Info level")
	}
	if !l.Enabled(LevelError) {
		t.Fatal("Error must be enabled at Info level")
	}
}

func TestContextPlumbs(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	type k struct{}
	ctx := context.WithValue(context.Background(), k{}, "v")
	l.InfoContext(ctx, "ctx-line")
	if !strings.Contains(buf.String(), "ctx-line") {
		t.Fatal("context log not emitted")
	}
}

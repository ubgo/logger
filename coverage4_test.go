package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
)

func TestEventAtAndLogAtBelowLevelAndNilCtx(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelError)) // Info/Warn gated out

	// eventAt: below-level early return + nil-ctx branch
	l.EventAt(nil, LevelInfo, "gated.event")
	if buf.Len() != 0 {
		t.Fatalf("gated event leaked: %s", buf.String())
	}
	l.EventAt(nil, LevelError, "kept.event") // nil ctx → Background
	if !strings.Contains(buf.String(), "kept.event") {
		t.Fatalf("event nil-ctx path: %s", buf.String())
	}

	// logAt (slog bridge path) below-level early return
	buf.Reset()
	sl := l.NewSlog()
	sl.Info("gated slog")           // below Error → dropped in logAt
	sl.Error("kept slog via logAt") // emitted
	if strings.Contains(buf.String(), "gated slog") {
		t.Fatalf("logAt did not gate below-level: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "kept slog via logAt") {
		t.Fatalf("logAt kept path: %s", buf.String())
	}
}

func TestLogAtNilCtxBranch(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	core := New(WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelTrace))
	h := core.Handler()
	// Drive Handle directly with a zero-time record and a nil-ish context.
	var rec slog.Record // zero Time → exercises the zero-time + logAt path
	rec.Level = slog.LevelInfo
	rec.Message = "direct handle"
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if !strings.Contains(buf.String(), "direct handle") {
		t.Fatalf("logAt via Handle: %s", buf.String())
	}
}

func TestIsTTYBranches(t *testing.T) {
	// non-*os.File writer → false
	if isTTY(&bytes.Buffer{}) {
		t.Fatal("bytes.Buffer is not a TTY")
	}
	// NO_COLOR set → false even for a *os.File
	t.Setenv("NO_COLOR", "1")
	if isTTY(os.Stdout) {
		t.Fatal("NO_COLOR must disable color")
	}
	// a regular file is a *os.File but not a char device → false
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	os.Unsetenv("NO_COLOR")
	if isTTY(f) {
		t.Fatal("a regular file is not a TTY")
	}
	// NewConsoleSink uses isTTY internally (non-file → no color)
	s := NewConsoleSink(&bytes.Buffer{}, LevelInfo)
	if s == nil {
		t.Fatal("NewConsoleSink returned nil")
	}
}

func TestChannelTransportBlockAndDropOldest(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	// Block: lossless
	bt := NewChannelTransport(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace), 4, Block)
	lb := New(WithTransport(bt), WithLevel(LevelTrace))
	for i := 0; i < 50; i++ {
		lb.Info("blk")
	}
	_ = lb.Close()
	if strings.Count(buf.String(), `"msg":"blk"`) != 50 {
		t.Fatalf("Block channel lost records")
	}
	// DropOldest path exercised under a slow sink
	do := NewChannelTransport(NewWriterSink(slowWriter2{}, NewJSONEncoder(), LevelTrace), 2, DropOldest)
	ld := New(WithTransport(do), WithLevel(LevelTrace))
	for i := 0; i < 300; i++ {
		ld.Info("old")
	}
	_ = ld.Close()
	// not asserting an exact drop count (timing), just that the branch ran
	// and Close drained without deadlock.
}

func TestRingTransportBlockLossless(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	rt := NewRingTransport(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace), 4, Block)
	l := New(WithTransport(rt), WithLevel(LevelTrace))
	for i := 0; i < 60; i++ {
		l.Info("ring-blk")
	}
	_ = l.Close()
	if strings.Count(buf.String(), `"msg":"ring-blk"`) != 60 {
		t.Fatalf("Block ring lost records")
	}
}

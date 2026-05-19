package logger

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Allocations/op is the load-bearing number. These run the typed hot path,
// the slog-bridge path (the honest "through-bridge" cost), and stdlib slog
// for reference — all writing to io.Discard so we measure the logger, not I/O.

func benchLogger() *Logger {
	return New(
		WithSink(NewWriterSink(io.Discard, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelInfo),
	)
}

func BenchmarkTypedHotPath(b *testing.B) {
	l := benchLogger()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("request handled",
			String("method", "GET"),
			String("path", "/v1/orders"),
			Int("status", 200),
			Int("bytes", 4096),
			Bool("cached", true),
		)
	}
}

func BenchmarkDisabledLevel(b *testing.B) {
	l := benchLogger()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Debug("should be cheap", String("k", "v")) // below Info
	}
}

func BenchmarkThroughSlogBridge(b *testing.B) {
	l := slog.New(benchLogger().Handler())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LogAttrs(context.Background(), slog.LevelInfo, "request handled",
			slog.String("method", "GET"),
			slog.String("path", "/v1/orders"),
			slog.Int("status", 200),
			slog.Int("bytes", 4096),
			slog.Bool("cached", true),
		)
	}
}

func BenchmarkStdlibSlogJSON(b *testing.B) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.LogAttrs(context.Background(), slog.LevelInfo, "request handled",
			slog.String("method", "GET"),
			slog.String("path", "/v1/orders"),
			slog.Int("status", 200),
			slog.Int("bytes", 4096),
			slog.Bool("cached", true),
		)
	}
}

package logger_test

import (
	"context"
	"os"

	logger "github.com/ubgo/logger"
)

// Basic structured logging with typed, zero-allocation fields.
func Example() {
	log := logger.New(
		logger.WithLevel(logger.LevelInfo),
		logger.WithSink(logger.NewWriterSink(os.Stdout, logger.NewJSONEncoder(), logger.LevelInfo)),
	)
	defer log.Close()

	log.Info("server started",
		logger.String("addr", ":8080"),
		logger.Int("workers", 8),
	)
}

// A child logger inherits bound fields; the parent is unaffected.
func ExampleLogger_With() {
	log := logger.New()
	reqLog := log.With(logger.String("request_id", "abc123"))
	reqLog.Info("handling")  // includes request_id
	log.Info("global event") // does not
}

// FingersCrossed: a successful request emits nothing below the activation
// level; the first error flushes the whole buffered debug trail.
func ExampleFingersCrossed() {
	fc := logger.NewFingersCrossed(
		logger.NewWriterSink(os.Stdout, logger.NewJSONEncoder(), logger.LevelTrace),
	)
	log := logger.New(logger.WithTransport(logger.NewSyncTransport(fc)), logger.WithLevel(logger.LevelTrace))

	ctx := logger.FCScope(context.Background())
	log.DebugContext(ctx, "step 1") // buffered
	log.DebugContext(ctx, "step 2") // buffered
	log.ErrorContext(ctx, "failed") // flushes step 1 + step 2 + this
}

// Compiled path-DSL redaction masks secrets before any sink sees them.
func ExampleNewPathRedactor() {
	pr := logger.NewPathRedactor(logger.Mask, "[SECRET]",
		"*.password", "req.headers.authorization")
	log := logger.New(
		logger.WithProcessors(pr),
		logger.WithSink(logger.NewWriterSink(os.Stdout, logger.NewJSONEncoder(), logger.LevelInfo)),
	)
	log.Info("login", logger.String("user.password", "hunter2")) // → [SECRET]
}

// Spans give scoped context + a causal tree from a flat log stream.
func ExampleLogger_StartSpan() {
	log := logger.New()
	ctx, span := log.StartSpan(context.Background(), "handle_order",
		logger.String("order_id", "o-42"))
	defer span.End() // emits span.end with duration + ok

	log.InfoContext(ctx, "charging card") // inherits span_id/order_id
}

// Message templates keep a stable key, render text, and emit structured
// fields — all from one call.
func ExampleLogger_Infot() {
	log := logger.New()
	log.Infot("processed {count} files for {user}", 12, "ada")
	// msg="processed 12 files for ada", msg_template kept, count/user structured
}

// The slog bridge: the whole slog ecosystem composes on top.
func ExampleLogger_NewSlog() {
	log := logger.New()
	sl := log.NewSlog()
	sl.Info("via slog", "key", "value")
}

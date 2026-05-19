package logger

import (
	"context"
	"runtime"
)

// Recover logs a panic (with stack) at FATAL and re-panics, so crashes are
// never invisible while the original crash semantics are preserved. Use as
// the first deferred call:
//
//	defer log.Recover(ctx)
func (l *Logger) Recover(ctx context.Context) {
	if r := recover(); r != nil {
		l.logPanic(ctx, r)
		panic(r) // preserve original crash behavior
	}
}

// RecoverAndContinue logs a panic at ERROR and swallows it — for worker loops
// / request handlers that must not take the process down. MUST be used as a
// direct deferred call (Go's recover() only works when the deferred function
// itself calls it):
//
//	defer log.RecoverAndContinue(ctx)
func (l *Logger) RecoverAndContinue(ctx context.Context) {
	if r := recover(); r != nil {
		l.log(ctxOr(ctx), LevelError, "recovered panic",
			[]Field{Any("panic", r), String("stack", stack())})
	}
}

// Go runs fn in a goroutine whose panics are logged (ERROR) instead of
// crashing the process — a safe-goroutine primitive.
func (l *Logger) Go(ctx context.Context, fn func()) {
	go func() {
		defer l.RecoverAndContinue(ctx)
		fn()
	}()
}

func (l *Logger) logPanic(ctx context.Context, r any) {
	l.log(ctxOr(ctx), LevelFatal, "panic",
		[]Field{Any("panic", r), String("stack", stack())})
	_ = l.Sync()
}

func ctxOr(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func stack() string {
	buf := make([]byte, 8<<10)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

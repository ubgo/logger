package logger

import (
	"context"
	"log"
	"strings"
)

// stdWriter turns each stdlib log.Logger line into one structured record at a
// fixed level. The migration lever: point legacy `log` output at us with one
// line, no call-site edits.
type stdWriter struct {
	l     *Logger
	level Level
}

func (w stdWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	w.l.log(context.Background(), w.level, msg, nil)
	return len(p), nil
}

// StdLogWriter returns an io.Writer suitable for log.SetOutput, so existing
// `log.Print*` calls flow through this logger at the given level.
func (l *Logger) StdLogWriter(level Level) *stdWriter {
	return &stdWriter{l: l, level: level}
}

// StdLogger returns a *log.Logger that writes through this logger — a drop-in
// for code that takes a *log.Logger.
func (l *Logger) StdLogger(level Level) *log.Logger {
	return log.New(l.StdLogWriter(level), "", 0)
}

// RedirectStdLog routes the global stdlib logger (log.Default) through this
// logger and returns a function restoring the previous output (handy in tests
// — the global-logger boundary, made explicit and reversible).
func RedirectStdLog(l *Logger, level Level) func() {
	prev := log.Writer()
	pf := log.Flags()
	pp := log.Prefix()
	log.SetOutput(l.StdLogWriter(level))
	log.SetFlags(0)
	log.SetPrefix("")
	return func() {
		log.SetOutput(prev)
		log.SetFlags(pf)
		log.SetPrefix(pp)
	}
}

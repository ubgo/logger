package logger

import "os"

// Development returns a logger tuned for local work: pretty, colored,
// TTY-aware console on stderr, Debug level, caller location on. One line, no
// boilerplate.
func Development() *Logger {
	return New(
		WithLevel(LevelDebug),
		WithSink(NewConsoleSink(os.Stderr, LevelDebug)),
		WithCaller(0),
	)
}

// Production returns a logger tuned for services: JSON to stderr, Info level,
// async (bounded channel, drop-newest under pressure so a log storm can't
// stall request handling), light sampling that never drops errors. Call
// Close() on shutdown to drain.
func Production() *Logger {
	sink := NewWriterSink(os.Stderr, NewJSONEncoder(), LevelInfo)
	return New(
		WithLevel(LevelInfo),
		WithProcessors(NewSampleProcessor(100, 100)),
		WithTransport(NewChannelTransport(sink, 4096, DropNewest)),
	)
}

// Test returns a logger writing pretty output to the given writer at Trace
// with no async/sampling — deterministic for examples and manual debugging.
// For assertions use the logtest package instead.
func Test(w interface{ Write([]byte) (int, error) }) *Logger {
	return New(
		WithLevel(LevelTrace),
		WithSink(NewWriterSink(w, NewConsoleEncoder(), LevelTrace)),
	)
}

package logger

import "time"

// ConsoleEncoder produces human-readable, optionally colored output for dev
// terminals. Color is opt-in by the sink (TTY-aware), never forced here.
type ConsoleEncoder struct {
	Color      bool
	TimeFormat string // default "15:04:05.000"
}

// NewConsoleEncoder returns a ConsoleEncoder with sensible defaults.
func NewConsoleEncoder() ConsoleEncoder {
	return ConsoleEncoder{TimeFormat: "15:04:05.000"}
}

// Name implements Encoder.
func (ConsoleEncoder) Name() string { return "console" }

// ANSI colors by severity band.
func levelColor(l Level) string {
	switch {
	case l < LevelDebug:
		return "\x1b[90m" // trace: grey
	case l < LevelInfo:
		return "\x1b[36m" // debug: cyan
	case l < LevelWarn:
		return "\x1b[32m" // info: green
	case l < LevelError:
		return "\x1b[33m" // warn: yellow
	default:
		return "\x1b[31m" // error/fatal: red
	}
}

// Encode implements Encoder.
func (e ConsoleEncoder) Encode(buf *buffer, r *Record) {
	tf := e.TimeFormat
	if tf == "" {
		tf = "15:04:05.000"
	}
	buf.b = r.Time.AppendFormat(buf.b, tf)
	buf.writeByte(' ')

	lvl := r.Level.lower()
	pad := ""
	if n := 5 - len(lvl); n > 0 {
		pad = "     "[:n] // slice of a constant — no alloc
	}
	if e.Color {
		buf.writeString(levelColor(r.Level))
		buf.writeString(lvl)
		buf.writeString(pad)
		buf.writeString("\x1b[0m")
	} else {
		buf.writeString(lvl)
		buf.writeString(pad)
	}

	buf.writeByte(' ')
	buf.writeString(r.Message)

	if src, ok := r.source(); ok {
		buf.writeByte(' ')
		if e.Color {
			buf.writeString("\x1b[2m")
		}
		buf.writeByte('<')
		buf.writeString(src.File)
		buf.writeByte(':')
		buf.writeInt(int64(src.Line))
		buf.writeByte('>')
		if e.Color {
			buf.writeString("\x1b[0m")
		}
	}

	for i := range r.Fields {
		f := &r.Fields[i]
		buf.writeByte(' ')
		if e.Color {
			buf.writeString("\x1b[2m")
		}
		buf.writeString(f.Key)
		buf.writeByte('=')
		if e.Color {
			buf.writeString("\x1b[0m")
		}
		appendFieldValue(buf, *f, false)
	}
	buf.writeByte('\n')
}

// timeNow is overridable in tests for deterministic golden output.
var timeNow = time.Now

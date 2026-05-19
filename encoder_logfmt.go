package logger

import "strings"

// LogfmtEncoder emits `key=value` pairs (Heroku/Go-kit style) — the
// human-skimmable yet machine-parseable middle ground. Values needing
// quoting (spaces, quotes, =) are double-quoted.
type LogfmtEncoder struct{}

// NewLogfmtEncoder returns a LogfmtEncoder.
func NewLogfmtEncoder() LogfmtEncoder { return LogfmtEncoder{} }

// Name implements Encoder.
func (LogfmtEncoder) Name() string { return "logfmt" }

func logfmtNeedsQuote(s string) bool {
	if s == "" {
		return true
	}
	return strings.ContainsAny(s, " =\"\n\t")
}

func (b *buffer) writeLogfmtValue(s string) {
	if logfmtNeedsQuote(s) {
		b.writeJSONString(s) // JSON quoting is a valid logfmt quoting
		return
	}
	b.writeString(s)
}

// Encode implements Encoder.
func (LogfmtEncoder) Encode(buf *buffer, r *Record) {
	if !r.Time.IsZero() {
		buf.writeString("time=")
		buf.writeByte('"')
		buf.writeTimeRFC3339(r.Time)
		buf.writeByte('"')
		buf.writeByte(' ')
	}
	buf.writeString("level=")
	buf.writeString(r.Level.lower())
	buf.writeString(" msg=")
	buf.writeLogfmtValue(r.Message)

	if src, ok := r.source(); ok {
		buf.writeString(" caller=")
		buf.writeByte('"')
		buf.writeString(src.File)
		buf.writeByte(':')
		buf.writeInt(int64(src.Line))
		buf.writeByte('"')
	}

	for i := range r.Fields {
		f := &r.Fields[i]
		buf.writeByte(' ')
		buf.writeString(f.Key)
		buf.writeByte('=')
		// reuse the text path; wrap strings/anys that need quoting
		switch f.knd {
		case kindString:
			buf.writeLogfmtValue(f.string())
		case kindError:
			if e := f.errVal(); e != nil {
				buf.writeLogfmtValue(e.Error())
			} else {
				buf.writeString("null")
			}
		default:
			appendFieldValue(buf, *f, false)
		}
	}
	buf.writeByte('\n')
}

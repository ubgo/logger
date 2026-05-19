package logger

import (
	"context"
	"strings"
)

// Message templates (Serilog's signature idea, unported to Go until now).
// Infot("processed {count} files for {user}", n, user) emits:
//   - msg            : "processed 12 files for ada"   (rendered, human)
//   - msg_template   : "processed {count} files for {user}"  (STABLE key for
//                       grouping/alerting — survives changing values)
//   - count=12, user="ada"  (structured fields, named by the holes)
//
// One call gives readable text AND structured data AND a stable event
// identity — no Printf-vs-KV tradeoff. {{ and }} are literal braces.
//
// This is the convenience tier (it allocates a rendered string); use the
// typed API on zero-alloc hot paths.

func parseTemplate(tmpl string, args []any) (msg string, fields []Field) {
	var sb strings.Builder
	sb.Grow(len(tmpl) + 16)
	fields = make([]Field, 0, len(args)+1)
	ai := 0
	for i := 0; i < len(tmpl); i++ {
		c := tmpl[i]
		if c == '{' {
			if i+1 < len(tmpl) && tmpl[i+1] == '{' { // escaped {{
				sb.WriteByte('{')
				i++
				continue
			}
			end := strings.IndexByte(tmpl[i:], '}')
			if end < 0 { // unterminated — emit literally
				sb.WriteString(tmpl[i:])
				break
			}
			name := tmpl[i+1 : i+end]
			var val any
			if ai < len(args) {
				val = args[ai]
				ai++
			}
			fields = append(fields, Any(name, val))
			sb.WriteString(sprintAny(val))
			i += end
			continue
		}
		if c == '}' && i+1 < len(tmpl) && tmpl[i+1] == '}' { // escaped }}
			sb.WriteByte('}')
			i++
			continue
		}
		sb.WriteByte(c)
	}
	fields = append(fields, String("msg_template", tmpl))
	return sb.String(), fields
}

func (l *Logger) logt(ctx context.Context, level Level, tmpl string, args []any) {
	if !l.Enabled(level) {
		return
	}
	msg, fields := parseTemplate(tmpl, args)
	l.log(ctx, level, msg, fields)
}

// Templated level methods. The trailing args fill {holes} left to right.

func (l *Logger) Debugt(tmpl string, args ...any) {
	l.logt(context.Background(), LevelDebug, tmpl, args)
}
func (l *Logger) Infot(tmpl string, args ...any) {
	l.logt(context.Background(), LevelInfo, tmpl, args)
}
func (l *Logger) Warnt(tmpl string, args ...any) {
	l.logt(context.Background(), LevelWarn, tmpl, args)
}
func (l *Logger) Errort(tmpl string, args ...any) {
	l.logt(context.Background(), LevelError, tmpl, args)
}

// Logt emits a templated record at an explicit level with context.
func (l *Logger) Logt(ctx context.Context, level Level, tmpl string, args ...any) {
	l.logt(ctx, level, tmpl, args)
}

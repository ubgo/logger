package logger

import (
	"strconv"
	"sync"
	"time"
)

// buffer is a pooled byte builder. Encoders write into it; sinks flush it.
type buffer struct{ b []byte }

var bufPool = sync.Pool{New: func() any { return &buffer{b: make([]byte, 0, 512)} }}

func getBuffer() *buffer { return bufPool.Get().(*buffer) }

func putBuffer(b *buffer) {
	// Drop oversized buffers so a single huge log line doesn't pin memory.
	if cap(b.b) <= 64<<10 {
		b.b = b.b[:0]
		bufPool.Put(b)
	}
}

func (b *buffer) writeString(s string) { b.b = append(b.b, s...) }
func (b *buffer) writeByte(c byte)     { b.b = append(b.b, c) }
func (b *buffer) writeInt(i int64)     { b.b = strconv.AppendInt(b.b, i, 10) }
func (b *buffer) writeUint(u uint64)   { b.b = strconv.AppendUint(b.b, u, 10) }
func (b *buffer) writeFloat(f float64) { b.b = strconv.AppendFloat(b.b, f, 'g', -1, 64) }
func (b *buffer) writeBool(v bool)     { b.b = strconv.AppendBool(b.b, v) }
func (b *buffer) writeTimeRFC3339(t time.Time) {
	b.b = t.AppendFormat(b.b, time.RFC3339Nano)
}

// writeJSONString appends s as a quoted, escaped JSON string.
func (b *buffer) writeJSONString(s string) {
	b.b = append(b.b, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			continue
		}
		b.b = append(b.b, s[start:i]...)
		switch c {
		case '"':
			b.b = append(b.b, '\\', '"')
		case '\\':
			b.b = append(b.b, '\\', '\\')
		case '\n':
			b.b = append(b.b, '\\', 'n')
		case '\r':
			b.b = append(b.b, '\\', 'r')
		case '\t':
			b.b = append(b.b, '\\', 't')
		default:
			b.b = append(b.b, '\\', 'u', '0', '0',
				hexDigit(c>>4), hexDigit(c&0xf))
		}
		start = i + 1
	}
	b.b = append(b.b, s[start:]...)
	b.b = append(b.b, '"')
}

func hexDigit(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + n - 10
}

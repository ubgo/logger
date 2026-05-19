package logger

import (
	"time"
)

// kind enumerates how a Field's value is stored so encoders never reflect on
// common types (the zero-alloc path). Reflection is reserved for kindAny.
type kind uint8

const (
	kindAny kind = iota
	kindString
	kindInt64
	kindUint64
	kindFloat64
	kindBool
	kindDuration
	kindTime
	kindError
)

// Field is one structured key/value pair. Scalars are stored unboxed (num/str)
// so the typed API allocates nothing; only kindAny escapes to interface{}.
type Field struct {
	Key string
	knd kind
	num uint64
	str string
	any any
}

// --- Type-safe generic constructors ----------------------------------------

// Integer/Float constraints kept local so the core module stays zero-dep.
type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

type float interface{ ~float32 | ~float64 }

// String adds a string field.
func String(key, val string) Field {
	return Field{Key: key, knd: kindString, str: val}
}

// Int adds any integer-typed value without boxing.
func Int[T integer](key string, v T) Field {
	if T(0)-1 < T(0) { // signed type: 0-1 == -1 < 0
		return Field{Key: key, knd: kindInt64, num: uint64(int64(v))}
	}
	return Field{Key: key, knd: kindUint64, num: uint64(v)}
}

// Float adds any float-typed value without boxing.
func Float[T float](key string, v T) Field {
	return Field{Key: key, knd: kindFloat64, num: float64bits(float64(v))}
}

// Bool adds a boolean field.
func Bool(key string, v bool) Field {
	var n uint64
	if v {
		n = 1
	}
	return Field{Key: key, knd: kindBool, num: n}
}

// Dur adds a time.Duration field.
func Dur(key string, d time.Duration) Field {
	return Field{Key: key, knd: kindDuration, num: uint64(d)}
}

// Time adds a time.Time field (stored as UnixNano).
func Time(key string, t time.Time) Field {
	return Field{Key: key, knd: kindTime, num: uint64(t.UnixNano())}
}

// Err adds an error under the conventional "error" key. nil is preserved so
// the encoder can emit an explicit null rather than dropping the field.
func Err(err error) Field {
	return Field{Key: "error", knd: kindError, any: err}
}

// NamedErr is Err with a caller-chosen key.
func NamedErr(key string, err error) Field {
	return Field{Key: key, knd: kindError, any: err}
}

// Any is the escape hatch: arbitrary value via reflection at encode time.
// Prefer a typed constructor on hot paths.
func Any(key string, v any) Field {
	return Field{Key: key, knd: kindAny, any: v}
}

// --- accessors used by encoders --------------------------------------------

func (f Field) string() string          { return f.str }
func (f Field) int64() int64            { return int64(f.num) }
func (f Field) uint64() uint64          { return f.num }
func (f Field) float64() float64        { return float64frombits(f.num) }
func (f Field) bool() bool              { return f.num != 0 }
func (f Field) duration() time.Duration { return time.Duration(f.num) }
func (f Field) time() time.Time         { return time.Unix(0, int64(f.num)) }
func (f Field) errVal() error {
	e, _ := f.any.(error)
	return e
}

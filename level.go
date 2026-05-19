package logger

import "strconv"

// Level is a log severity modeled directly on the OpenTelemetry Logs
// SeverityNumber (1..24, four sub-steps per band) so the level survives an
// OTEL bridge without a lossy remap. 0 means "unspecified".
//
//	TRACE 1-4 · DEBUG 5-8 · INFO 9-12 · WARN 13-16 · ERROR 17-20 · FATAL 21-24
//
// Any SeverityNumber >= 17 (ERROR) denotes an erroneous record.
type Level int

// Canonical band anchors. Custom levels are just other ints in a band, e.g.
// LevelDebug+1 for "DEBUG2".
const (
	LevelTrace Level = 1
	LevelDebug Level = 5
	LevelInfo  Level = 9
	LevelWarn  Level = 13
	LevelError Level = 17
	LevelFatal Level = 21
)

// SeverityText returns the OTEL severity band name for the level.
func (l Level) String() string {
	switch {
	case l <= 0:
		return "UNSPECIFIED"
	case l < LevelDebug:
		return "TRACE"
	case l < LevelInfo:
		return "DEBUG"
	case l < LevelWarn:
		return "INFO"
	case l < LevelError:
		return "WARN"
	case l < LevelFatal:
		return "ERROR"
	default:
		return "FATAL"
	}
}

// SeverityNumber returns the raw OTEL SeverityNumber.
func (l Level) SeverityNumber() int { return int(l) }

// MarshalText implements encoding.TextMarshaler (lowercase band name).
func (l Level) MarshalText() ([]byte, error) {
	s := l.String()
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return b, nil
}

func (l Level) appendNumeric(b []byte) []byte {
	return strconv.AppendInt(b, int64(l), 10)
}

// Leveler reports the minimum level a logger currently emits. Implementations
// may change the returned value at runtime (see LevelVar) so callers must call
// Level() per-decision, never cache it.
type Leveler interface {
	Level() Level
}

// LevelVar is a Leveler whose value can be swapped atomically at runtime —
// the basis for dynamic per-module level control without a restart.
type LevelVar struct {
	v atomicInt64
}

// NewLevelVar returns a LevelVar set to l.
func NewLevelVar(l Level) *LevelVar {
	lv := &LevelVar{}
	lv.v.Store(int64(l))
	return lv
}

// Level implements Leveler.
func (lv *LevelVar) Level() Level { return Level(lv.v.Load()) }

// Set atomically changes the minimum level.
func (lv *LevelVar) Set(l Level) { lv.v.Store(int64(l)) }

// staticLevel adapts a constant Level to the Leveler interface.
type staticLevel Level

func (s staticLevel) Level() Level { return Level(s) }

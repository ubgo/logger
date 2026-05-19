package logger

import (
	"io"
	"testing"
)

// TestZeroAllocTypedPath is the allocation-regression gate: the typed hot
// path MUST stay 0 allocs/op. If a change reintroduces an allocation (as the
// Level.MarshalText []byte did), this fails in CI before it ships.
func TestZeroAllocTypedPath(t *testing.T) {
	if raceEnabled {
		t.Skip("race detector instruments allocations; gate is meaningless under -race")
	}
	l := New(
		WithSink(NewWriterSink(io.Discard, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelInfo),
	)
	avg := testing.AllocsPerRun(2000, func() {
		l.Info("request handled",
			String("method", "GET"),
			String("path", "/v1/orders"),
			Int("status", 200),
			Int("bytes", 4096),
			Bool("cached", true),
		)
	})
	if avg != 0 {
		t.Fatalf("typed hot path allocates %.2f objects/op, want 0", avg)
	}
}

func TestZeroAllocDisabledLevel(t *testing.T) {
	if raceEnabled {
		t.Skip("race detector instruments allocations; gate is meaningless under -race")
	}
	l := New(
		WithSink(NewWriterSink(io.Discard, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelInfo),
	)
	avg := testing.AllocsPerRun(2000, func() {
		l.Debug("below level", String("k", "v")) // gated out
	})
	if avg != 0 {
		t.Fatalf("disabled-level path allocates %.2f objects/op, want 0", avg)
	}
}

package logger

import "sync/atomic"

// atomicInt64 is a thin alias kept separate so level.go reads cleanly and the
// concurrency primitive is swappable in one place.
type atomicInt64 = atomic.Int64

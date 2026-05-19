package logger

import "math"

// Float bit-cast helpers kept in one place so field.go stays focused.
func float64bits(f float64) uint64     { return math.Float64bits(f) }
func float64frombits(b uint64) float64 { return math.Float64frombits(b) }

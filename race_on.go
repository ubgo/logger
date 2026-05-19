//go:build race

package logger

// raceEnabled is true when built with -race. The race detector instruments
// allocations, so allocation-count assertions are meaningless under it.
const raceEnabled = true

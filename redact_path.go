package logger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// RedactStrategy is what happens to a matched field.
type RedactStrategy uint8

const (
	// Mask replaces the value with the censor string.
	Mask RedactStrategy = iota
	// Hash replaces it with a stable sha256 prefix (preserves correlation
	// without exposing the value).
	Hash
	// Drop removes the field entirely.
	Drop
)

// PathRedactor is the compiled declarative redaction stage (pino/LogTape
// model). Patterns are dotted paths over field keys — which already carry
// slog group prefixes ("http.headers.authorization"). Segment wildcards:
//
//   - matches exactly one segment
//     **  matches zero or more segments (must be the last segment)
//
// Examples: "password", "*.password", "req.headers.authorization",
// "user.**".
type PathRedactor struct {
	patterns [][]string
	strategy RedactStrategy
	censor   string
}

// NewPathRedactor compiles patterns once. Strategy + censor are fixed for the
// stage (compose multiple stages for mixed policies).
func NewPathRedactor(strategy RedactStrategy, censor string, patterns ...string) *PathRedactor {
	if censor == "" {
		censor = "[REDACTED]"
	}
	pr := &PathRedactor{strategy: strategy, censor: censor}
	for _, p := range patterns {
		pr.patterns = append(pr.patterns, strings.Split(p, "."))
	}
	return pr
}

func segMatch(pat, key []string) bool {
	for i := 0; i < len(pat); i++ {
		if pat[i] == "**" { // greedy tail wildcard
			return true
		}
		if i >= len(key) {
			return false
		}
		if pat[i] == "*" || pat[i] == key[i] {
			continue
		}
		return false
	}
	return len(pat) == len(key)
}

func (pr *PathRedactor) matches(key string) bool {
	ks := strings.Split(key, ".")
	for _, p := range pr.patterns {
		if segMatch(p, ks) {
			return true
		}
	}
	return false
}

// Process implements Processor.
func (pr *PathRedactor) Process(_ context.Context, r *Record) error {
	dst := r.Fields[:0]
	for _, f := range r.Fields {
		if !pr.matches(f.Key) {
			dst = append(dst, f)
			continue
		}
		switch pr.strategy {
		case Drop:
			// skip — field removed
		case Hash:
			dst = append(dst, String(f.Key, hashValue(f)))
		default: // Mask
			dst = append(dst, String(f.Key, pr.censor))
		}
	}
	r.Fields = dst
	return nil
}

func hashValue(f Field) string {
	var s string
	if f.knd == kindString {
		s = f.string()
	} else {
		s = sprintAny(f.Value())
	}
	sum := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(sum[:8])
}

package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"testing/slogtest"
)

// TestSlogConformance runs the standard library's slogtest harness — the
// gauntlet most third-party slog.Handlers fail. Our wire format is flat
// dotted keys ("g.a") by deliberate design; the results parser un-flattens
// them back into nested maps so slogtest's group expectations are satisfied
// without forcing nested JSON on the wire.
func TestSlogConformance(t *testing.T) {
	var buf bytes.Buffer
	core := New(
		WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelTrace),
	)
	h := core.Handler()

	results := func() []map[string]any {
		var out []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
			if line == "" {
				continue
			}
			var flat map[string]any
			if err := json.Unmarshal([]byte(line), &flat); err != nil {
				t.Fatalf("invalid JSON line %q: %v", line, err)
			}
			out = append(out, unflatten(flat))
		}
		return out
	}

	if err := slogtest.TestHandler(h, results); err != nil {
		t.Fatalf("slogtest conformance failed:\n%v", err)
	}
}

// unflatten turns {"a.b.c": 1} into {"a":{"b":{"c":1}}} so grouped attrs
// round-trip through slogtest.
func unflatten(flat map[string]any) map[string]any {
	root := map[string]any{}
	for k, v := range flat {
		parts := strings.Split(k, ".")
		m := root
		for i := 0; i < len(parts)-1; i++ {
			next, ok := m[parts[i]].(map[string]any)
			if !ok {
				next = map[string]any{}
				m[parts[i]] = next
			}
			m = next
		}
		m[parts[len(parts)-1]] = v
	}
	return root
}

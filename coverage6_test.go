package logger

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// itoa64 negative branch (Loki uses UnixNano so it's positive in practice;
// covered directly for completeness/correctness).
func TestItoa64(t *testing.T) {
	for in, want := range map[int64]string{
		0: "0", 42: "42", -42: "-42", 9223372036854775807: "9223372036854775807",
	} {
		if got := itoa64(in); got != want {
			t.Fatalf("itoa64(%d)=%q want %q", in, got, want)
		}
	}
}

// SampleProcessor: the "first N are kept" branch (n <= First).
func TestSamplerFirstNKept(t *testing.T) {
	fixedTime(t)
	sp := NewSampleProcessor(5, 1000) // keep first 5, then ~none
	l, buf := newBufLogger(WithProcessors(sp), WithLevel(LevelTrace))
	for i := 0; i < 3; i++ {
		l.Info("early") // all within First → kept via n<=First branch
	}
	if strings.Count(buf.String(), `"msg":"early"`) != 3 {
		t.Fatalf("first-N keep branch: %s", buf.String())
	}
}

// slog bridge + a failing processor → logAt's pipeline-error drop branch
// (and the dropped metric on that path).
func TestSlogLogAtPipelineDrop(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(
		WithProcessors(ProcessorFunc(func(_ context.Context, _ *Record) error {
			return errors.New("fail in logAt path")
		})),
		WithLevel(LevelTrace),
	)
	sl := l.NewSlog()
	sl.Info("dropped in logAt")
	if buf.Len() != 0 {
		t.Fatalf("logAt must drop on processor error: %s", buf.String())
	}
	if l.Metrics().Snapshot().Dropped == 0 {
		t.Fatal("logAt drop not counted in metrics")
	}
}

// logfmt encoder with caller enabled (the caller-render branch) + a record
// with no fields.
func TestLogfmtWithCallerNoFields(t *testing.T) {
	fixedTime(t)
	var sb strings.Builder
	l := New(
		WithSink(NewWriterSink(&sb, NewLogfmtEncoder(), LevelTrace)),
		WithLevel(LevelTrace),
		WithCaller(0),
	)
	l.Info("logfmt caller")
	out := sb.String()
	if !strings.Contains(out, "level=info") || !strings.Contains(out, "caller=") {
		t.Fatalf("logfmt caller branch: %q", out)
	}
}

// NetSink where BOTH the initial dial and the re-dial fail: exercises the
// second dial-failure return + dropped accounting.
func TestNetSinkBothDialsFail(t *testing.T) {
	fixedTime(t)
	s := NewTCPSink("127.0.0.1:1", NewJSONEncoder(), LevelInfo) // nothing listens
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	l.Info("undeliverable")
	if s.Dropped() == 0 {
		t.Fatal("failed dial must count as dropped")
	}
	_ = s.Close()
}

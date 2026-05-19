package logger

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFingersCrossedDiscardsOnSuccess(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	fc := NewFingersCrossed(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace))
	l := New(WithTransport(NewSyncTransport(fc)), WithLevel(LevelTrace))

	ctx := FCScope(context.Background())
	l.DebugContext(ctx, "step 1")
	l.DebugContext(ctx, "step 2")
	l.InfoContext(ctx, "step 3")
	// no error in scope → nothing should be emitted
	if buf.Len() != 0 {
		t.Fatalf("successful scope leaked logs: %s", buf.String())
	}
}

func TestFingersCrossedFlushesOnError(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	fc := NewFingersCrossed(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace))
	l := New(WithTransport(NewSyncTransport(fc)), WithLevel(LevelTrace))

	ctx := FCScope(context.Background())
	l.DebugContext(ctx, "loading config")
	l.DebugContext(ctx, "connecting db")
	l.ErrorContext(ctx, "db failed")

	out := buf.String()
	for _, want := range []string{"loading config", "connecting db", "db failed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("error scope missing %q in flushed trail:\n%s", want, out)
		}
	}
	// records after activation pass straight through
	l.DebugContext(ctx, "post-error detail")
	if !strings.Contains(buf.String(), "post-error detail") {
		t.Fatal("post-activation record not passed through")
	}
}

func TestPathRedactorWildcards(t *testing.T) {
	fixedTime(t)
	pr := NewPathRedactor(Mask, "[X]", "*.password", "req.headers.authorization", "ssn")
	l, buf := newBufLogger(WithProcessors(pr))
	l.Info("login",
		String("user.password", "hunter2"),
		String("req.headers.authorization", "Bearer abc"),
		String("ssn", "123-45-6789"),
		String("safe", "keep"),
	)
	out := buf.String()
	if strings.Contains(out, "hunter2") || strings.Contains(out, "Bearer abc") || strings.Contains(out, "123-45-6789") {
		t.Fatalf("secret leaked through path redactor: %s", out)
	}
	if !strings.Contains(out, `"safe":"keep"`) {
		t.Fatalf("non-matching field wrongly redacted: %s", out)
	}
}

func TestPathRedactorHashAndDrop(t *testing.T) {
	fixedTime(t)
	hash := NewPathRedactor(Hash, "", "token")
	drop := NewPathRedactor(Drop, "", "secret")
	l, buf := newBufLogger(WithProcessors(hash, drop))
	l.Info("x", String("token", "abc"), String("secret", "shh"), String("ok", "y"))
	out := buf.String()
	if strings.Contains(out, `"secret"`) {
		t.Fatalf("Drop strategy left field: %s", out)
	}
	if !strings.Contains(out, "sha256:") {
		t.Fatalf("Hash strategy missing: %s", out)
	}
	if strings.Contains(out, `"token":"abc"`) {
		t.Fatalf("Hash leaked raw value: %s", out)
	}
}

func TestEnrichBoundFieldsAndTrace(t *testing.T) {
	fixedTime(t)
	ex := func(ctx context.Context) (string, string, bool) {
		return "tid-123", "sid-456", true
	}
	l, buf := newBufLogger(WithProcessors(NewEnrichProcessor(ex)))
	ctx := ContextWith(context.Background(), String("request_id", "r-1"))
	l.InfoContext(ctx, "handled")
	out := buf.String()
	for _, want := range []string{`"request_id":"r-1"`, `"trace_id":"tid-123"`, `"span_id":"sid-456"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("enrich missing %s in: %s", want, out)
		}
	}
}

func TestLevelHTTPHandler(t *testing.T) {
	lv := NewLevelVar(LevelInfo)
	h := NewLevelHandler(lv)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/loglevel", nil))
	if !strings.Contains(rec.Body.String(), `"level":"info"`) {
		t.Fatalf("GET wrong: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/loglevel?level=debug", nil))
	if lv.Level() != LevelDebug {
		t.Fatalf("PUT did not change level: got %v", lv.Level())
	}
}

func TestDedupThrottles(t *testing.T) {
	fixedTime(t) // timeNow frozen → all within window
	l, buf := newBufLogger(WithProcessors(NewDedupProcessor(time.Minute)), WithLevel(LevelTrace))
	for i := 0; i < 10; i++ {
		l.Warn("disk slow")
	}
	l.Warn("different")
	got := strings.Count(buf.String(), `"msg":"disk slow"`)
	if got != 1 {
		t.Fatalf("dedup let %d identical lines through, want 1\n%s", got, buf.String())
	}
	if !strings.Contains(buf.String(), `"msg":"different"`) {
		t.Fatal("dedup wrongly suppressed a distinct message")
	}
}

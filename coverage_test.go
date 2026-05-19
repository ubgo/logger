package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// fakeStringer exercises the fmt.Stringer / Any reflection paths.
type fakeStringer struct{}

func (fakeStringer) String() string { return "I-stringify" }

func TestAllFieldKindsEncode(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.Info("kinds",
		String("s", "x"),
		Int("i", -7),
		Int("u", uint32(7)),
		Float("f", 3.5),
		Bool("b", true),
		Dur("d", 2*time.Second),
		Time("t", time.Unix(0, 0).UTC()),
		Err(errors.New("boom")),
		NamedErr("nerr", nil),
		Any("any_str", "hi"),
		Any("any_int", 9),
		Any("any_struct", struct{ A int }{1}),
		Any("any_stringer", fakeStringer{}),
		Any("any_nil", nil),
	)
	s := buf.String()
	for _, want := range []string{`"s":"x"`, `"i":-7`, `"u":7`, `"f":3.5`,
		`"b":true`, `"d":"2s"`, `"boom"`, `"nerr":null`, `"any_struct":{"A":1}`,
		`"any_stringer":"I-stringify"`, `"any_nil":null`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}

func TestFieldValueAllKinds(t *testing.T) {
	cases := []struct {
		f    Field
		want any
	}{
		{String("k", "v"), "v"},
		{Int("k", 5), int64(5)},
		{Int("k", uint64(5)), uint64(5)},
		{Float("k", 1.5), 1.5},
		{Bool("k", true), true},
		{Dur("k", time.Second), time.Second},
	}
	for _, c := range cases {
		if got := c.f.Value(); got != c.want {
			t.Fatalf("Value()=%v(%T) want %v(%T)", got, got, c.want, c.want)
		}
	}
	if _, ok := Time("k", time.Now()).Value().(time.Time); !ok {
		t.Fatal("Time Value not time.Time")
	}
	if Err(errors.New("e")).Value().(error).Error() != "e" {
		t.Fatal("Err Value not error")
	}
	if Any("k", 42).Value() != 42 {
		t.Fatal("Any Value mismatch")
	}
}

func TestLevelHelpers(t *testing.T) {
	if LevelInfo.SeverityNumber() != 9 {
		t.Fatal("SeverityNumber")
	}
	if Level(0).String() != "UNSPECIFIED" || LevelTrace.String() != "TRACE" ||
		LevelFatal.String() != "FATAL" {
		t.Fatal("String bands")
	}
	b, _ := LevelError.MarshalText()
	if string(b) != "error" {
		t.Fatalf("MarshalText=%s", b)
	}
	if Level(0).lower() != "unspecified" {
		t.Fatal("lower unspecified")
	}
	var bb []byte
	bb = LevelWarn.appendNumeric(bb)
	if string(bb) != "13" {
		t.Fatalf("appendNumeric=%s", bb)
	}
	lv := NewLevelVar(LevelInfo)
	lv.Set(LevelDebug)
	if lv.Level() != LevelDebug {
		t.Fatal("LevelVar.Set")
	}
}

func TestEncodersConsoleAndLogfmt(t *testing.T) {
	fixedTime(t)
	r := newRecord()
	r.Time = timeNow()
	r.Level = LevelWarn
	r.Message = "msg here"
	r.Fields = []Field{String("k", "v"), Int("n", 1), String("sp", "has space")}

	ce := NewConsoleEncoder()
	ce.Color = true
	if ce.Name() != "console" {
		t.Fatal("console Name")
	}
	cb := getBuffer()
	ce.Encode(cb, r)
	if !strings.Contains(string(cb.b), "msg here") || !strings.Contains(string(cb.b), "\x1b[") {
		t.Fatalf("console color encode: %q", cb.b)
	}
	putBuffer(cb)

	le := NewLogfmtEncoder()
	if le.Name() != "logfmt" {
		t.Fatal("logfmt Name")
	}
	lb := getBuffer()
	le.Encode(lb, r)
	out := string(lb.b)
	if !strings.Contains(out, "level=warn") || !strings.Contains(out, `sp="has space"`) {
		t.Fatalf("logfmt encode: %q", out)
	}
	putBuffer(lb)

	if NewJSONEncoder().Name() != "json" {
		t.Fatal("json Name")
	}
	// numeric level + caller render
	je := NewJSONEncoder()
	je.NumericLevel = true
	r.PC = pcOfThisFunc()
	jb := getBuffer()
	je.Encode(jb, r)
	if !strings.Contains(string(jb.b), `"level":13`) {
		t.Fatalf("numeric level: %q", jb.b)
	}
	putBuffer(jb)
	r.release()
}

func pcOfThisFunc() uint64 {
	var p [1]uintptr
	// 2 = runtime.Callers + this helper's caller
	n := runtime.Callers(2, p[:])
	if n == 0 {
		return 0
	}
	return uint64(p[0])
}

func TestAllLevelMethods(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	ctx := context.Background()
	l.Trace("a")
	l.Debug("b")
	l.Warn("c")
	l.TraceContext(ctx, "d")
	l.DebugContext(ctx, "e")
	l.WarnContext(ctx, "f")
	l.Log(ctx, LevelInfo, "g")
	l.EventAt(ctx, LevelWarn, "evt.x", String("k", "v"))
	for _, w := range []string{`"a"`, `"b"`, `"c"`, `"d"`, `"e"`, `"f"`, `"g"`, `"evt.x"`} {
		if !strings.Contains(buf.String(), w) {
			t.Fatalf("missing %s", w)
		}
	}
}

func TestProcessorFuncAndAddField(t *testing.T) {
	fixedTime(t)
	p := ProcessorFunc(func(_ context.Context, r *Record) error {
		r.AddField(String("injected", "yes"))
		return nil
	})
	l, buf := newBufLogger(WithProcessors(p))
	l.Info("x")
	if !strings.Contains(buf.String(), `"injected":"yes"`) {
		t.Fatalf("ProcessorFunc/AddField: %s", buf.String())
	}
}

func TestPipelineErrorIsDropAndCounted(t *testing.T) {
	fixedTime(t)
	boom := ProcessorFunc(func(_ context.Context, _ *Record) error {
		return errors.New("processor failed")
	})
	l, buf := newBufLogger(WithProcessors(boom), WithLevel(LevelTrace))
	l.Info("never")
	if buf.Len() != 0 {
		t.Fatalf("failed processor must drop: %s", buf.String())
	}
	if l.Metrics().Snapshot().Dropped == 0 {
		t.Fatal("drop not counted")
	}
}

func TestWithLevelerAndPresetTest(t *testing.T) {
	var buf bytes.Buffer
	tl := Test(&buf)
	tl.Info("via test preset")
	if !strings.Contains(buf.String(), "via test preset") {
		t.Fatal("Test preset")
	}
	lv := NewLevelVar(LevelError)
	l := New(WithLeveler(lv), WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)))
	if l.Enabled(LevelInfo) {
		t.Fatal("WithLeveler not applied")
	}
}

func TestStdLoggerAndMetricsSinkError(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	std := l.StdLogger(LevelWarn)
	std.Println("legacy")
	if !strings.Contains(buf.String(), "legacy") || !strings.Contains(buf.String(), `"warn"`) {
		t.Fatalf("StdLogger: %s", buf.String())
	}
	l.Metrics().IncSinkError()
	if l.Metrics().Snapshot().SinkErrors != 1 {
		t.Fatal("IncSinkError")
	}
}

func TestFingersCrossedSyncCloseAndGlobal(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	fc := NewFingersCrossed(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace))
	l := New(WithTransport(NewSyncTransport(fc)), WithLevel(LevelTrace))
	// no scope → global ring; error triggers global flush
	l.Debug("g1")
	l.Error("gerr")
	if !strings.Contains(buf.String(), "g1") || !strings.Contains(buf.String(), "gerr") {
		t.Fatalf("global fingerscrossed: %s", buf.String())
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAuditSyncClose(t *testing.T) {
	var buf bytes.Buffer
	a := NewAuditSink(&buf, NewJSONEncoder())
	if err := a.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	// malformed line detection
	res := VerifyAudit(strings.NewReader("garbage line without parts\n"))
	if res.OK {
		t.Fatal("malformed audit must fail verification")
	}
}

func TestAdminParseLevelVariants(t *testing.T) {
	for in, want := range map[string]Level{
		"trace": LevelTrace, "DEBUG": LevelDebug, "warning": LevelWarn,
		"fatal": LevelFatal, "17": LevelError,
	} {
		got, ok := parseLevel(in)
		if !ok || got != want {
			t.Fatalf("parseLevel(%q)=%v,%v want %v", in, got, ok, want)
		}
	}
	if _, ok := parseLevel("nope"); ok {
		t.Fatal("invalid level parsed")
	}
	if _, ok := parseLevel("99"); ok {
		t.Fatal("out-of-range severity parsed")
	}
	// handler bad input + method-not-allowed
	h := NewLevelHandler(NewLevelVar(LevelInfo))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/l?level=bogus", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad level → %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/l", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE → %d", rec.Code)
	}
}

func TestConfigReloadOnReloadAndStopIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/c.json"
	_ = os.WriteFile(path, []byte(`{"level":"error"}`), 0o644)
	lv := NewLevelVar(LevelInfo)
	w, stop := WatchConfigFile(path, lv, time.Second)
	got := make(chan FileConfig, 1)
	w.OnReload(func(c FileConfig) {
		select {
		case got <- c:
		default:
		}
	})
	// trigger a reload by rewriting after mtime gap
	time.Sleep(1100 * time.Millisecond)
	_ = os.WriteFile(path, []byte(`{"level":"trace"}`), 0o644)
	select {
	case c := <-got:
		if c.Level == "" {
			t.Fatal("OnReload empty")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("OnReload never fired")
	}
	stop()
	stop() // idempotent — must not panic
	w.Stop()
}

func TestUDPSinkAndNetClose(t *testing.T) {
	s := NewUDPSink("127.0.0.1:65500", NewJSONEncoder(), LevelInfo)
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	l.Info("udp datagram") // fire-and-forget; no listener is fine for UDP
	_ = s.Sync()
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// closing a never-dialed sink is safe
	if err := NewTCPSink("127.0.0.1:1", NewJSONEncoder(), LevelInfo).Close(); err != nil {
		t.Fatalf("close undialed: %v", err)
	}
}

func TestHTTPBatchDroppedOnBadURL(t *testing.T) {
	fixedTime(t)
	s := NewElasticsearchSink("http://127.0.0.1:1/_bulk", "idx", LevelInfo)
	s.MaxBatch = 1
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	l.Info("will fail to ship")
	deadline := time.After(3 * time.Second)
	for s.Dropped() == 0 {
		select {
		case <-deadline:
			t.Fatal("expected dropped count on unreachable ES")
		case <-time.After(50 * time.Millisecond):
		}
	}
	_ = l.Close()
}

func TestAnyvalTextModePaths(t *testing.T) {
	b := getBuffer()
	appendAny(b, fmt.Errorf("wrapped"), false)
	appendAny(b, map[string]int{"a": 1}, false)
	appendAny(b, fakeStringer{}, false)
	appendAny(b, nil, false)
	out := string(b.b)
	putBuffer(b)
	if !strings.Contains(out, "wrapped") || !strings.Contains(out, "I-stringify") || !strings.Contains(out, "null") {
		t.Fatalf("anyval text paths: %q", out)
	}
}

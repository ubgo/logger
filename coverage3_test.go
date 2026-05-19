package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLevelColorAllBands(t *testing.T) {
	seen := map[string]bool{}
	for _, lv := range []Level{LevelTrace, LevelDebug, LevelInfo, LevelWarn, LevelError, LevelFatal} {
		c := levelColor(lv)
		if c == "" || !strings.HasPrefix(c, "\x1b[") {
			t.Fatalf("levelColor(%v)=%q", lv, c)
		}
		seen[c] = true
	}
	if len(seen) < 4 {
		t.Fatalf("expected distinct colors per band, got %d", len(seen))
	}
}

func TestAppendAnyJSONScalarBranches(t *testing.T) {
	b := getBuffer()
	for _, v := range []any{
		"str", true, 7, int64(8), uint64(9), 1.5,
		errors.New("e"), fakeStringer{}, nil,
		map[string]int{"a": 1}, // json.Marshal path
		make(chan int),         // json.Marshal error path → !ERR
	} {
		appendAny(b, v, true)
		b.writeByte('\n')
	}
	out := string(b.b)
	putBuffer(b)
	for _, want := range []string{`"str"`, "true", "7", "8", "9", "1.5",
		`"e"`, `"I-stringify"`, "null", `{"a":1}`, "!ERR:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("appendAny json missing %q in:\n%s", want, out)
		}
	}
}

func TestLogfmtAllValueShapes(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	l := New(WithSink(NewWriterSink(&buf, NewLogfmtEncoder(), LevelTrace)),
		WithLevel(LevelTrace))
	l.Info("logfmt shapes",
		String("plain", "v"),
		String("spaced", "a b"),
		String("empty", ""),
		Int("n", 3),
		Float("f", 2.5),
		Bool("ok", true),
		Dur("d", time.Second),
		Time("ts", time.Unix(0, 0).UTC()),
		Err(errors.New("bad")),
		NamedErr("nilerr", nil),
		Any("obj", map[string]int{"x": 1}),
	)
	out := buf.String()
	for _, want := range []string{`plain=v`, `spaced="a b"`, `empty=""`,
		`n=3`, `f=2.5`, `ok=true`, `d=1s`, `bad`, `nilerr=null`} {
		if !strings.Contains(out, want) {
			t.Fatalf("logfmt missing %q in: %s", want, out)
		}
	}
}

func TestEnrichEdgeCases(t *testing.T) {
	fixedTime(t)
	// nil ctx → returns early; no extractor; extractor returning ok=false;
	// then a working extractor.
	noop := func(context.Context) (string, string, bool) { return "", "", false }
	ok := func(context.Context) (string, string, bool) { return "tid", "", true }
	e := NewEnrichProcessor(noop, ok)
	e.TraceKey = ""
	e.SpanKey = ""
	l, buf := newBufLogger(WithProcessors(e), WithLevel(LevelTrace))
	l.Info("no ctx fields") // ctx == background, no bound fields
	l.InfoContext(context.Background(), "still none")
	ctx := ContextWith(context.Background(), String("rid", "1"))
	l.InfoContext(ctx, "enriched")
	out := buf.String()
	if !strings.Contains(out, `"rid":"1"`) || !strings.Contains(out, `"trace_id":"tid"`) {
		t.Fatalf("enrich edge: %s", out)
	}
	// EnrichProcessor.Process with a literally nil ctx and nil r.Ctx
	r := newRecord()
	r.Ctx = nil
	if err := e.Process(context.TODO(), r); err != nil {
		t.Fatalf("enrich nil-ctx: %v", err)
	}
	r.release()
}

func TestSlogHandlerAllAttrKinds(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	core := New(WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelTrace))
	sl := core.NewSlog()
	sl.LogAttrs(context.Background(), slog.LevelInfo, "all kinds",
		slog.String("s", "x"),
		slog.Int64("i", 1),
		slog.Uint64("u", 2),
		slog.Float64("f", 3.5),
		slog.Bool("b", true),
		slog.Duration("d", time.Second),
		slog.Time("t", time.Unix(0, 0).UTC()),
		slog.Any("a", []int{1, 2}),
		slog.Group("g", slog.String("inner", "v")),
	)
	// inline group (empty key) + empty attr are skipped
	sl.WithGroup("").Info("empty group noop", slog.Attr{})
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.SplitN(buf.String(), "\n", 2)[0]), &m); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	for _, k := range []string{"s", "i", "u", "f", "b", "d", "t", "a", "g.inner"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("slog attr kind %q missing: %v", k, m)
		}
	}
}

func TestDedupDefaultWindowAndCtxOr(t *testing.T) {
	d := NewDedupProcessor(0) // <=0 → default 5s
	if d.Window != 5*time.Second {
		t.Fatalf("default window = %v", d.Window)
	}
	if got := ctxOr(context.Background()); got == nil {
		t.Fatal("ctxOr(non-nil) returned nil")
	}
	if got := ctxOr(nil); got == nil {
		t.Fatal("ctxOr(nil) must return Background")
	}
}

type syncCloseBuf struct {
	bytes.Buffer
	synced, closed bool
}

func (s *syncCloseBuf) Sync() error  { s.synced = true; return nil }
func (s *syncCloseBuf) Close() error { s.closed = true; return nil }

func TestAuditSyncCloseDelegate(t *testing.T) {
	scb := &syncCloseBuf{}
	a := NewAuditSink(scb, NewJSONEncoder())
	if err := a.Sync(); err != nil || !scb.synced {
		t.Fatalf("AuditSink.Sync must delegate: %v synced=%v", err, scb.synced)
	}
	if err := a.Close(); err != nil || !scb.closed {
		t.Fatalf("AuditSink.Close must delegate: %v closed=%v", err, scb.closed)
	}
}

func TestFingersCrossedRingOverflowAndDefaults(t *testing.T) {
	b := newFCBuffer(0) // <1 → default cap 256
	if b.cap != 256 {
		t.Fatalf("default cap = %d", b.cap)
	}
	small := newFCBuffer(2)
	small.push(&Record{Message: "a"})
	small.push(&Record{Message: "b"})
	small.push(&Record{Message: "c"}) // overwrites oldest "a"
	got := small.drain()
	if len(got) != 2 || got[0].Message != "b" || got[1].Message != "c" {
		t.Fatalf("ring overwrite-oldest wrong: %+v", got)
	}
}

func TestNewRotatingFileError(t *testing.T) {
	// a path whose parent cannot be created (a file used as a directory)
	tmp := t.TempDir() + "/afile"
	if err := writeFileHelper(tmp, "x"); err != nil {
		t.Fatal(err)
	}
	_, err := NewRotatingFile(tmp + "/under/app.log")
	if err == nil {
		t.Fatal("expected error creating log under a non-directory path")
	}
}

func TestHexDigitHighNibble(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	l.Info("ctrl", String("k", "\x1f"))
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("hexDigit produced invalid JSON: %v", err)
	}
	if m["k"] != "\x1f" {
		t.Fatalf("hexDigit round-trip: %q", m["k"])
	}
}

func writeFileHelper(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

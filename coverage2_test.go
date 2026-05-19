package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestFanoutEmitSyncCloseAndError(t *testing.T) {
	fixedTime(t)
	var a, b bytes.Buffer
	s1 := NewWriterSink(&a, NewJSONEncoder(), LevelTrace)
	s2 := NewWriterSink(&b, NewJSONEncoder(), LevelError) // higher level
	bad := badSink{}
	fan := NewFanout(s1, s2, bad)
	var gotErr error
	fan.OnError = func(_ Sink, e error) { gotErr = e }

	l := New(WithTransport(NewSyncTransport(fan)), WithLevel(LevelTrace))
	l.Info("info-line") // s1 only (s2 is Error+)
	l.Error("err-line") // both
	if !strings.Contains(a.String(), "info-line") || strings.Contains(b.String(), "info-line") {
		t.Fatalf("per-sink level not honored")
	}
	if gotErr == nil {
		t.Fatal("Fanout.OnError not invoked for failing sink")
	}
	if err := fan.Sync(); err == nil {
		t.Fatal("Fanout.Sync should surface the bad sink error")
	}
	if err := fan.Close(); err == nil {
		t.Fatal("Fanout.Close should surface the bad sink error")
	}
}

type badSink struct{}

func (badSink) Emit(*Record) error { return errors.New("sink down") }
func (badSink) Sync() error        { return errors.New("sync fail") }
func (badSink) Close() error       { return errors.New("close fail") }

func TestTemplatedLevelMethods(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.Debugt("d {x}", 1)
	l.Infot("i {x}", 2)
	l.Warnt("w {x}", 3)
	l.Errort("e {x}", 4)
	l.Logt(context.Background(), LevelInfo, "L {x}", 5)
	l.Infot("unterminated {oops and {ok}", "v") // unterminated brace path
	l.Infot("trailing hole {a} {b}", 1)         // fewer args than holes
	out := buf.String()
	for _, w := range []string{"d 1", "i 2", "w 3", "e 4", "L 5"} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in %s", w, out)
		}
	}
}

func TestSpanSetLevel(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	_, s := l.StartSpan(context.Background(), "op")
	s.SetLevel(LevelWarn)
	s.End()
	if !strings.Contains(buf.String(), `"level":"warn"`) {
		t.Fatalf("SetLevel not applied: %s", buf.String())
	}
}

func TestSlogToLevelAllBands(t *testing.T) {
	cases := map[slog.Level]Level{
		slog.LevelDebug - 1: LevelTrace,
		slog.LevelDebug:     LevelDebug,
		slog.LevelInfo:      LevelInfo,
		slog.LevelWarn:      LevelWarn,
		slog.LevelError:     LevelError,
	}
	for in, want := range cases {
		if got := slogToLevel(in); got != want {
			t.Fatalf("slogToLevel(%v)=%v want %v", in, got, want)
		}
	}
}

func TestJSONControlCharEscaping(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger()
	l.Info("ctrl", String("k", "tab\tnl\nbell\x07end"))
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &m); err != nil {
		t.Fatalf("control chars produced invalid JSON: %v", err)
	}
	if m["k"] != "tab\tnl\nbell\x07end" {
		t.Fatalf("round-trip mismatch: %q", m["k"])
	}
}

func TestGzipFileRotation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "g.log")
	rf, err := NewRotatingFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rf.MaxSizeBytes = 120
	rf.MaxBackups = 5
	rf.Compress = true
	l := New(WithSink(NewFileSink(rf, NewJSONEncoder(), LevelTrace)), WithLevel(LevelTrace))
	for i := 0; i < 80; i++ {
		l.Info("compress me", String("pad", strings.Repeat("y", 20)))
	}
	_ = l.Sync()
	_ = l.Close()
	// give the async gzip goroutine a beat
	deadline := time.After(3 * time.Second)
	for {
		entries, _ := os.ReadDir(dir)
		gz := false
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".gz") {
				gz = true
			}
		}
		if gz {
			break
		}
		select {
		case <-deadline:
			t.Fatal("no .gz segment produced by Compress rotation")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestTransportDroppedAndSyncAccessors(t *testing.T) {
	sink := NewWriterSink(&bytes.Buffer{}, NewJSONEncoder(), LevelTrace)

	st := NewSyncTransport(sink)
	if st.Dropped() != 0 || st.Sync() != nil {
		t.Fatal("sync transport accessors")
	}
	ct := NewChannelTransport(sink, 8, DropNewest)
	_ = ct.Dropped()
	_ = ct.Sync()
	_ = ct.Close()
	rt := NewRingTransport(sink, 8, DropNewest)
	_ = rt.Dropped()
	_ = rt.Sync()
	_ = rt.Close()
	dt := NewDisruptorTransport(sink, 8, DropNewest)
	_ = dt.Sync()
	_ = dt.Close()
}

func TestChannelTransportDropNewestCounts(t *testing.T) {
	sink := NewWriterSink(slowWriter2{}, NewJSONEncoder(), LevelTrace)
	ct := NewChannelTransport(sink, 2, DropNewest)
	l := New(WithTransport(ct), WithLevel(LevelTrace))
	for i := 0; i < 500; i++ {
		l.Info("flood")
	}
	_ = l.Close()
	if ct.Dropped() == 0 {
		t.Fatal("channel DropNewest should report drops under a slow sink")
	}
}

type slowWriter2 struct{}

func (slowWriter2) Write(p []byte) (int, error) {
	time.Sleep(time.Millisecond)
	return len(p), nil
}

func TestCycleLevelOnSignal(t *testing.T) {
	lv := NewLevelVar(LevelInfo)
	stop := CycleLevelOnSignal(lv, syscall.SIGUSR2, LevelInfo, LevelDebug)
	defer stop()
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	deadline := time.After(2 * time.Second)
	for lv.Level() != LevelDebug {
		select {
		case <-deadline:
			t.Fatalf("signal did not flip level, still %v", lv.Level())
		case <-time.After(50 * time.Millisecond):
		}
	}
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGUSR2)
	deadline = time.After(2 * time.Second)
	for lv.Level() != LevelInfo {
		select {
		case <-deadline:
			t.Fatal("signal did not flip back")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestSyslogSinkBestEffort(t *testing.T) {
	s, err := NewSyslogSink("", "", "ubgologtest", NewJSONEncoder(), LevelInfo)
	if err != nil {
		t.Skipf("no local syslog in this environment: %v", err)
	}
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelTrace))
	l.Trace("t")
	l.Debug("d")
	l.Info("i")
	l.Warn("w")
	l.Error("e")
	l.Log(context.Background(), LevelFatal, "f")
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTLSSinkConstructorAndDroppedNoServer(t *testing.T) {
	s := NewTLSSink("127.0.0.1:1", nil, NewJSONEncoder(), LevelInfo)
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	l.Info("will fail to dial TLS")
	if s.Dropped() == 0 {
		t.Fatal("TLS sink should count a failed dial as dropped")
	}
	_ = s.Close()
}

func TestHTTPBatchFlushTimerAndSync(t *testing.T) {
	fixedTime(t)
	var mu sync.Mutex
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		w.WriteHeader(204)
	}))
	defer srv.Close()
	s := NewLokiSink(srv.URL, map[string]string{"a": "b"}, LevelInfo)
	s.MaxBatch = 1000 // won't hit by size
	s.Flush = 200 * time.Millisecond
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	l.Info("one") // should flush via the timer
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		h := hits
		mu.Unlock()
		if h > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("flush timer never fired")
		case <-time.After(50 * time.Millisecond):
		}
	}
	_ = s.Sync()
	_ = l.Close()
}

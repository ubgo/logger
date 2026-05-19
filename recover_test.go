package logger

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecoverRepanicsAndLogs(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("Recover must re-panic")
			}
		}()
		defer l.Recover(context.Background())
		panic("boom")
	}()
	out := buf.String()
	if !strings.Contains(out, `"level":"fatal"`) || !strings.Contains(out, "boom") {
		t.Fatalf("panic not logged at fatal: %s", out)
	}
	if !strings.Contains(out, `"stack"`) {
		t.Fatalf("stack not captured: %s", out)
	}
}

func TestRecoverAndContinueSwallows(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	worker := func() {
		defer l.RecoverAndContinue(context.Background())
		panic("worker blew up")
	}
	worker() // if the panic propagated, the test process would crash here
	if !strings.Contains(buf.String(), "worker blew up") {
		t.Fatalf("panic not logged: %s", buf.String())
	}
}

// signalSink emits into an inner sink and closes done after the first record
// — establishing a happens-before edge so the test can read safely (the
// goroutine's deferred recover-log is the thing we must wait for, not fn).
type signalSink struct {
	inner Sink
	done  chan struct{}
	once  sync.Once
}

func (s *signalSink) Emit(r *Record) error {
	err := s.inner.Emit(r)
	s.once.Do(func() { close(s.done) })
	return err
}
func (s *signalSink) Sync() error  { return s.inner.Sync() }
func (s *signalSink) Close() error { return s.inner.Close() }

func TestGoSafeGoroutine(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	ss := &signalSink{inner: NewWriterSink(&buf, NewJSONEncoder(), LevelTrace), done: make(chan struct{})}
	l := New(WithTransport(NewSyncTransport(ss)), WithLevel(LevelTrace))

	l.Go(context.Background(), func() { panic("goroutine panic") })

	select {
	case <-ss.done: // recover-log emitted (happens-before our read)
	case <-time.After(2 * time.Second):
		t.Fatal("safe goroutine never logged the panic")
	}
	if !strings.Contains(buf.String(), "goroutine panic") {
		t.Fatalf("safe goroutine did not log panic: %s", buf.String())
	}
}

func TestPresetsConstruct(t *testing.T) {
	d := Development()
	if !d.Enabled(LevelDebug) {
		t.Fatal("Development should enable Debug")
	}
	p := Production()
	if p.Enabled(LevelDebug) {
		t.Fatal("Production should not enable Debug")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Production Close: %v", err)
	}
}

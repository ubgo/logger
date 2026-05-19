package logger

import (
	"bytes"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestNetSinkRedialAfterWriteError exercises the resilience path: the server
// accepts, reads one message, then drops the connection so the next write
// fails and the sink must transparently re-dial.
func TestNetSinkRedialAfterWriteError(t *testing.T) {
	fixedTime(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	var mu sync.Mutex
	got := 0
	accepted := make(chan struct{}, 8)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepted <- struct{}{}
			go func(c net.Conn) {
				buf := make([]byte, 4096)
				n, _ := c.Read(buf)
				if n > 0 {
					mu.Lock()
					got++
					mu.Unlock()
				}
				_ = c.Close() // drop after first message → forces re-dial
			}(c)
		}
	}()

	s := NewTCPSink(ln.Addr().String(), NewJSONEncoder(), LevelInfo)
	l := New(WithTransport(NewSyncTransport(s)), WithLevel(LevelInfo))
	for i := 0; i < 6; i++ {
		l.Info("resilient", Int("i", i))
		time.Sleep(20 * time.Millisecond) // let the server close between writes
	}
	// at least two separate connections must have been accepted (re-dial)
	conns := 0
	timeout := time.After(2 * time.Second)
	for conns < 2 {
		select {
		case <-accepted:
			conns++
		case <-timeout:
			t.Fatalf("expected >=2 connections (re-dial), got %d", conns)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if got < 2 {
		t.Fatalf("server received %d messages, expected re-delivery after drop", got)
	}
	_ = s.Close()
}

// slog handler logAt: below-level gate + the With()-bound-fields branch.
func TestSlogLogAtWithFieldsAndGate(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	core := New(
		WithSink(NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)),
		WithLevel(LevelWarn),
	).With(String("bound", "yes")) // exercises len(l.with)>0 in logAt
	sl := core.NewSlog()
	sl.Info("gated via logAt") // below Warn → early return in logAt
	sl.Warn("kept via logAt")  // emitted, carries bound field
	out := buf.String()
	if strings.Contains(out, "gated via logAt") {
		t.Fatalf("logAt did not gate: %s", out)
	}
	if !strings.Contains(out, `"bound":"yes"`) || !strings.Contains(out, "kept via logAt") {
		t.Fatalf("logAt with-branch: %s", out)
	}
}

// Child-logger Event() exercises eventAt's len(l.with)>0 branch.
func TestChildLoggerEventInheritsFields(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelTrace))
	l.With(String("svc", "api")).Event("user.created", Int("uid", 1))
	out := buf.String()
	if !strings.Contains(out, `"event":"user.created"`) || !strings.Contains(out, `"svc":"api"`) {
		t.Fatalf("child Event inheritance: %s", out)
	}
}

// Sampler with Thereafter==0 must disable sampling (pass everything).
func TestSamplerDisabledWhenThereafterZero(t *testing.T) {
	fixedTime(t)
	sp := NewSampleProcessor(0, 0) // Thereafter 0 → no sampling
	l, buf := newBufLogger(WithProcessors(sp), WithLevel(LevelTrace))
	for i := 0; i < 20; i++ {
		l.Info("kept")
	}
	if strings.Count(buf.String(), `"msg":"kept"`) != 20 {
		t.Fatalf("Thereafter=0 must disable sampling")
	}
	if sp.Dropped() != 0 {
		t.Fatal("no drops expected when sampling disabled")
	}
}

// PathRedactor Hash on a non-string field hits hashValue's sprintAny branch.
func TestRedactHashNonStringField(t *testing.T) {
	fixedTime(t)
	pr := NewPathRedactor(Hash, "", "secret_id")
	l, buf := newBufLogger(WithProcessors(pr))
	l.Info("x", Int("secret_id", 12345))
	if strings.Contains(buf.String(), "12345") {
		t.Fatalf("hashed int field leaked: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "sha256:") {
		t.Fatalf("hash marker missing: %s", buf.String())
	}
}

// Templated call below the active level hits logt's disabled branch.
func TestTemplatedCallBelowLevel(t *testing.T) {
	fixedTime(t)
	l, buf := newBufLogger(WithLevel(LevelError))
	l.Infot("never {x}", 1) // below Error
	if buf.Len() != 0 {
		t.Fatalf("templated below-level should be dropped: %s", buf.String())
	}
}

// Console encoder without color, with caller, and with zero fields.
func TestConsoleEncoderNoColorWithCallerNoFields(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	enc := NewConsoleEncoder() // Color=false
	l := New(
		WithSink(NewWriterSink(&buf, enc, LevelTrace)),
		WithLevel(LevelTrace),
		WithCaller(0),
	)
	l.Info("plain line") // no fields, caller on, no color
	out := buf.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("no-color console must not emit ANSI: %q", out)
	}
	if !strings.Contains(out, "plain line") || !strings.Contains(out, "<") {
		t.Fatalf("console caller render: %q", out)
	}
}

// Ring transport DropNewest under a slow sink: the drop branch + counter.
func TestRingTransportDropNewest(t *testing.T) {
	rt := NewRingTransport(NewWriterSink(slowWriter2{}, NewJSONEncoder(), LevelTrace), 2, DropNewest)
	l := New(WithTransport(rt), WithLevel(LevelTrace))
	for i := 0; i < 500; i++ {
		l.Info("flood")
	}
	_ = l.Close()
	if rt.Dropped() == 0 {
		t.Fatal("ring DropNewest should report drops under a slow sink")
	}
}

// Disruptor DropNewest under a slow sink.
func TestDisruptorDropNewest(t *testing.T) {
	dt := NewDisruptorTransport(NewWriterSink(slowWriter2{}, NewJSONEncoder(), LevelTrace), 2, DropNewest)
	l := New(WithTransport(dt), WithLevel(LevelTrace))
	for i := 0; i < 500; i++ {
		l.Info("flood")
	}
	_ = l.Close()
	if dt.Dropped() == 0 {
		t.Fatal("disruptor DropNewest should report drops under a slow sink")
	}
}

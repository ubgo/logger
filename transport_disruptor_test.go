package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDisruptorBlockNoLoss(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	sink := NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)
	tr := NewDisruptorTransport(sink, 64, Block) // small ring forces contention
	l := New(WithTransport(tr), WithLevel(LevelTrace))

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 250; i++ {
				l.Info("x")
			}
		}()
	}
	wg.Wait()
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if got := strings.Count(buf.String(), `"msg":"x"`); got != 2000 {
		t.Fatalf("lock-free Block transport lost records: got %d want 2000", got)
	}
}

func TestDisruptorDropOldestCounts(t *testing.T) {
	var buf bytes.Buffer
	sink := NewWriterSink(slowWriter{&buf}, NewJSONEncoder(), LevelTrace)
	tr := NewDisruptorTransport(sink, 4, DropOldest)
	l := New(WithTransport(tr), WithLevel(LevelTrace))
	for i := 0; i < 2000; i++ {
		l.Info("flood")
	}
	_ = l.Close()
	if tr.Dropped() == 0 {
		t.Fatal("expected DropOldest to report drops under flood")
	}
}

type slowWriter struct{ w *bytes.Buffer }

func (s slowWriter) Write(p []byte) (int, error) {
	time.Sleep(time.Microsecond)
	return s.w.Write(p)
}

func TestDisruptorDrainsOnClose(t *testing.T) {
	fixedTime(t)
	var buf bytes.Buffer
	sink := NewWriterSink(&buf, NewJSONEncoder(), LevelTrace)
	tr := NewDisruptorTransport(sink, 1024, Block)
	l := New(WithTransport(tr), WithLevel(LevelTrace))
	for i := 0; i < 500; i++ {
		l.Info("pending")
	}
	if err := l.Close(); err != nil { // must flush the in-flight ring
		t.Fatalf("close: %v", err)
	}
	if got := strings.Count(buf.String(), `"msg":"pending"`); got != 500 {
		t.Fatalf("Close did not drain the ring: got %d want 500", got)
	}
}

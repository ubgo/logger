package logger

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMetricsCountsEmittedAndDropped(t *testing.T) {
	fixedTime(t)
	sp := NewSampleProcessor(0, 1_000_000) // drop ~all info
	l, _ := newBufLogger(WithProcessors(sp), WithLevel(LevelTrace))
	for i := 0; i < 20; i++ {
		l.Info("noise")
	}
	l.Error("real")

	s := l.Metrics().Snapshot()
	if s.Emitted == 0 {
		t.Fatal("expected some emitted records")
	}
	if s.Dropped == 0 {
		t.Fatal("expected sampler drops counted in metrics")
	}
	if s.ByLevel["ERROR"] != 1 {
		t.Fatalf("ByLevel ERROR = %d, want 1", s.ByLevel["ERROR"])
	}

	rec := httptest.NewRecorder()
	l.Metrics().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), `"emitted":`) {
		t.Fatalf("metrics endpoint malformed: %s", rec.Body.String())
	}
}

func TestOnSIGHUPFires(t *testing.T) {
	fired := make(chan struct{}, 1)
	stop := OnSIGHUP(func() { fired <- struct{}{} })
	defer stop()
	_ = syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("SIGHUP handler did not fire")
	}
}

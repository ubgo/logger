package sentrylogger

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	logger "github.com/ubgo/logger"
)

// mockTransport captures events instead of shipping them.
type mockTransport struct {
	mu     sync.Mutex
	events []*sentry.Event
	closed bool
}

func (m *mockTransport) Configure(sentry.ClientOptions) {}
func (m *mockTransport) SendEvent(e *sentry.Event) {
	m.mu.Lock()
	m.events = append(m.events, e)
	m.mu.Unlock()
}
func (m *mockTransport) Flush(time.Duration) bool              { return true }
func (m *mockTransport) FlushWithContext(context.Context) bool { return true }
func (m *mockTransport) Close()                                { m.closed = true }

func newTestHub(t *testing.T) (*sentry.Hub, *mockTransport) {
	t.Helper()
	mt := &mockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:       "https://public@example.com/1",
		Transport: mt,
	})
	if err != nil {
		t.Fatal(err)
	}
	return sentry.NewHub(client, sentry.NewScope()), mt
}

func TestSentrySinkLevelMappingAndExceptions(t *testing.T) {
	hub, mt := newTestHub(t)
	s := NewWithHub(hub, logger.LevelWarn)
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(s)),
		logger.WithLevel(logger.LevelTrace),
	)

	l.Info("below threshold — dropped")
	l.Warn("disk low", logger.String("dev", "sda"))
	l.Error("save failed", logger.Err(errors.New("io timeout")),
		logger.String("table", "orders"))
	l.Log(context.Background(), logger.LevelFatal, "fatal one")

	mt.mu.Lock()
	defer mt.mu.Unlock()
	if len(mt.events) != 3 { // Info filtered out by minLevel
		t.Fatalf("got %d events, want 3", len(mt.events))
	}
	if mt.events[0].Level != sentry.LevelWarning {
		t.Fatalf("warn → %v", mt.events[0].Level)
	}
	if mt.events[1].Level != sentry.LevelError || len(mt.events[1].Exception) == 0 {
		t.Fatalf("error event missing exception: %+v", mt.events[1])
	}
	if mt.events[1].Contexts["fields"]["table"] != "orders" {
		t.Fatalf("field context lost: %+v", mt.events[1].Contexts)
	}
	if mt.events[2].Level != sentry.LevelFatal {
		t.Fatalf("fatal → %v", mt.events[2].Level)
	}
}

func TestSentryEventNameTag(t *testing.T) {
	hub, mt := newTestHub(t)
	s := NewWithHub(hub, logger.LevelWarn)
	l := logger.New(logger.WithTransport(logger.NewSyncTransport(s)),
		logger.WithLevel(logger.LevelTrace))
	l.EventAt(context.Background(), logger.LevelError, "payment.failed",
		logger.String("order", "o-1"))
	mt.mu.Lock()
	defer mt.mu.Unlock()
	if len(mt.events) != 1 || mt.events[0].Message != "payment.failed" ||
		mt.events[0].Tags["event"] != "payment.failed" {
		t.Fatalf("event-name mapping: %+v", mt.events[0])
	}
}

func TestSentryNewClampsLevelAndSyncClose(t *testing.T) {
	// New() with a too-low level must clamp to Warn.
	s := New(logger.LevelDebug)
	if s.minLvl != logger.LevelWarn {
		t.Fatalf("New did not clamp minLevel: %v", s.minLvl)
	}
	hub, _ := newTestHub(t)
	s2 := NewWithHub(hub, logger.LevelError)
	if err := s2.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := s2.Close(); err != nil {
		t.Fatal(err)
	}
	// below-minLevel Emit returns nil without sending
	if err := s2.Emit(&logger.Record{Level: logger.LevelInfo, Message: "x"}); err != nil {
		t.Fatal(err)
	}
}

package logtest_test

import (
	"testing"

	logger "github.com/ubgo/logger"
	"github.com/ubgo/logger/logtest"
)

func TestCaptureAndAssertions(t *testing.T) {
	l, cap := logtest.New()
	l.Info("user created", logger.String("user", "ada"), logger.Int("id", 7))
	l.Warn("disk low")

	cap.AssertLogged(t, logger.LevelInfo, "user created")
	cap.AssertLogged(t, logger.LevelWarn, "disk low")
	cap.AssertField(t, "user", "ada")
	cap.AssertField(t, "id", int64(7))
	cap.AssertNoErrors(t)

	if got := len(cap.Entries()); got != 2 {
		t.Fatalf("captured %d entries, want 2", got)
	}
}

func TestCaptureSeesErrorLevel(t *testing.T) {
	l, cap := logtest.New()
	l.Error("kaboom")
	es := cap.Entries()
	if len(es) != 1 || es[0].Level != logger.LevelError {
		t.Fatalf("expected one ERROR entry, got %v", es)
	}
	// AssertNoErrors would (correctly) fail here — not exercised against a
	// real *testing.T since a forced Fatalf aborts the goroutine.
}

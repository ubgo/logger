package zerologlogger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	logger "github.com/ubgo/logger"
)

func TestZerologAllLevelsFieldsEventMinLevel(t *testing.T) {
	var buf bytes.Buffer
	zl := zerolog.New(&buf)
	s := New(zl, logger.LevelDebug) // Trace must be filtered
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(s)),
		logger.WithLevel(logger.LevelTrace),
	)
	l.Trace("dropped-trace")
	l.Debug("d")
	l.Info("i", logger.String("s", "x"), logger.Int("n", 2))
	l.Warn("w")
	l.Error("e")
	l.Event("evt.k", logger.Bool("b", true))

	out := buf.String()
	if strings.Contains(out, "dropped-trace") {
		t.Fatalf("minLevel not enforced: %s", out)
	}
	for _, w := range []string{`"level":"debug"`, `"level":"info"`,
		`"level":"warn"`, `"level":"error"`, `"s":"x"`, `"event":"evt.k"`} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in: %s", w, out)
		}
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestZerologLevelMap(t *testing.T) {
	for in, want := range map[logger.Level]zerolog.Level{
		logger.LevelTrace: zerolog.TraceLevel,
		logger.LevelDebug: zerolog.DebugLevel,
		logger.LevelInfo:  zerolog.InfoLevel,
		logger.LevelWarn:  zerolog.WarnLevel,
		logger.LevelError: zerolog.ErrorLevel,
		logger.LevelFatal: zerolog.FatalLevel,
	} {
		if got := mapLevel(in); got != want {
			t.Fatalf("mapLevel(%v)=%v want %v", in, got, want)
		}
	}
}

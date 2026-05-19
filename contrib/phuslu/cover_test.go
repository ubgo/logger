package phulogger

import (
	"bytes"
	"strings"
	"testing"

	plog "github.com/phuslu/log"
	logger "github.com/ubgo/logger"
)

func TestPhusluAllLevelsFieldsEventMinLevel(t *testing.T) {
	var buf bytes.Buffer
	pl := &plog.Logger{Level: plog.TraceLevel, Writer: &plog.IOWriter{Writer: &buf}}
	s := New(pl, logger.LevelDebug) // Trace filtered by minLevel
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(s)),
		logger.WithLevel(logger.LevelTrace),
	)
	l.Trace("dropped")
	l.Debug("d")
	l.Info("i", logger.String("s", "x"), logger.Int("n", 4), logger.Bool("b", true))
	l.Warn("w")
	l.Error("e")
	// NB: do not emit Fatal through phuslu in tests — phuslu's Fatal
	// .Msg() calls os.Exit. entryFor's Fatal branch is covered below
	// without calling .Msg().
	l.Event("evt.x", logger.String("a", "b"))

	out := buf.String()
	if strings.Contains(out, "dropped") {
		t.Fatalf("minLevel not enforced: %s", out)
	}
	for _, w := range []string{`"level":"debug"`, `"level":"info"`,
		`"level":"warn"`, `"level":"error"`, `"s":"x"`, `"event":"evt.x"`} {
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

func TestPhusluEntryForLevels(t *testing.T) {
	pl := &plog.Logger{Level: plog.TraceLevel, Writer: plog.IOWriter{Writer: &bytes.Buffer{}}}
	for _, lv := range []logger.Level{
		logger.LevelTrace, logger.LevelDebug, logger.LevelInfo,
		logger.LevelWarn, logger.LevelError, logger.LevelFatal,
	} {
		if e := entryFor(pl, lv); e == nil {
			t.Fatalf("entryFor(%v) nil", lv)
		}
	}
}

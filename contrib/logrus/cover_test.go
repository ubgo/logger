package logruslogger

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logger "github.com/ubgo/logger"
)

func TestLogrusAllLevelsAndFieldTypes(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	lr := logrus.New()
	lr.SetLevel(logrus.TraceLevel)
	Attach(lr, core)

	lr.Trace("t")
	lr.Debug("d")
	lr.Info("i")
	lr.Warn("w")
	lr.Error("e")
	lr.WithContext(context.Background()).Info("with-ctx")
	lr.WithFields(logrus.Fields{
		"s": "x", "b": true, "i": 1, "i64": int64(2),
		"f": 1.5, "err": errors.New("boom"), "any": []int{1},
	}).Info("typed")

	out := buf.String()
	for _, w := range []string{`"level":"trace"`, `"level":"debug"`, `"level":"info"`,
		`"level":"warn"`, `"level":"error"`, "with-ctx",
		`"s":"x"`, `"b":true`, `"i":1`, `"i64":2`, `"f":1.5`, "boom"} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in: %s", w, out)
		}
	}
}

func TestLogrusHookLevelsAndFatalMap(t *testing.T) {
	core := logger.New()
	h := NewHook(core)
	if len(h.Levels()) == 0 {
		t.Fatal("hook must report levels")
	}
	for in, want := range map[logrus.Level]logger.Level{
		logrus.TraceLevel: logger.LevelTrace,
		logrus.DebugLevel: logger.LevelDebug,
		logrus.InfoLevel:  logger.LevelInfo,
		logrus.WarnLevel:  logger.LevelWarn,
		logrus.ErrorLevel: logger.LevelError,
		logrus.FatalLevel: logger.LevelFatal,
		logrus.PanicLevel: logger.LevelFatal,
	} {
		if got := mapLevel(in); got != want {
			t.Fatalf("mapLevel(%v)=%v want %v", in, got, want)
		}
	}
}

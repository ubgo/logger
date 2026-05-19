package logrlogger

import (
	"bytes"
	"strings"
	"testing"

	logger "github.com/ubgo/logger"
)

func TestLogrVLevelsNamesAndInit(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	lg := New(core) // logr.New calls LogSink.Init

	lg.V(0).Info("v0-info")  // → Info
	lg.V(1).Info("v1-debug") // → Debug
	lg.V(2).Info("v2-trace") // → Trace
	lg.V(5).Info("v5-trace") // → Trace (default arm)

	// nested WithName builds a dotted logger field
	lg.WithName("a").WithName("b").Info("named")

	out := buf.String()
	for _, w := range []string{
		`"level":"info"`, `v0-info`,
		`"level":"debug"`, `v1-debug`,
		`"level":"trace"`, `v2-trace`,
		`"logger":"a.b"`, `named`,
	} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in: %s", w, out)
		}
	}
}

func TestLogrVToLevelAllArms(t *testing.T) {
	for in, want := range map[int]logger.Level{
		-1: logger.LevelInfo, // <=0
		0:  logger.LevelInfo,
		1:  logger.LevelDebug,
		2:  logger.LevelTrace, // default arm
		9:  logger.LevelTrace,
	} {
		if got := vToLevel(in); got != want {
			t.Fatalf("vToLevel(%d)=%v want %v", in, got, want)
		}
	}
}

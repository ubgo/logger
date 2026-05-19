package zaplogger

import (
	"bytes"
	"strings"
	"testing"

	logger "github.com/ubgo/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapAllLevelsFieldsAndNamed(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	z := New(core, zapcore.DebugLevel).Named("svc").With(zap.String("base", "b"))

	z.Debug("d")
	z.Info("i", zap.String("s", "x"), zap.Int("n", 1), zap.Bool("ok", true),
		zap.Float64("f", 2.5), zap.Int64("i64", 9))
	z.Warn("w")
	z.Error("e")
	_ = z.Sync()

	out := buf.String()
	for _, w := range []string{`"level":"debug"`, `"level":"info"`,
		`"level":"warn"`, `"level":"error"`, `"base":"b"`, `"logger":"svc"`,
		`"s":"x"`, `"n":1`, `"ok":true`, `"f":2.5`, `"i64":9`} {
		if !strings.Contains(out, w) {
			t.Fatalf("missing %q in: %s", w, out)
		}
	}
}

func TestZapCoreCheckEnabledAndLevelMap(t *testing.T) {
	core := logger.New()
	c := NewCore(core, zapcore.InfoLevel)
	if c.Enabled(zapcore.DebugLevel) {
		t.Fatal("Debug must be disabled at Info")
	}
	if !c.Enabled(zapcore.ErrorLevel) {
		t.Fatal("Error must be enabled")
	}
	ce := c.Check(zapcore.Entry{Level: zapcore.InfoLevel}, nil)
	if ce == nil {
		t.Fatal("Check should return a CheckedEntry when enabled")
	}
	for in, want := range map[zapcore.Level]logger.Level{
		zapcore.DebugLevel:  logger.LevelDebug,
		zapcore.InfoLevel:   logger.LevelInfo,
		zapcore.WarnLevel:   logger.LevelWarn,
		zapcore.ErrorLevel:  logger.LevelError,
		zapcore.DPanicLevel: logger.LevelError, // < PanicLevel ⇒ Error
		zapcore.PanicLevel:  logger.LevelFatal,
		zapcore.FatalLevel:  logger.LevelFatal,
	} {
		if got := mapLevel(in); got != want {
			t.Fatalf("mapLevel(%v)=%v want %v", in, got, want)
		}
	}
}

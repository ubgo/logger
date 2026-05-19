package zaplogger

import (
	"bytes"
	"strings"
	"testing"

	logger "github.com/ubgo/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestZapShimRoutesThroughCore(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	z := New(core, zapcore.DebugLevel).With(zap.String("svc", "api"))

	z.Info("zap line", zap.Int("status", 200), zap.Bool("ok", true))

	out := buf.String()
	if !strings.Contains(out, `"msg":"zap line"`) {
		t.Fatalf("message not routed: %s", out)
	}
	if !strings.Contains(out, `"level":"info"`) {
		t.Fatalf("level not mapped: %s", out)
	}
	if !strings.Contains(out, `"svc":"api"`) {
		t.Fatalf("With() fields lost: %s", out)
	}
	if !strings.Contains(out, `"status":200`) || !strings.Contains(out, `"ok":true`) {
		t.Fatalf("call-site fields lost: %s", out)
	}
}

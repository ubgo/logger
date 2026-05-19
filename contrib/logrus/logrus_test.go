package logruslogger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logger "github.com/ubgo/logger"
)

func TestLogrusShimRoutesThroughCore(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	lr := logrus.New()
	Attach(lr, core)

	lr.WithField("user", "ada").WithField("n", 42).Warn("legacy warning")

	out := buf.String()
	if !strings.Contains(out, `"msg":"legacy warning"`) {
		t.Fatalf("message not routed: %s", out)
	}
	if !strings.Contains(out, `"level":"warn"`) {
		t.Fatalf("level not mapped: %s", out)
	}
	if !strings.Contains(out, `"user":"ada"`) || !strings.Contains(out, `"n":42`) {
		t.Fatalf("fields lost: %s", out)
	}
}

package zerologlogger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	logger "github.com/ubgo/logger"
)

func TestZerologSinkForwards(t *testing.T) {
	var buf bytes.Buffer
	zl := zerolog.New(&buf)
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(New(zl, logger.LevelTrace))),
		logger.WithLevel(logger.LevelTrace),
	)
	l.Info("hello", logger.String("user", "ada"), logger.Int("n", 5))
	out := buf.String()
	if !strings.Contains(out, `"message":"hello"`) || !strings.Contains(out, `"user":"ada"`) {
		t.Fatalf("zerolog sink lost data: %s", out)
	}
	if !strings.Contains(out, `"level":"info"`) {
		t.Fatalf("level not mapped: %s", out)
	}
}

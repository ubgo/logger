package phulogger

import (
	"bytes"
	"strings"
	"testing"

	plog "github.com/phuslu/log"
	logger "github.com/ubgo/logger"
)

func TestPhusluSinkForwards(t *testing.T) {
	var buf bytes.Buffer
	pl := &plog.Logger{Level: plog.TraceLevel, Writer: &plog.IOWriter{Writer: &buf}}
	l := logger.New(
		logger.WithTransport(logger.NewSyncTransport(New(pl, logger.LevelTrace))),
		logger.WithLevel(logger.LevelTrace),
	)
	l.Warn("disk slow", logger.String("dev", "sda"), logger.Int("pct", 92))
	out := buf.String()
	if !strings.Contains(out, `"message":"disk slow"`) || !strings.Contains(out, `"dev":"sda"`) {
		t.Fatalf("phuslu sink lost data: %s", out)
	}
	if !strings.Contains(out, `"level":"warn"`) {
		t.Fatalf("level not mapped: %s", out)
	}
}

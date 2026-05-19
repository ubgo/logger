package logrlogger

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	logger "github.com/ubgo/logger"
)

func TestLogrBridge(t *testing.T) {
	var buf bytes.Buffer
	core := logger.New(
		logger.WithSink(logger.NewWriterSink(&buf, logger.NewJSONEncoder(), logger.LevelTrace)),
		logger.WithLevel(logger.LevelTrace),
	)
	lg := New(core).WithName("ctrl").WithValues("reconciler", "pod")

	lg.Info("reconciling", "ns", "default")
	lg.Error(errors.New("conflict"), "update failed")

	out := buf.String()
	if !strings.Contains(out, `"msg":"reconciling"`) || !strings.Contains(out, `"reconciler":"pod"`) {
		t.Fatalf("logr Info lost data: %s", out)
	}
	if !strings.Contains(out, `"logger":"ctrl"`) || !strings.Contains(out, `"ns":"default"`) {
		t.Fatalf("WithName/WithValues lost: %s", out)
	}
	if !strings.Contains(out, `"level":"error"`) || !strings.Contains(out, "conflict") {
		t.Fatalf("logr Error not mapped: %s", out)
	}
}

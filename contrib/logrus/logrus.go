// Package logruslogger is a drop-in migration shim: register the Hook on an
// existing *logrus.Logger and every logrus call flows through ubgo/logger
// with zero call-site edits — the cheap-switch lever.
package logruslogger

import (
	"github.com/sirupsen/logrus"
	logger "github.com/ubgo/logger"
)

// Hook forwards logrus entries into a *logger.Logger.
type Hook struct {
	L      *logger.Logger
	levels []logrus.Level
}

// NewHook builds a Hook for all logrus levels.
func NewHook(l *logger.Logger) *Hook {
	return &Hook{L: l, levels: logrus.AllLevels}
}

// Levels implements logrus.Hook.
func (h *Hook) Levels() []logrus.Level { return h.levels }

func mapLevel(l logrus.Level) logger.Level {
	switch l {
	case logrus.TraceLevel:
		return logger.LevelTrace
	case logrus.DebugLevel:
		return logger.LevelDebug
	case logrus.InfoLevel:
		return logger.LevelInfo
	case logrus.WarnLevel:
		return logger.LevelWarn
	case logrus.ErrorLevel:
		return logger.LevelError
	default: // Fatal, Panic
		return logger.LevelFatal
	}
}

// Fire implements logrus.Hook.
func (h *Hook) Fire(e *logrus.Entry) error {
	fields := make([]logger.Field, 0, len(e.Data))
	for k, v := range e.Data {
		switch x := v.(type) {
		case string:
			fields = append(fields, logger.String(k, x))
		case bool:
			fields = append(fields, logger.Bool(k, x))
		case int:
			fields = append(fields, logger.Int(k, x))
		case int64:
			fields = append(fields, logger.Int(k, x))
		case float64:
			fields = append(fields, logger.Float(k, x))
		case error:
			fields = append(fields, logger.NamedErr(k, x))
		default:
			fields = append(fields, logger.Any(k, v))
		}
	}
	ctx := e.Context
	if ctx == nil {
		h.L.Log(nil, mapLevel(e.Level), e.Message, fields...)
		return nil
	}
	h.L.Log(ctx, mapLevel(e.Level), e.Message, fields...)
	return nil
}

// Attach disables logrus's own output and routes everything through l.
func Attach(dst *logrus.Logger, l *logger.Logger) {
	dst.AddHook(NewHook(l))
	dst.SetOutput(discard{})
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

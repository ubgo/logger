// Package zaplogger is a drop-in migration shim: build a *zap.Logger whose
// zapcore.Core forwards every entry+fields into ubgo/logger, so existing zap
// call sites keep working while the engine becomes ubgo/logger.
package zaplogger

import (
	logger "github.com/ubgo/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type core struct {
	l    *logger.Logger
	lvl  zapcore.Level
	with []logger.Field
}

// NewCore returns a zapcore.Core backed by l.
func NewCore(l *logger.Logger, enab zapcore.Level) zapcore.Core {
	return &core{l: l, lvl: enab}
}

// New returns a ready *zap.Logger backed by l.
func New(l *logger.Logger, enab zapcore.Level) *zap.Logger {
	return zap.New(NewCore(l, enab))
}

func mapLevel(z zapcore.Level) logger.Level {
	switch {
	case z < zapcore.DebugLevel:
		return logger.LevelTrace
	case z < zapcore.InfoLevel:
		return logger.LevelDebug
	case z < zapcore.WarnLevel:
		return logger.LevelInfo
	case z < zapcore.ErrorLevel:
		return logger.LevelWarn
	case z < zapcore.PanicLevel:
		return logger.LevelError
	default:
		return logger.LevelFatal
	}
}

func (c *core) Enabled(l zapcore.Level) bool { return l >= c.lvl }

func (c *core) With(fs []zapcore.Field) zapcore.Core {
	nc := &core{l: c.l, lvl: c.lvl}
	nc.with = append(append([]logger.Field(nil), c.with...), convert(fs)...)
	return nc
}

func (c *core) Check(e zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(e.Level) {
		return ce.AddCore(e, c)
	}
	return ce
}

func (c *core) Write(e zapcore.Entry, fs []zapcore.Field) error {
	fields := append(append([]logger.Field(nil), c.with...), convert(fs)...)
	if e.LoggerName != "" {
		fields = append(fields, logger.String("logger", e.LoggerName))
	}
	c.l.Log(nil, mapLevel(e.Level), e.Message, fields...)
	return nil
}

func (c *core) Sync() error { return c.l.Sync() }

// convert maps zap fields to ubgo fields using zap's own encoder so every
// zap.Field type (Object/Array/etc.) is captured faithfully.
func convert(fs []zapcore.Field) []logger.Field {
	if len(fs) == 0 {
		return nil
	}
	enc := zapcore.NewMapObjectEncoder()
	for _, f := range fs {
		f.AddTo(enc)
	}
	out := make([]logger.Field, 0, len(enc.Fields))
	for k, v := range enc.Fields {
		switch x := v.(type) {
		case string:
			out = append(out, logger.String(k, x))
		case bool:
			out = append(out, logger.Bool(k, x))
		case int64:
			out = append(out, logger.Int(k, x))
		case float64:
			out = append(out, logger.Float(k, x))
		default:
			out = append(out, logger.Any(k, v))
		}
	}
	return out
}

// Package logrlogger bridges the Kubernetes-ecosystem logr.Logger onto
// ubgo/logger: code written against logr (controller-runtime, klog, etc.)
// can run on ubgo's engine unchanged.
package logrlogger

import (
	"github.com/go-logr/logr"
	logger "github.com/ubgo/logger"
)

type sink struct {
	l    *logger.Logger
	name string
	kv   []logger.Field
}

// New returns a logr.Logger backed by l. logr V-levels map to ubgo levels:
// V(0)=Info, V(1)=Debug, V(2+)=Trace; Error() → ubgo Error.
func New(l *logger.Logger) logr.Logger {
	return logr.New(&sink{l: l})
}

func (s *sink) Init(logr.RuntimeInfo) {}

func (s *sink) Enabled(level int) bool {
	return s.l.Enabled(vToLevel(level))
}

func vToLevel(v int) logger.Level {
	switch {
	case v <= 0:
		return logger.LevelInfo
	case v == 1:
		return logger.LevelDebug
	default:
		return logger.LevelTrace
	}
}

func kvToFields(base []logger.Field, kv []any) []logger.Field {
	out := append([]logger.Field(nil), base...)
	for i := 0; i+1 < len(kv); i += 2 {
		key, _ := kv[i].(string)
		out = append(out, logger.Any(key, kv[i+1]))
	}
	return out
}

func (s *sink) Info(level int, msg string, kv ...any) {
	f := kvToFields(s.kv, kv)
	if s.name != "" {
		f = append(f, logger.String("logger", s.name))
	}
	s.l.Log(nil, vToLevel(level), msg, f...)
}

func (s *sink) Error(err error, msg string, kv ...any) {
	f := kvToFields(s.kv, kv)
	f = append(f, logger.NamedErr("error", err))
	if s.name != "" {
		f = append(f, logger.String("logger", s.name))
	}
	s.l.Log(nil, logger.LevelError, msg, f...)
}

func (s *sink) WithValues(kv ...any) logr.LogSink {
	return &sink{l: s.l, name: s.name, kv: kvToFields(s.kv, kv)}
}

func (s *sink) WithName(name string) logr.LogSink {
	n := name
	if s.name != "" {
		n = s.name + "." + name
	}
	return &sink{l: s.l, name: n, kv: s.kv}
}

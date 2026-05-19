// Package zerologlogger is a migration shim: forward records from an existing
// zerolog pipeline into ubgo/logger by using ubgo as a zerolog io.Writer
// sink, or forward ubgo records out to a zerolog.Logger.
package zerologlogger

import (
	"github.com/rs/zerolog"
	logger "github.com/ubgo/logger"
)

// Sink implements logger.Sink by re-emitting each record through a
// zerolog.Logger (use when a team standardizes shipping on zerolog but wants
// ubgo's pipeline/redaction/FingersCrossed in front of it).
type Sink struct {
	zl     zerolog.Logger
	minLvl logger.Level
}

// New wraps a zerolog.Logger as a ubgo sink.
func New(zl zerolog.Logger, minLevel logger.Level) *Sink {
	return &Sink{zl: zl, minLvl: minLevel}
}

func mapLevel(l logger.Level) zerolog.Level {
	switch {
	case l < logger.LevelDebug:
		return zerolog.TraceLevel
	case l < logger.LevelInfo:
		return zerolog.DebugLevel
	case l < logger.LevelWarn:
		return zerolog.InfoLevel
	case l < logger.LevelError:
		return zerolog.WarnLevel
	case l < logger.LevelFatal:
		return zerolog.ErrorLevel
	default:
		return zerolog.FatalLevel
	}
}

// Emit implements logger.Sink.
func (s *Sink) Emit(r *logger.Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	ev := s.zl.WithLevel(mapLevel(r.Level))
	for _, f := range r.Fields {
		ev = ev.Interface(f.Key, f.Value())
	}
	if r.EventName != "" {
		ev = ev.Str("event", r.EventName)
	}
	ev.Msg(r.Message)
	return nil
}

// Sync implements logger.Sink.
func (s *Sink) Sync() error { return nil }

// Close implements logger.Sink.
func (s *Sink) Close() error { return nil }

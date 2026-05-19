// Package phulogger forwards ubgo/logger records into a phuslu/log Logger —
// the migration shim for teams on phuslu that want ubgo's pipeline
// (FingersCrossed, redaction, sampling) in front of phuslu's fast writer.
package phulogger

import (
	plog "github.com/phuslu/log"
	logger "github.com/ubgo/logger"
)

// Sink implements logger.Sink by re-emitting through a *phuslu/log.Logger.
type Sink struct {
	pl     *plog.Logger
	minLvl logger.Level
}

// New wraps a phuslu logger as a ubgo sink.
func New(pl *plog.Logger, minLevel logger.Level) *Sink {
	return &Sink{pl: pl, minLvl: minLevel}
}

func entryFor(pl *plog.Logger, l logger.Level) *plog.Entry {
	switch {
	case l < logger.LevelDebug:
		return pl.Trace()
	case l < logger.LevelInfo:
		return pl.Debug()
	case l < logger.LevelWarn:
		return pl.Info()
	case l < logger.LevelError:
		return pl.Warn()
	case l < logger.LevelFatal:
		return pl.Error()
	default:
		return pl.Fatal()
	}
}

// Emit implements logger.Sink.
func (s *Sink) Emit(r *logger.Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	e := entryFor(s.pl, r.Level)
	for _, f := range r.Fields {
		e = e.Any(f.Key, f.Value())
	}
	if r.EventName != "" {
		e = e.Str("event", r.EventName)
	}
	e.Msg(r.Message)
	return nil
}

// Sync implements logger.Sink.
func (s *Sink) Sync() error { return nil }

// Close implements logger.Sink.
func (s *Sink) Close() error { return nil }

// Package sentrylogger ships error-level ubgo/logger records to Sentry as
// events (with the record fields as Sentry extra/tags). Only WARN+ is sent by
// default — Sentry is for problems, not chatter.
package sentrylogger

import (
	"github.com/getsentry/sentry-go"
	logger "github.com/ubgo/logger"
)

// Sink implements logger.Sink by capturing records as Sentry events.
type Sink struct {
	hub    *sentry.Hub
	minLvl logger.Level
}

// New uses the current Sentry hub (sentry.Init must have been called).
// minLevel defaults sensibly to Warn if you pass a lower one.
func New(minLevel logger.Level) *Sink {
	if minLevel < logger.LevelWarn {
		minLevel = logger.LevelWarn
	}
	return &Sink{hub: sentry.CurrentHub(), minLvl: minLevel}
}

// NewWithHub wires an explicit hub (per-tenant isolation, tests).
func NewWithHub(h *sentry.Hub, minLevel logger.Level) *Sink {
	return &Sink{hub: h, minLvl: minLevel}
}

func level(l logger.Level) sentry.Level {
	switch {
	case l < logger.LevelWarn:
		return sentry.LevelInfo
	case l < logger.LevelError:
		return sentry.LevelWarning
	case l < logger.LevelFatal:
		return sentry.LevelError
	default:
		return sentry.LevelFatal
	}
}

// Emit implements logger.Sink.
func (s *Sink) Emit(r *logger.Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	ev := sentry.NewEvent()
	ev.Level = level(r.Level)
	ev.Message = r.Message
	if ev.Tags == nil {
		ev.Tags = map[string]string{}
	}
	if r.EventName != "" {
		ev.Message = r.EventName
		ev.Tags["event"] = r.EventName
	}
	fields := sentry.Context{}
	for _, f := range r.Fields {
		if err, ok := f.Value().(error); ok && err != nil {
			ev.Exception = append(ev.Exception, sentry.Exception{
				Type: f.Key, Value: err.Error(),
			})
			continue
		}
		fields[f.Key] = f.Value()
	}
	if len(fields) > 0 {
		if ev.Contexts == nil {
			ev.Contexts = map[string]sentry.Context{}
		}
		ev.Contexts["fields"] = fields
	}
	s.hub.CaptureEvent(ev)
	return nil
}

// Sync flushes buffered Sentry events.
func (s *Sink) Sync() error {
	s.hub.Flush(0)
	return nil
}

// Close flushes and is final.
func (s *Sink) Close() error {
	s.hub.Flush(0)
	return nil
}

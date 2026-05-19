//go:build !windows && !plan9

package logger

import (
	"log/syslog"
)

// SyslogSink writes to a local or remote syslog daemon (RFC 3164/5424 via the
// stdlib). Severity is mapped from the record level so syslog filtering works.
// Unix-only (build-tagged); on Windows use NetSink or the event-log path.
type SyslogSink struct {
	w      *syslog.Writer
	enc    Encoder
	minLvl Level
}

// NewSyslogSink dials syslog. network "" + addr "" uses the local daemon;
// otherwise e.g. ("tcp","logs.example:514"). tag is the syslog program tag.
func NewSyslogSink(network, addr, tag string, enc Encoder, minLevel Level) (*SyslogSink, error) {
	w, err := syslog.Dial(network, addr, syslog.LOG_INFO|syslog.LOG_USER, tag)
	if err != nil {
		return nil, err
	}
	return &SyslogSink{w: w, enc: enc, minLvl: minLevel}, nil
}

// Emit implements Sink, routing to the matching syslog severity.
func (s *SyslogSink) Emit(r *Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	buf := getBuffer()
	s.enc.Encode(buf, r)
	msg := string(buf.b)
	putBuffer(buf)

	switch {
	case r.Level < LevelDebug:
		return s.w.Debug(msg)
	case r.Level < LevelInfo:
		return s.w.Debug(msg)
	case r.Level < LevelWarn:
		return s.w.Info(msg)
	case r.Level < LevelError:
		return s.w.Warning(msg)
	case r.Level < LevelFatal:
		return s.w.Err(msg)
	default:
		return s.w.Crit(msg)
	}
}

// Sync is a no-op (syslog writer is unbuffered).
func (s *SyslogSink) Sync() error { return nil }

// Close closes the syslog connection.
func (s *SyslogSink) Close() error { return s.w.Close() }

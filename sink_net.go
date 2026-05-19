package logger

import (
	"crypto/tls"
	"net"
	"sync"
	"time"
)

// NetSink streams encoded records over TCP/UDP/TLS. It dials lazily and
// transparently re-dials on a write error so a transient network blip drops
// at most the in-flight line (counted) rather than wedging the app.
type NetSink struct {
	network string // "tcp" | "udp"
	addr    string
	enc     Encoder
	minLvl  Level
	tlsCfg  *tls.Config

	mu      sync.Mutex
	conn    net.Conn
	dropped uint64
}

// NewTCPSink streams to a TCP endpoint.
func NewTCPSink(addr string, enc Encoder, minLevel Level) *NetSink {
	return &NetSink{network: "tcp", addr: addr, enc: enc, minLvl: minLevel}
}

// NewUDPSink streams to a UDP endpoint (fire-and-forget).
func NewUDPSink(addr string, enc Encoder, minLevel Level) *NetSink {
	return &NetSink{network: "udp", addr: addr, enc: enc, minLvl: minLevel}
}

// NewTLSSink streams over TLS-wrapped TCP.
func NewTLSSink(addr string, cfg *tls.Config, enc Encoder, minLevel Level) *NetSink {
	return &NetSink{network: "tcp", addr: addr, enc: enc, minLvl: minLevel, tlsCfg: cfg}
}

func (s *NetSink) dialLocked() error {
	d := net.Dialer{Timeout: 5 * time.Second}
	var c net.Conn
	var err error
	if s.tlsCfg != nil {
		c, err = tls.DialWithDialer(&d, s.network, s.addr, s.tlsCfg)
	} else {
		c, err = d.Dial(s.network, s.addr)
	}
	if err != nil {
		return err
	}
	s.conn = c
	return nil
}

// Emit implements Sink.
func (s *NetSink) Emit(r *Record) error {
	if r.Level < s.minLvl {
		return nil
	}
	buf := getBuffer()
	s.enc.Encode(buf, r)

	s.mu.Lock()
	defer func() { s.mu.Unlock(); putBuffer(buf) }()

	if s.conn == nil {
		if err := s.dialLocked(); err != nil {
			s.dropped++
			return err
		}
	}
	if _, err := s.conn.Write(buf.b); err != nil {
		_ = s.conn.Close()
		s.conn = nil
		if err2 := s.dialLocked(); err2 != nil { // one re-dial attempt
			s.dropped++
			return err2
		}
		if _, err2 := s.conn.Write(buf.b); err2 != nil {
			s.dropped++
			return err2
		}
	}
	return nil
}

// Dropped reports records lost to network failures (never silent).
func (s *NetSink) Dropped() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Sync is a no-op (sockets are unbuffered here).
func (s *NetSink) Sync() error { return nil }

// Close closes the connection.
func (s *NetSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

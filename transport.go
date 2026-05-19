package logger

import (
	"sync"
	"sync/atomic"
)

// OverflowPolicy is the explicit, named backpressure choice for async
// transports (spdlog/Logback model). The library never silently guesses.
type OverflowPolicy uint8

const (
	// Block: the caller waits until the queue has room (lossless, adds latency).
	Block OverflowPolicy = iota
	// DropNewest: discard the incoming record (protects latency, loses newest).
	DropNewest
	// DropOldest: evict the oldest queued record to make room.
	DropOldest
)

// Transport moves a finished Record from the log call to the sink. It is the
// pluggable async engine seam: Sync (inline), Channel (bounded chan + worker),
// Ring (bounded ring + worker) are all interchangeable behind this interface.
type Transport interface {
	// Dispatch delivers r to the sink. Implementations that defer delivery
	// MUST Clone r — the caller's *Record is pooled and reused on return.
	Dispatch(r *Record)
	// Dropped returns the total records lost to the overflow policy (never
	// silent: callers/self-metrics surface this).
	Dropped() uint64
	Sync() error
	Close() error
}

// SyncTransport delivers inline on the calling goroutine. Simplest, lossless,
// correct; the default until the caller opts into async.
type SyncTransport struct{ sink Sink }

// NewSyncTransport wraps a sink for inline delivery.
func NewSyncTransport(s Sink) *SyncTransport { return &SyncTransport{sink: s} }

func (t *SyncTransport) Dispatch(r *Record) { _ = t.sink.Emit(r) }
func (t *SyncTransport) Dropped() uint64    { return 0 }
func (t *SyncTransport) Sync() error        { return t.sink.Sync() }
func (t *SyncTransport) Close() error       { return t.sink.Close() }

// ChannelTransport: bounded buffered channel feeding one worker goroutine.
// The pragmatic default async engine — correct, simple, good for ~99%.
type ChannelTransport struct {
	sink    Sink
	ch      chan *Record
	policy  OverflowPolicy
	dropped atomic.Uint64
	done    chan struct{}
	once    sync.Once
}

// NewChannelTransport starts a worker draining a queue of the given capacity.
func NewChannelTransport(s Sink, capacity int, policy OverflowPolicy) *ChannelTransport {
	if capacity < 1 {
		capacity = 1024
	}
	t := &ChannelTransport{
		sink:   s,
		ch:     make(chan *Record, capacity),
		policy: policy,
		done:   make(chan struct{}),
	}
	go t.run()
	return t
}

func (t *ChannelTransport) run() {
	for r := range t.ch {
		_ = t.sink.Emit(r)
		r.release()
	}
	close(t.done)
}

// Dispatch implements Transport.
func (t *ChannelTransport) Dispatch(r *Record) {
	c := r.Clone() // record is pooled; copy before crossing the goroutine
	switch t.policy {
	case Block:
		t.ch <- c
	case DropNewest:
		select {
		case t.ch <- c:
		default:
			t.dropped.Add(1)
		}
	case DropOldest:
		for {
			select {
			case t.ch <- c:
				return
			default:
				select {
				case <-t.ch: // evict oldest, retry
					t.dropped.Add(1)
				default:
				}
			}
		}
	}
}

func (t *ChannelTransport) Dropped() uint64 { return t.dropped.Load() }
func (t *ChannelTransport) Sync() error     { return t.sink.Sync() }

// Close drains the queue, stops the worker, and closes the sink.
func (t *ChannelTransport) Close() error {
	t.once.Do(func() {
		close(t.ch)
		<-t.done
	})
	_ = t.sink.Sync()
	return t.sink.Close()
}

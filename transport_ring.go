package logger

import (
	"sync"
	"sync/atomic"
)

// RingTransport is a bounded ring buffer drained by one worker goroutine. It
// is the high-throughput async engine: a fixed pre-allocated slice avoids the
// per-record channel-send overhead and makes DropOldest O(1).
//
// NOTE: this is a correct mutex+cond bounded ring, not yet a lock-free
// Disruptor. The Transport interface is the contract; a lock-free MPSC variant
// can replace this implementation later without touching callers. We ship the
// correct version rather than a subtly-broken lock-free claim.
type RingTransport struct {
	sink     Sink
	mu       sync.Mutex
	notEmpty *sync.Cond
	notFull  *sync.Cond
	buf      []*Record
	head     int
	tail     int
	count    int
	policy   OverflowPolicy
	closed   bool
	dropped  atomic.Uint64
	done     chan struct{}
}

// NewRingTransport starts a worker draining a ring of the given capacity.
func NewRingTransport(s Sink, capacity int, policy OverflowPolicy) *RingTransport {
	if capacity < 2 {
		capacity = 1024
	}
	t := &RingTransport{
		sink:   s,
		buf:    make([]*Record, capacity),
		policy: policy,
		done:   make(chan struct{}),
	}
	t.notEmpty = sync.NewCond(&t.mu)
	t.notFull = sync.NewCond(&t.mu)
	go t.run()
	return t
}

func (t *RingTransport) run() {
	for {
		t.mu.Lock()
		for t.count == 0 && !t.closed {
			t.notEmpty.Wait()
		}
		if t.count == 0 && t.closed {
			t.mu.Unlock()
			close(t.done)
			return
		}
		r := t.buf[t.head]
		t.buf[t.head] = nil
		t.head = (t.head + 1) % len(t.buf)
		t.count--
		t.notFull.Signal()
		t.mu.Unlock()

		_ = t.sink.Emit(r)
		r.release()
	}
}

// Dispatch implements Transport.
func (t *RingTransport) Dispatch(r *Record) {
	c := r.Clone()
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	if t.count == len(t.buf) {
		switch t.policy {
		case Block:
			for t.count == len(t.buf) && !t.closed {
				t.notFull.Wait()
			}
			if t.closed {
				t.mu.Unlock()
				return
			}
		case DropNewest:
			t.mu.Unlock()
			t.dropped.Add(1)
			return
		case DropOldest:
			t.buf[t.head] = nil
			t.head = (t.head + 1) % len(t.buf)
			t.count--
			t.dropped.Add(1)
		}
	}
	t.buf[t.tail] = c
	t.tail = (t.tail + 1) % len(t.buf)
	t.count++
	t.notEmpty.Signal()
	t.mu.Unlock()
}

func (t *RingTransport) Dropped() uint64 { return t.dropped.Load() }
func (t *RingTransport) Sync() error     { return t.sink.Sync() }

// Close drains remaining records, stops the worker, closes the sink.
func (t *RingTransport) Close() error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil
	}
	t.closed = true
	t.notEmpty.Broadcast()
	t.notFull.Broadcast()
	t.mu.Unlock()
	<-t.done
	_ = t.sink.Sync()
	return t.sink.Close()
}

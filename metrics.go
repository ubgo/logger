package logger

import (
	"net/http"
	"strconv"
	"sync/atomic"
)

// Metrics is the logger's self-observability: how many records it emitted,
// dropped (sampling/dedup/pipeline), and how many sink errors occurred — so
// you can monitor your monitoring. Cheap atomics; always on.
type Metrics struct {
	emitted    atomic.Uint64
	dropped    atomic.Uint64
	sinkErrors atomic.Uint64
	byLevel    [25]atomic.Uint64 // index = OTEL SeverityNumber (0..24)
}

// Snapshot is an immutable read of the counters.
type Snapshot struct {
	Emitted    uint64            `json:"emitted"`
	Dropped    uint64            `json:"dropped"`
	SinkErrors uint64            `json:"sink_errors"`
	ByLevel    map[string]uint64 `json:"by_level"`
}

func (m *Metrics) incEmitted(l Level) {
	m.emitted.Add(1)
	if l >= 0 && int(l) < len(m.byLevel) {
		m.byLevel[l].Add(1)
	}
}
func (m *Metrics) incDropped() { m.dropped.Add(1) }

// IncSinkError records a sink failure. Wire it through Fanout.OnError:
//
//	fan.OnError = func(Sink, error) { log.Metrics().IncSinkError() }
func (m *Metrics) IncSinkError() { m.sinkErrors.Add(1) }

// Snapshot reads the current counters.
func (m *Metrics) Snapshot() Snapshot {
	s := Snapshot{
		Emitted:    m.emitted.Load(),
		Dropped:    m.dropped.Load(),
		SinkErrors: m.sinkErrors.Load(),
		ByLevel:    map[string]uint64{},
	}
	for i := 1; i < len(m.byLevel); i++ {
		if v := m.byLevel[i].Load(); v > 0 {
			s.ByLevel[Level(i).String()] += v
		}
	}
	return s
}

// ServeHTTP exposes the snapshot as JSON for an admin/metrics endpoint.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	s := m.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	b := []byte(`{"emitted":`)
	b = strconv.AppendUint(b, s.Emitted, 10)
	b = append(b, `,"dropped":`...)
	b = strconv.AppendUint(b, s.Dropped, 10)
	b = append(b, `,"sink_errors":`...)
	b = strconv.AppendUint(b, s.SinkErrors, 10)
	b = append(b, `,"by_level":{`...)
	first := true
	for k, v := range s.ByLevel {
		if !first {
			b = append(b, ',')
		}
		b = append(b, '"')
		b = append(b, k...)
		b = append(b, '"', ':')
		b = strconv.AppendUint(b, v, 10)
		first = false
	}
	b = append(b, '}', '}', '\n')
	_, _ = w.Write(b)
}

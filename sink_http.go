package logger

import (
	"bytes"
	"context"
	"net/http"
	"sync"
	"time"
)

// HTTPBatchSink is the shared primitive behind the cloud sinks: it buffers
// encoded records and flushes them as one HTTP POST when the batch hits
// MaxBatch records or FlushInterval elapses. One bad flush never blocks the
// app (delivery happens on a background goroutine); failures increment a
// dropped counter rather than panicking.
type HTTPBatchSink struct {
	URL      string
	Headers  map[string]string
	MaxBatch int
	Flush    time.Duration
	MinLvl   Level
	Client   *http.Client
	// Build turns a batch of records into a request body + content-type.
	Build func(recs []*Record) (body []byte, contentType string)

	mu      sync.Mutex
	buf     []*Record
	timer   *time.Timer
	dropped uint64
	closed  bool
}

// NewHTTPBatchSink constructs a batch sink. Build is required.
func NewHTTPBatchSink(url string, minLevel Level, build func([]*Record) ([]byte, string)) *HTTPBatchSink {
	return &HTTPBatchSink{
		URL: url, MinLvl: minLevel, MaxBatch: 256, Flush: 5 * time.Second,
		Client: &http.Client{Timeout: 10 * time.Second}, Build: build,
	}
}

// WithHeader adds a request header (e.g. auth) and returns the sink.
func (h *HTTPBatchSink) WithHeader(k, v string) *HTTPBatchSink {
	if h.Headers == nil {
		h.Headers = map[string]string{}
	}
	h.Headers[k] = v
	return h
}

// Emit implements Sink (buffers; flushes on size/timer).
func (h *HTTPBatchSink) Emit(r *Record) error {
	if r.Level < h.MinLvl {
		return nil
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.buf = append(h.buf, r.Clone())
	if len(h.buf) >= h.MaxBatch {
		batch := h.takeLocked()
		h.mu.Unlock()
		go h.send(batch)
		return nil
	}
	if h.timer == nil {
		h.timer = time.AfterFunc(h.Flush, h.flushTimer)
	}
	h.mu.Unlock()
	return nil
}

func (h *HTTPBatchSink) takeLocked() []*Record {
	b := h.buf
	h.buf = nil
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	return b
}

func (h *HTTPBatchSink) flushTimer() {
	h.mu.Lock()
	batch := h.takeLocked()
	h.mu.Unlock()
	if len(batch) > 0 {
		h.send(batch)
	}
}

func (h *HTTPBatchSink) send(recs []*Record) {
	body, ct := h.Build(recs)
	req, err := http.NewRequestWithContext(context.Background(),
		http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		h.addDropped(len(recs))
		return
	}
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	for k, v := range h.Headers {
		req.Header.Set(k, v)
	}
	resp, err := h.Client.Do(req)
	if err != nil {
		h.addDropped(len(recs))
		return
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		h.addDropped(len(recs))
	}
}

func (h *HTTPBatchSink) addDropped(n int) {
	h.mu.Lock()
	h.dropped += uint64(n)
	h.mu.Unlock()
}

// Dropped reports records lost to delivery failures (never silent).
func (h *HTTPBatchSink) Dropped() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.dropped
}

// Sync flushes the pending batch synchronously.
func (h *HTTPBatchSink) Sync() error {
	h.mu.Lock()
	batch := h.takeLocked()
	h.mu.Unlock()
	if len(batch) > 0 {
		h.send(batch)
	}
	return nil
}

// Close flushes and stops accepting.
func (h *HTTPBatchSink) Close() error {
	h.mu.Lock()
	h.closed = true
	batch := h.takeLocked()
	h.mu.Unlock()
	if len(batch) > 0 {
		h.send(batch)
	}
	return nil
}

// --- Concrete cloud sinks (thin Build configs, zero extra deps) ------------

func jsonLine(r *Record) []byte {
	b := getBuffer()
	NewJSONEncoder().Encode(b, r)
	out := append([]byte(nil), b.b...)
	putBuffer(b)
	return out
}

// NewLokiSink pushes to Grafana Loki's HTTP push API. labels become the
// stream labels; the log line is the JSON-encoded record.
func NewLokiSink(pushURL string, labels map[string]string, minLevel Level) *HTTPBatchSink {
	return NewHTTPBatchSink(pushURL, minLevel, func(recs []*Record) ([]byte, string) {
		var b bytes.Buffer
		b.WriteString(`{"streams":[{"stream":{`)
		first := true
		for k, v := range labels {
			if !first {
				b.WriteByte(',')
			}
			b.WriteByte('"')
			b.WriteString(k)
			b.WriteString(`":"`)
			b.WriteString(v)
			b.WriteByte('"')
			first = false
		}
		b.WriteString(`},"values":[`)
		for i, r := range recs {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(`["`)
			b.WriteString(itoa64(r.Time.UnixNano()))
			b.WriteString(`",`)
			line := jsonLine(r)
			for len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			bq := getBuffer()
			bq.writeJSONString(string(line))
			b.Write(bq.b)
			putBuffer(bq)
			b.WriteByte(']')
		}
		b.WriteString(`]}]}`)
		return b.Bytes(), "application/json"
	})
}

// NewDatadogSink ships to the Datadog logs intake (ndjson array). apiKey is
// sent via the DD-API-KEY header.
func NewDatadogSink(intakeURL, apiKey string, minLevel Level) *HTTPBatchSink {
	s := NewHTTPBatchSink(intakeURL, minLevel, func(recs []*Record) ([]byte, string) {
		var b bytes.Buffer
		b.WriteByte('[')
		for i, r := range recs {
			if i > 0 {
				b.WriteByte(',')
			}
			line := jsonLine(r)
			for len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			b.Write(line)
		}
		b.WriteByte(']')
		return b.Bytes(), "application/json"
	})
	return s.WithHeader("DD-API-KEY", apiKey)
}

// NewElasticsearchSink uses the ES _bulk API (action + source per record).
func NewElasticsearchSink(bulkURL, index string, minLevel Level) *HTTPBatchSink {
	return NewHTTPBatchSink(bulkURL, minLevel, func(recs []*Record) ([]byte, string) {
		var b bytes.Buffer
		meta := `{"index":{"_index":"` + index + `"}}` + "\n"
		for _, r := range recs {
			b.WriteString(meta)
			line := jsonLine(r)
			for len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			b.Write(line)
			b.WriteByte('\n')
		}
		return b.Bytes(), "application/x-ndjson"
	})
}

func itoa64(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var d [20]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		d[i] = '-'
	}
	return string(d[i:])
}

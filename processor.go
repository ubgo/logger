package logger

import (
	"context"
	"errors"
	"sync/atomic"
)

// ErrDrop is the sentinel a Processor returns to drop the record silently
// (this is how sampling/rate-limiting is expressed — one concept, not a
// separate subsystem). Any other non-nil error is a processing failure.
var ErrDrop = errors.New("logger: record dropped")

// Processor is the single extension seam (structlog model). Enrichment,
// redaction, sampling, dedup are all Processors composed into a pipeline.
// Mutate r in place; return ErrDrop to discard; return nil to continue.
type Processor interface {
	Process(ctx context.Context, r *Record) error
}

// ProcessorFunc adapts a function to Processor.
type ProcessorFunc func(ctx context.Context, r *Record) error

func (f ProcessorFunc) Process(ctx context.Context, r *Record) error { return f(ctx, r) }

// runPipeline applies processors in order. A drop short-circuits.
func runPipeline(ps []Processor, ctx context.Context, r *Record) error {
	for _, p := range ps {
		if err := p.Process(ctx, r); err != nil {
			return err
		}
	}
	return nil
}

// --- Built-in processors ---------------------------------------------------

// RedactProcessor masks the values of fields whose key is in Deny. This is the
// minimal v1 redaction stage; the compiled path-DSL (*.password) lands later
// behind this same interface.
type RedactProcessor struct {
	Deny   map[string]struct{}
	Censor string // replacement, default "[REDACTED]"
}

// NewRedactProcessor builds a key-denylist redactor.
func NewRedactProcessor(keys ...string) *RedactProcessor {
	d := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		d[k] = struct{}{}
	}
	return &RedactProcessor{Deny: d, Censor: "[REDACTED]"}
}

// Process implements Processor.
func (p *RedactProcessor) Process(_ context.Context, r *Record) error {
	c := p.Censor
	if c == "" {
		c = "[REDACTED]"
	}
	for i := range r.Fields {
		if _, deny := p.Deny[r.Fields[i].Key]; deny {
			r.Fields[i] = String(r.Fields[i].Key, c)
		}
	}
	return nil
}

// SampleProcessor keeps the first N records per reset window then 1 in M, and
// NEVER samples records at or above NeverBelow (default LevelError) — you must
// not drop errors. Drops are counted (no silent loss).
type SampleProcessor struct {
	First      uint64
	Thereafter uint64 // keep 1 in M after First; 0 disables sampling
	NeverBelow Level
	n          atomic.Uint64
	dropped    atomic.Uint64
}

// NewSampleProcessor builds a leveled sampler.
func NewSampleProcessor(first, thereafter uint64) *SampleProcessor {
	return &SampleProcessor{First: first, Thereafter: thereafter, NeverBelow: LevelError}
}

// Dropped reports how many records the sampler discarded.
func (p *SampleProcessor) Dropped() uint64 { return p.dropped.Load() }

// Process implements Processor.
func (p *SampleProcessor) Process(_ context.Context, r *Record) error {
	if r.Level >= p.NeverBelow || p.Thereafter == 0 {
		return nil
	}
	n := p.n.Add(1)
	if n <= p.First {
		return nil
	}
	if (n-p.First)%p.Thereafter == 0 {
		return nil
	}
	p.dropped.Add(1)
	return ErrDrop
}

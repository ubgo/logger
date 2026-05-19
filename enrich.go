package logger

import "context"

// --- request-scoped bound fields (MDC-equivalent) --------------------------

type ctxFieldsKey struct{}

// ContextWith returns a context carrying fields that EnrichProcessor will
// merge into every record logged with that context — the idiomatic-Go
// replacement for thread-local MDC. Repeated calls accumulate.
func ContextWith(ctx context.Context, fields ...Field) context.Context {
	prev, _ := ctx.Value(ctxFieldsKey{}).([]Field)
	merged := append(append(make([]Field, 0, len(prev)+len(fields)), prev...), fields...)
	return context.WithValue(ctx, ctxFieldsKey{}, merged)
}

// TraceExtractor pulls correlation IDs out of a context. The core stays
// zero-dependency: contrib/otel registers a real OTEL/W3C extractor; tests or
// custom propagation can register their own.
type TraceExtractor func(ctx context.Context) (traceID, spanID string, ok bool)

// EnrichProcessor injects ctx-bound fields + trace correlation into every
// record. Place it early in the pipeline so redaction/sampling see the
// enriched record.
type EnrichProcessor struct {
	Extractors []TraceExtractor
	TraceKey   string // default "trace_id"
	SpanKey    string // default "span_id"
}

// NewEnrichProcessor builds an enricher with optional trace extractors.
func NewEnrichProcessor(ex ...TraceExtractor) *EnrichProcessor {
	return &EnrichProcessor{Extractors: ex, TraceKey: "trace_id", SpanKey: "span_id"}
}

// Process implements Processor.
func (e *EnrichProcessor) Process(ctx context.Context, r *Record) error {
	if ctx == nil {
		ctx = r.Ctx
	}
	if ctx == nil {
		return nil
	}
	if bound, ok := ctx.Value(ctxFieldsKey{}).([]Field); ok && len(bound) > 0 {
		r.Fields = append(r.Fields, bound...)
	}
	tk, sk := e.TraceKey, e.SpanKey
	if tk == "" {
		tk = "trace_id"
	}
	if sk == "" {
		sk = "span_id"
	}
	for _, ex := range e.Extractors {
		if tid, sid, ok := ex(ctx); ok {
			if tid != "" {
				r.Fields = append(r.Fields, String(tk, tid))
			}
			if sid != "" {
				r.Fields = append(r.Fields, String(sk, sid))
			}
			break
		}
	}
	return nil
}

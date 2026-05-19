# Processors & the pipeline

A **Processor** is the single extension point. Everything that transforms, filters, enriches, or samples a record is a processor:

```go
type Processor interface {
	Process(ctx context.Context, r *Record) error
}
```

Compose them with `logger.WithProcessors(p1, p2, …)`. They run in order. Return `logger.ErrDrop` to drop the record (this is how sampling/dedup work); any other error is a counted failure.

## Built-in processors

### EnrichProcessor — context-bound fields + trace correlation

```go
log := logger.New(logger.WithProcessors(logger.NewEnrichProcessor()))

ctx = logger.ContextWith(ctx, logger.String("tenant", "acme"))
log.InfoContext(ctx, "work") // tenant added automatically (MDC-equivalent)
```

Add trace correlation by passing an extractor (the OTEL one lives in `contrib/otel`):

```go
logger.NewEnrichProcessor(otellogger.TraceExtractor())
// → trace_id / span_id stamped on every record from the active span
```

You can write your own `TraceExtractor` (e.g. for a custom propagation header) — the core stays dependency-free.

### PathRedactor — compiled secret/PII redaction

Declarative, dotted-path patterns compiled once:

```go
logger.NewPathRedactor(logger.Mask, "[REDACTED]",
	"password",                  // exact key
	"*.password",                // one wildcard segment
	"req.headers.authorization", // exact path (slog groups → dotted keys)
	"user.**",                   // everything under user
)
```

| Strategy | Effect |
|---|---|
| `logger.Mask` | replace value with the censor string |
| `logger.Hash` | replace with `sha256:` prefix — keeps correlation, hides value |
| `logger.Drop` | remove the field entirely |

Compose multiple redactors for mixed policies. Redaction runs **before any sink**, so secrets never touch disk/stdout/network.

### SampleProcessor — keep first N, then 1/M

```go
logger.NewSampleProcessor(100, 100) // first 100, then 1-in-100
```

Records at/above `NeverBelow` (default `LevelError`) are **never** sampled. Dropped count is exposed via `.Dropped()` and the logger's `Metrics()`.

### DedupProcessor — collapse repeats

```go
logger.NewDedupProcessor(5 * time.Second)
```

Identical `level+message` within the window are dropped; the next survivor is annotated with `deduped_count`. One hot error can't bury everything else — without silently hiding that it happened.

## FingersCrossed — debug-on-error buffering

Not a processor (it needs to control emission), but the same idea: a **sink decorator**. Successful scopes emit nothing below the activation level; the first error flushes the whole buffered trail.

```go
fc := logger.NewFingersCrossed(realSink)         // activation defaults to LevelError
log := logger.New(logger.WithTransport(logger.NewSyncTransport(fc)),
	logger.WithLevel(logger.LevelTrace))

ctx := logger.FCScope(r.Context())               // one buffer per request
log.DebugContext(ctx, "step 1")                  // buffered
log.DebugContext(ctx, "step 2")                  // buffered
log.ErrorContext(ctx, "boom")                    // flushes step 1 + 2 + boom
// success path → nothing below LevelError is ever written
```

Without `FCScope` it falls back to a single process-global ring (still correct, less precise).

## Writing a custom processor

```go
hostname, _ := os.Hostname()
addHost := logger.ProcessorFunc(func(_ context.Context, r *logger.Record) error {
	r.AddField(logger.String("host", hostname))
	return nil
})

log := logger.New(logger.WithProcessors(addHost))
```

Order matters: put enrichment before redaction before sampling, so each stage sees the final record.

See also: [architecture](./architecture.md), [sinks & transports](./sinks.md).

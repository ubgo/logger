# Architecture & design

ubgo/logger has exactly **five concepts**. Learn these and you know the whole library.

```
log.Info(...)                     your call site
   │
   ▼
[ Logger ]  ── builds a pooled ─▶  [ Record ]
   │                                  │
   ▼                                  ▼
[ Processor pipeline ]  enrich · redact · sample · dedup · (ErrDrop = drop)
   │
   ▼
[ Transport ]  Sync | Channel | Disruptor   (+ explicit OverflowPolicy)
   │
   ▼
[ Sink / Fanout ]  console · file · network · cloud   (per-sink level + encoder)
```

## 1. Logger

What you call. Immutable after construction. `With(fields...)` returns a child that prepends bound fields; the parent is unaffected. Child loggers share the parent's pipeline, transport, and metrics.

## 2. Record

One log event: time, level, message (or `EventName`), fields, ctx, caller PC. Records are **pooled** (`sync.Pool`) — never retain a `*Record` past the call; async transports `Clone()` before crossing a goroutine.

## 3. Processor — the single extension seam

```go
type Processor interface {
	Process(ctx context.Context, r *Record) error
}
```

This is the whole extension model (borrowed from Python's structlog). Enrichment, redaction, sampling, dedup, FingersCrossed activation — all processors. Return:

- `nil` → continue
- `logger.ErrDrop` → drop the record (this is how **sampling** is expressed — one concept, not a subsystem)
- any other error → processing failure (counted)

Processors run in order; mutate the record in place.

## 4. Transport — the async engine seam

```go
type Transport interface {
	Dispatch(r *Record)
	Dropped() uint64
	Sync() error
	Close() error
}
```

Three interchangeable implementations:

| Transport | Use when |
|---|---|
| `SyncTransport` | default — inline, lossless, simplest |
| `ChannelTransport` | async, bounded channel + worker — good for ~99% |
| `DisruptorTransport` | lock-free Vyukov MPMC ring — max throughput under heavy concurrency |

Every async transport takes an explicit **`OverflowPolicy`**: `Block` (lossless, adds latency), `DropNewest`, or `DropOldest`. Dropped records are **counted** (`.Dropped()`) — never silently lost.

## 5. Sink — the destination

```go
type Sink interface {
	Emit(r *Record) error
	Sync() error
	Close() error
}
```

Each sink owns its own level and encoder. `Fanout` broadcasts to many sinks with **failure isolation**: one sink erroring (disk full, network down) never blocks or kills the others.

## Design principles

- **slog-native.** The logger *is* a correct `slog.Handler` (passes `testing/slogtest`, including the WithGroup/WithAttrs ordering most handlers get wrong). The entire slog ecosystem composes on top. No concrete backend type ever leaks into a public signature.
- **Zero-allocation typed path.** Scalar fields are stored unboxed; only `Any` escapes to reflection. Enforced by a CI allocation-regression gate.
- **Honest accounting.** Sampling/backpressure drops are always counted and exposed via `Metrics()`. No silent loss.
- **Redact at source.** PII/secret redaction runs in-process before bytes reach any sink — the only place structure and raw values coexist.
- **Zero core dependencies.** Heavy integrations (zap, OTEL, Sentry, …) live in opt-in `contrib/*` submodules.

## Level model = OTEL SeverityNumber

`Level` is the OpenTelemetry `SeverityNumber` (1–24) directly, so severity survives the OTEL bridge with no lossy remap. `>= 17` (ERROR) marks an erroneous record per the OTEL data model.

See also: [processors](./processors.md), [sinks & transports](./sinks.md).

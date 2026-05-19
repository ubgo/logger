# ubgo/logger — the last Go logging library you'll need

[![CI](https://github.com/ubgo/logger/actions/workflows/ci.yml/badge.svg)](https://github.com/ubgo/logger/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ubgo/logger.svg)](https://pkg.go.dev/github.com/ubgo/logger)
[![Go Report Card](https://goreportcard.com/badge/github.com/ubgo/logger)](https://goreportcard.com/report/github.com/ubgo/logger)
[![Go 1.24+](https://img.shields.io/badge/go-1.24%2B-00ADD8.svg)](https://go.dev/dl/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

**ubgo/logger is a pluggable, adapter-based, `log/slog`-native structured logging library for Go** — zero-allocation on the hot path, batteries included, and a drop-in upgrade path from `zap`, `zerolog`, `logrus`, `slog`, and `logr`.

It is the consolidation of the best ideas from the Go, JVM, .NET, Rust, JavaScript, and Python logging ecosystems into one coherent, benchmarked package: **structured logging + debug-on-error buffering + secret redaction + sampling + OpenTelemetry trace correlation + log rotation + tamper-evident audit logs + spans + message templates**, behind one small API.

> If you've ever asked "which Go logging library should I use — zap, zerolog, logrus, or slog?", this is the answer that ends the question.

---

## Table of contents

- [Why ubgo/logger](#why-ubgologger)
- [Feature highlights](#feature-highlights)
- [Install](#install)
- [Quick start (step by step)](#quick-start-step-by-step)
- [Core concepts](#core-concepts)
- [Recipes](#recipes)
  - [Structured fields (zero-allocation)](#structured-fields-zero-allocation)
  - [Fan-out to multiple sinks](#fan-out-to-multiple-sinks)
  - [Debug-on-error (FingersCrossed)](#debug-on-error-fingerscrossed)
  - [Secret/PII redaction](#secretpii-redaction)
  - [Sampling under load](#sampling-under-load)
  - [Context, tracing, and request scoping](#context-tracing-and-request-scoping)
  - [Spans (causal log trees)](#spans-causal-log-trees)
  - [Message templates](#message-templates)
  - [Events, not messages](#events-not-messages)
  - [Log file rotation](#log-file-rotation)
  - [Async delivery & backpressure](#async-delivery--backpressure)
  - [Tamper-evident audit logs](#tamper-evident-audit-logs)
  - [Runtime log level (HTTP / signal / file)](#runtime-log-level-http--signal--file)
  - [The slog bridge](#the-slog-bridge)
  - [Testing your logs](#testing-your-logs)
- [Migrating from zap / zerolog / logrus / slog](#migrating-from-zap--zerolog--logrus--slog)
- [Contrib modules](#contrib-modules)
- [Performance](#performance)
- [FAQ](#faq)
- [Documentation](#documentation)
- [License](#license)

---

## Why ubgo/logger

`log/slog` won the Go logging interface war — the whole ecosystem now writes `slog.Handler` backends. But `slog` is deliberately minimal: **no sampling, no log rotation, no async/backpressure, no PII redaction, no dedup, no runtime level control**, and writing a *correct* `slog.Handler` is a documented footgun. The community filled the gaps with 50+ tiny, single-purpose dependencies.

`ubgo/logger` is **the slog backend that fills every gap** — one dependency, one mental model, honest benchmarks:

- ✅ **slog-native** — it *is* a correct `slog.Handler` (passes the standard library's `testing/slogtest`). The entire slog ecosystem composes on top.
- ✅ **Zero-allocation** typed hot path (CI-enforced), competitive with `zap` and `zerolog`.
- ✅ **One extension seam** — a processor pipeline. Redaction, sampling, enrichment, dedup are all the same concept.
- ✅ **Batteries included** — rotation, redaction, sampling, OTEL correlation, FingersCrossed, audit, network/cloud sinks — built in, not 50 dependencies.
- ✅ **Drop-in migration** from zap, zerolog, logrus, std `log`, and `logr`.

## Feature highlights

| Category | What you get |
|---|---|
| **API** | `slog`-native · type-safe generic fields (`String`, `Int[T]`, …) · message templates · named events |
| **Performance** | zero-allocation typed path (~295 ns/op, 0 B, 0 allocs, CI-gated) · object pooling |
| **Transports** | sync · bounded-channel · **lock-free Disruptor ring**; explicit `Block`/`DropNewest`/`DropOldest` backpressure + dropped-count |
| **Reliability** | per-sink level + encoder + failure isolation · honest drop accounting |
| **Differentiators** | **FingersCrossed** debug-on-error buffering · **compiled path-DSL redaction** · **spans-as-context** causal trees · **tamper-evident audit chain** |
| **Context** | `context.Context` propagation · OTEL `trace_id`/`span_id` correlation · MDC-equivalent bound fields |
| **Sinks** | console (TTY-aware) · JSON · logfmt · file (rotation/retention/gzip) · syslog · TCP/UDP/TLS · Loki · Datadog · Elasticsearch · OTLP · Sentry |
| **Ops** | runtime level via HTTP / signal / config file · self-metrics endpoint |
| **DX** | `Development()`/`Production()` presets · `logtest` assertion kit · panic-recovery helpers |

## Install

Requires **Go 1.24+**.

```bash
go get github.com/ubgo/logger
```

Optional adapter modules (only pull the heavy dependency you use):

```bash
go get github.com/ubgo/logger/contrib/zap      # migrate from uber-go/zap
go get github.com/ubgo/logger/contrib/logrus   # migrate from sirupsen/logrus
go get github.com/ubgo/logger/contrib/zerolog  # migrate from rs/zerolog
go get github.com/ubgo/logger/contrib/phuslu   # migrate from phuslu/log
go get github.com/ubgo/logger/contrib/logr     # Kubernetes / controller-runtime
go get github.com/ubgo/logger/contrib/otel     # OpenTelemetry Logs bridge
go get github.com/ubgo/logger/contrib/sentry   # Sentry error events
```

## Quick start (step by step)

### 1. The simplest possible logger

```go
package main

import logger "github.com/ubgo/logger"

func main() {
	log := logger.New() // JSON to stderr at Info
	defer log.Close()

	log.Info("server started", logger.String("addr", ":8080"), logger.Int("pid", 4242))
}
```

```json
{"time":"2026-05-19T12:00:00Z","level":"info","msg":"server started","addr":":8080","pid":4242}
```

### 2. Use a preset

```go
log := logger.Development() // pretty, colored, Debug, caller — for local dev
// or
log := logger.Production()  // JSON, Info, async, sampled — for services
defer log.Close()
```

### 3. Add request context

```go
reqLog := log.With(logger.String("request_id", "abc-123"))
reqLog.Info("handling request") // request_id on every line
```

### 4. Wire it as the standard `slog` logger (so all libraries benefit)

```go
import "log/slog"

slog.SetDefault(log.NewSlog())
slog.Info("now every slog call in your deps flows through ubgo/logger")
```

### 5. Build a production pipeline

```go
log := logger.New(
	logger.WithLevel(logger.LevelInfo),
	logger.WithProcessors(
		logger.NewPathRedactor(logger.Mask, "[REDACTED]", "*.password", "*.token"),
		logger.NewSampleProcessor(100, 100), // first 100, then 1/100 — never drops ERROR
	),
	logger.WithTransport(logger.NewDisruptorTransport(
		logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo),
		8192, logger.DropNewest,
	)),
)
defer log.Close() // drains the async ring
```

That's the whole setup. The sections below show each capability.

## Core concepts

There are five nouns:

- **Logger** — what you call (`log.Info(...)`). Immutable; `With()` returns a child.
- **Field** — a type-safe key/value (`logger.String`, `logger.Int[T]`, `logger.Err`, …). Scalars are unboxed → zero allocation.
- **Processor** — the single extension seam: `func(ctx, *Record) error`. Enrichment, redaction, sampling, dedup are all processors. Returning `logger.ErrDrop` drops the record (this is how sampling works).
- **Transport** — how a record gets from the call site to the sink: `Sync` (inline), `Channel` (bounded queue), or `Disruptor` (lock-free ring) — each with an explicit overflow policy.
- **Sink** — the destination (console, file, network, cloud). Each sink owns its own level + encoder; a `Fanout` broadcasts to many with failure isolation.

Full design rationale: [`docs/architecture.md`](./docs/architecture.md).

## Recipes

### Structured fields (zero-allocation)

```go
log.Info("payment processed",
	logger.String("user", userID),
	logger.Int("amount_cents", 1999),
	logger.Bool("captured", true),
	logger.Dur("latency", elapsed),
	logger.Err(err), // nil-safe; emits "error":null
)
```

Use `logger.Any(key, v)` for arbitrary values (reflection, off the hot path).

### Fan-out to multiple sinks

```go
console := logger.NewConsoleSink(os.Stdout, logger.LevelDebug) // pretty, TTY-aware
jsonF, _ := logger.NewRotatingFile("/var/log/app.log")
file := logger.NewFileSink(jsonF, logger.NewJSONEncoder(), logger.LevelInfo)

log := logger.New(logger.WithSink(logger.NewFanout(console, file)))
```

Each sink keeps its own level and encoder; one failing sink never blocks the others.

### Debug-on-error (FingersCrossed)

The killer feature. A successful request logs **nothing** below the activation level. The first error flushes the entire buffered debug trail — so you get full forensics exactly when something breaks, and silence when it doesn't.

```go
fc := logger.NewFingersCrossed(
	logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelTrace),
)
log := logger.New(logger.WithTransport(logger.NewSyncTransport(fc)), logger.WithLevel(logger.LevelTrace))

func handler(w http.ResponseWriter, r *http.Request) {
	ctx := logger.FCScope(r.Context()) // one buffer per request
	log.DebugContext(ctx, "loaded config")
	log.DebugContext(ctx, "queried db")
	// if everything succeeds → nothing is emitted
	// if log.ErrorContext(ctx, "boom") fires → the two Debug lines + the error are all flushed
}
```

### Secret/PII redaction

Redaction happens **in-process, before bytes reach any sink** — the only place raw values and structure coexist.

```go
pr := logger.NewPathRedactor(logger.Mask, "[REDACTED]",
	"*.password",                  // any password field at any depth
	"req.headers.authorization",   // exact dotted path
	"user.**",                     // everything under user
)
log := logger.New(logger.WithProcessors(pr))
```

Strategies: `logger.Mask` (replace), `logger.Hash` (sha256 prefix — keeps correlation), `logger.Drop` (remove).

### Sampling under load

```go
// keep the first 100, then 1 in every 100 — but NEVER sample ERROR and above
log := logger.New(logger.WithProcessors(logger.NewSampleProcessor(100, 100)))
```

`DedupProcessor` collapses identical repeated lines and annotates the survivor with `deduped_count`.

### Context, tracing, and request scoping

```go
ctx = logger.ContextWith(ctx, logger.String("tenant", "acme")) // MDC-style bound field
log.InfoContext(ctx, "doing work")                              // tenant included automatically
```

For OpenTelemetry trace correlation, add the enricher with the OTEL extractor (see [`contrib/otel`](./contrib/otel)):

```go
log := logger.New(logger.WithProcessors(
	logger.NewEnrichProcessor(otellogger.TraceExtractor()), // adds trace_id/span_id from the active span
))
```

### Spans (causal log trees)

```go
ctx, span := log.StartSpan(ctx, "checkout", logger.String("order", id))
defer span.End() // emits span.end with duration + ok

log.InfoContext(ctx, "charging card") // inherits span identity + fields
_, child := log.StartSpan(ctx, "charge_gateway")
// ... span_path "1.1" lets you reconstruct the tree from a flat log stream
child.Fail(err) // span.end becomes level=error, ok=false
child.End()
```

### Message templates

Serilog-style: one call gives you readable text **and** structured fields **and** a stable grouping key.

```go
log.Infot("processed {count} files for {user}", 12, "ada")
// msg="processed 12 files for ada"
// msg_template="processed {count} files for {user}"  ← stable for alerting/grouping
// count=12, user="ada"                               ← structured
```

### Events, not messages

```go
log.Event("user.signup", logger.String("plan", "pro"), logger.Int("uid", 7))
// no prose — the event name is the primary index (great for analytics/AI)
```

### Log file rotation

Built in. No `lumberjack` dependency.

```go
rf, _ := logger.NewRotatingFile("/var/log/app.log")
rf.MaxSizeBytes = 100 << 20 // 100 MiB
rf.MaxBackups = 7
rf.MaxAge = 14 * 24 * time.Hour
rf.Compress = true // gzip rotated segments
log := logger.New(logger.WithSink(logger.NewFileSink(rf, logger.NewJSONEncoder(), logger.LevelInfo)))

// logrotate-friendly: reopen on SIGHUP
stop := logger.OnSIGHUP(func() { _ = rf.Reopen() })
defer stop()
```

### Async delivery & backpressure

```go
sink := logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)

// bounded channel + worker
t := logger.NewChannelTransport(sink, 4096, logger.DropNewest)
// or lock-free Disruptor ring for max throughput
t := logger.NewDisruptorTransport(sink, 8192, logger.Block)

log := logger.New(logger.WithTransport(t))
defer log.Close() // drains the queue

// dropped records are counted, never silent:
n := t.Dropped()
```

### Tamper-evident audit logs

```go
f, _ := os.Create("/var/log/audit.log")
audit := logger.NewAuditSink(f, logger.NewJSONEncoder())
log := logger.New(logger.WithTransport(logger.NewSyncTransport(audit)))

log.Info("user deleted record", logger.String("actor", "admin"), logger.Int("id", 42))
```

Each line is hash-chained (`sha256(prev || record)`). Verify integrity later:

```go
res := logger.VerifyAudit(file)
if !res.OK {
	fmt.Printf("tampered at seq %d: %s\n", res.BrokenAtSeq, res.Reason)
}
```

Detects edits, deletions, and reordering.

### Runtime log level (HTTP / signal / file)

```go
lv := logger.NewLevelVar(logger.LevelInfo)
log := logger.New(logger.WithLeveler(lv))

// 1. HTTP: GET/PUT /loglevel?level=debug
http.Handle("/loglevel", logger.NewLevelHandler(lv))

// 2. Signal: flip to debug on SIGUSR2, back on next
stop := logger.CycleLevelOnSignal(lv, syscall.SIGUSR2, logger.LevelInfo, logger.LevelDebug)
defer stop()

// 3. Config file: {"level":"warn"} hot-reloaded
_, stopW := logger.WatchConfigFile("/etc/app/log.json", lv, 5*time.Second)
defer stopW()
```

Self-metrics (emitted/dropped/by-level) are exposed too:

```go
http.Handle("/logmetrics", log.Metrics())
```

### The slog bridge

```go
slog.SetDefault(log.NewSlog())
// every slog.Handler middleware (samber/slog-*, otelslog) composes on top of ubgo/logger
```

### Testing your logs

```go
import "github.com/ubgo/logger/logtest"

func TestSignup(t *testing.T) {
	log, cap := logtest.New()
	svc := NewService(log)
	svc.Signup("ada")

	cap.AssertLogged(t, logger.LevelInfo, "signup complete")
	cap.AssertField(t, "user", "ada")
	cap.AssertNoErrors(t)
}
```

## Migrating from zap / zerolog / logrus / slog

Migration is mechanical — keep your existing call sites, swap the engine.

| From | How | Module |
|---|---|---|
| `log/slog` | `slog.SetDefault(log.NewSlog())` | core (no extra dep) |
| std `log` | `logger.RedirectStdLog(log, logger.LevelInfo)` | core |
| `uber-go/zap` | `zaplogger.New(core, zapcore.InfoLevel)` | [`contrib/zap`](./contrib/zap) |
| `sirupsen/logrus` | `logruslogger.Attach(logrusLogger, core)` | [`contrib/logrus`](./contrib/logrus) |
| `rs/zerolog` | `zerologlogger.New(zl, logger.LevelInfo)` | [`contrib/zerolog`](./contrib/zerolog) |
| `phuslu/log` | `phulogger.New(pl, logger.LevelInfo)` | [`contrib/phuslu`](./contrib/phuslu) |
| `go-logr/logr` | `logrlogger.New(core)` | [`contrib/logr`](./contrib/logr) |

Full guide: [`docs/migration.md`](./docs/migration.md).

## Contrib modules

Heavy third-party dependencies are isolated in separate, independently-versioned submodules so the core stays dependency-free:

| Module | Purpose |
|---|---|
| [`contrib/zap`](./contrib/zap) | Forward `zap` call sites through ubgo/logger |
| [`contrib/logrus`](./contrib/logrus) | `logrus.Hook` + `Attach()` drop-in |
| [`contrib/zerolog`](./contrib/zerolog) | Ship through a `zerolog.Logger` |
| [`contrib/phuslu`](./contrib/phuslu) | Ship through a `phuslu/log` writer |
| [`contrib/logr`](./contrib/logr) | `logr.Logger` for Kubernetes / controller-runtime |
| [`contrib/otel`](./contrib/otel) | OpenTelemetry Logs bridge + W3C trace extractor |
| [`contrib/sentry`](./contrib/sentry) | WARN+ records as Sentry events |

## Performance

Measured on Apple M-series, Go 1.24, output to `io.Discard`. Allocation count is enforced by a CI gate (`TestZeroAlloc*`).

| Path | ns/op | B/op | allocs/op |
|---|--:|--:|--:|
| **Typed hot path** | **~295** | **0** | **0** |
| Disabled level (gated out) | ~7 | 0 | 0 |
| Through the `slog` bridge | ~698 | 320 | 1 |
| stdlib `slog` JSON (reference) | ~704 | 0 | 0 |

The slog-bridge row is the **honest through-bridge cost** (slog's own `Record`/attrs allocation for >5 attrs) — published, not hidden. "Portable via slog" silently costing 10–40× is the ecosystem trap this library refuses to repeat.

See [`docs/performance.md`](./docs/performance.md) for the methodology and how to reproduce.

## FAQ

**Is ubgo/logger a replacement for zap / zerolog / logrus?**
Yes — it's a zero-allocation, slog-native superset with batteries included, plus drop-in migration shims so switching is mechanical.

**Should I use it instead of `log/slog`?**
Use slog's *API*; get ubgo/logger's *engine*. It implements `slog.Handler` (passing `testing/slogtest`) and adds sampling, rotation, redaction, async, FingersCrossed, audit, and trace correlation that slog deliberately omits.

**Does it support OpenTelemetry?**
Yes — `contrib/otel` is an OTEL Logs bridge, and the core's level model *is* the OTEL `SeverityNumber`. Logs correlate with traces via `trace_id`/`span_id`.

**Is it production-ready?**
The full feature set is implemented and race-tested with a CI matrix across all modules and an allocation-regression gate. APIs are stabilizing toward a `v1`.

**Why not just import 50 `samber/slog-*` packages?**
You can — they compose on top, since ubgo/logger is a correct `slog.Handler`. But the things you actually need in production (rotation, redaction, sampling, backpressure, debug-on-error) are first-class here, in one dependency, benchmarked together.

**Zero dependencies?**
The core module has **no third-party dependencies**. Heavy integrations live in opt-in `contrib/*` submodules.

## Documentation

- [Getting started](./docs/getting-started.md)
- [Architecture & design](./docs/architecture.md)
- [Sinks & transports](./docs/sinks.md)
- [Processors & the pipeline](./docs/processors.md)
- [Migration guide](./docs/migration.md)
- [Performance](./docs/performance.md)
- [API reference (pkg.go.dev)](https://pkg.go.dev/github.com/ubgo/logger)

## License

[Apache-2.0](./LICENSE) © the ubgo authors.

---

<sub>Keywords: Go logging library, golang structured logging, slog handler, zap alternative, zerolog alternative, logrus replacement, zero allocation logger, OpenTelemetry logging Go, log rotation, PII redaction, debug on error, tamper-evident audit log, Kubernetes logr.</sub>

# ubgo/logger

[![CI](https://github.com/ubgo/logger/actions/workflows/ci.yml/badge.svg)](https://github.com/ubgo/logger/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ubgo/logger.svg)](https://pkg.go.dev/github.com/ubgo/logger)
[![Go Report Card](https://goreportcard.com/badge/github.com/ubgo/logger)](https://goreportcard.com/report/github.com/ubgo/logger)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

A pluggable, adapter-based, **slog-native** structured logging core for Go — the consolidation of the best ideas across the Go, JVM, .NET, Rust, JS and Python logging ecosystems into one coherent, benchmarked package.

> Status: **v1 foundation, in development.** Core pipeline, transports, encoders, sinks and the slog bridge are implemented and race-tested. Sinks/adapters (OTEL, file rotation, zap/logrus shims) and v2 features are tracked in the plan repo.

## Why

`log/slog` won the interface war but is deliberately minimal — no sampling, rotation, async/backpressure, redaction, dedup, or runtime level control, and writing a *correct* `slog.Handler` is a documented footgun. The ecosystem fragmented into 50+ single-purpose micro-dependencies to fill the gaps. `ubgo/logger` is **the slog backend that fills them**, with one extension concept and honest, benchmarked performance.

## Design

- **slog-native** — it *is* a correct `slog.Handler` (WithGroup/WithAttrs threaded right), so the whole `slog` ecosystem composes on top. Never leaks a concrete backend type.
- **One extension seam** — a `Processor` pipeline (`func(ctx, *Record) error`). Enrichment, redaction, sampling, dedup are all Processors. `ErrDrop` = sampling. One concept teaches the library.
- **Pluggable transport** — `Sync` (inline), `Channel` (bounded chan + worker), `Ring` (bounded ring + worker), all behind one interface, each with an **explicit** `OverflowPolicy` (`Block` / `DropNewest` / `DropOldest`) and a surfaced dropped-count (never silent loss).
- **Type-safe fields via generics** — `Int[T]`, `Float[T]` store unboxed; only `Any` escapes to reflection.
- **Per-sink everything** — fan-out where each sink owns its level + encoder, with failure isolation (one dead sink can't kill the others).

## Quick start

```go
log := logger.New(
    logger.WithLevel(logger.LevelInfo),
    logger.WithProcessors(logger.NewRedactProcessor("password")),
    logger.WithSink(logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)),
)
defer log.Close()

log.Info("user login", logger.String("user", "ada"), logger.String("password", "secret"))
// → {"time":"...","level":"info","msg":"user login","user":"ada","password":"[REDACTED]"}

// slog bridge — the whole slog ecosystem sits on top:
slog.SetDefault(log.NewSlog())
```

See [`example/`](./example) for fan-out, async transport, sampling, and the slog bridge together.

## Tasks

```
task build       # compile
task test        # race tests
task bench       # benchmarks (allocs/op is the number that matters)
task check       # fmt + vet + race (pre-commit gate)
task example     # run the demo
```

## License

Apache-2.0.

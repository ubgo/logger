# Getting started with ubgo/logger

This guide takes you from zero to a production-grade logging setup in Go, step by step.

## Prerequisites

- Go 1.24 or newer
- A module (`go mod init your/app`)

## 1. Install

```bash
go get github.com/ubgo/logger
```

The core has **no third-party dependencies**.

## 2. Your first log

```go
package main

import logger "github.com/ubgo/logger"

func main() {
	log := logger.New()
	defer log.Close()

	log.Info("hello", logger.String("env", "dev"))
}
```

`logger.New()` with no options = JSON, `Info` level, to `stderr`, synchronous.

## 3. Choose a preset

For most apps you don't need to assemble anything:

```go
log := logger.Development() // colored, pretty, Debug, caller info — local dev
log := logger.Production()  // JSON, Info, async + sampled — services
```

## 4. Levels

Levels follow the OpenTelemetry `SeverityNumber` model:

```
LevelTrace(1) Debug(5) Info(9) Warn(13) Error(17) Fatal(21)
```

Guard expensive work:

```go
if log.Enabled(logger.LevelDebug) {
	log.Debug("expensive", logger.Any("dump", buildHugeStruct()))
}
```

## 5. Fields are typed

```go
log.Info("order",
	logger.String("id", id),
	logger.Int("cents", 1999),   // generic: Int[int], Int[int64], …
	logger.Bool("paid", true),
	logger.Dur("took", elapsed),
	logger.Err(err),             // nil-safe
)
```

Typed fields are **zero-allocation**. Use `logger.Any(k, v)` only when you must (reflection).

## 6. Per-request context

```go
reqLog := log.With(logger.String("request_id", rid))
reqLog.Info("started")  // request_id on every line; parent unaffected
```

Or carry fields in `context.Context`:

```go
ctx = logger.ContextWith(ctx, logger.String("tenant", "acme"))
log.InfoContext(ctx, "work")  // needs the EnrichProcessor — see processors.md
```

## 7. Make it the standard logger

So your dependencies' logs flow through the same pipeline:

```go
import "log/slog"
slog.SetDefault(log.NewSlog())
```

## 8. A realistic production logger

```go
log := logger.New(
	logger.WithLevel(logger.LevelInfo),
	logger.WithProcessors(
		logger.NewEnrichProcessor(),                                   // ctx-bound fields
		logger.NewPathRedactor(logger.Mask, "[REDACTED]", "*.password"),
		logger.NewSampleProcessor(100, 100),                           // never drops ERROR
	),
	logger.WithTransport(logger.NewDisruptorTransport(
		logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo),
		8192, logger.DropNewest,
	)),
)
defer log.Close() // ALWAYS — drains the async ring
```

## Next steps

- [Architecture & design](./architecture.md) — the five concepts
- [Processors & the pipeline](./processors.md) — redaction, sampling, FingersCrossed
- [Sinks & transports](./sinks.md) — files, network, cloud, backpressure
- [Migration guide](./migration.md) — from zap/zerolog/logrus/slog
- [Performance](./performance.md)

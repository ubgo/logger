# contrib/otel — OpenTelemetry Logs bridge for ubgo/logger

Emit `ubgo/logger` records through the OpenTelemetry Logs SDK so your logs land in the same OTLP pipeline as traces and metrics — with automatic `trace_id`/`span_id` correlation from `context.Context`.

`ubgo/logger`'s level model **is** the OTEL `SeverityNumber` (1–24), so severity survives the bridge with no lossy remap.

## Install

```bash
go get github.com/ubgo/logger/contrib/otel
```

## Usage

### 1. Ship logs to an OTLP collector

```go
import (
	logger "github.com/ubgo/logger"
	otellogger "github.com/ubgo/logger/contrib/otel"
)

// after you've configured an OTEL LoggerProvider (global or explicit):
sink := otellogger.New("my-service", logger.LevelInfo)
core := logger.New(logger.WithTransport(logger.NewSyncTransport(sink)))

core.InfoContext(ctx, "order placed", logger.String("order", id))
// the SDK attaches TraceId/SpanId/TraceFlags from the span in ctx
```

Use `otellogger.NewWithLogger(otelLogger, minLevel)` to pass an explicit `otel/log.Logger`.

### 2. Add trace correlation to *any* sink (no OTLP needed)

```go
core := logger.New(
	logger.WithProcessors(
		logger.NewEnrichProcessor(otellogger.TraceExtractor()), // trace_id/span_id on every line
	),
	logger.WithSink(logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)),
)
```

`TraceExtractor()` reads the active W3C span out of `context.Context` and the core `EnrichProcessor` stamps `trace_id`/`span_id` onto every record — so even plain JSON logs correlate with traces in Grafana/Tempo/Jaeger.

## How it works

`Sink.Emit` builds an `otel/log.Record` (timestamp, severity, body, attributes) and calls `Logger.Emit(ctx, rec)`; the OTEL SDK's processor attaches trace context from `ctx`. Flushing is owned by the SDK's batch processor.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

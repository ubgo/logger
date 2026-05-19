# Sinks & transports

A **sink** is where logs go. A **transport** is how records get there (sync or async, with an explicit backpressure policy). They are independent: any sink works with any transport.

## Encoders

| Encoder | Output |
|---|---|
| `logger.NewJSONEncoder()` | one JSON object per line (ndjson), OTEL-aligned keys |
| `logger.NewConsoleEncoder()` | human, aligned, optional color |
| `logger.NewLogfmtEncoder()` | `key=value` |

`logger.NewConsoleSink(w, level)` auto-enables color only on a real TTY and respects `NO_COLOR` — pretty in dev, plain in CI/files, no flag.

## Built-in sinks

### Console / writer

```go
logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)
logger.NewConsoleSink(os.Stdout, logger.LevelDebug) // TTY-aware
```

### File with rotation (no lumberjack)

```go
rf, _ := logger.NewRotatingFile("/var/log/app.log")
rf.MaxSizeBytes = 100 << 20
rf.MaxBackups   = 7
rf.MaxAge       = 14 * 24 * time.Hour
rf.Compress     = true // gzip rotated segments
sink := logger.NewFileSink(rf, logger.NewJSONEncoder(), logger.LevelInfo)

logger.OnSIGHUP(func() { _ = rf.Reopen() }) // logrotate compatibility
```

Retention (`MaxBackups`/`MaxAge`) is enforced synchronously on rotation; only gzip is async — so retention is deterministic.

### Syslog (Unix)

```go
s, _ := logger.NewSyslogSink("", "", "myapp", logger.NewJSONEncoder(), logger.LevelInfo) // local daemon
s, _ := logger.NewSyslogSink("tcp", "logs:514", "myapp", enc, logger.LevelWarn)          // remote
```

Severity is mapped so syslog-level filtering works. (Build-tagged: not built on Windows/Plan9.)

### Network: TCP / UDP / TLS

```go
logger.NewTCPSink("collector:5000", logger.NewJSONEncoder(), logger.LevelInfo)
logger.NewUDPSink("metrics:514", enc, logger.LevelInfo)
logger.NewTLSSink("secure:6514", tlsCfg, enc, logger.LevelInfo)
```

Lazy dial + one re-dial on write error; lost lines are counted via `.Dropped()`.

### Cloud (zero extra deps — plain HTTP)

```go
logger.NewLokiSink("http://loki:3100/loki/api/v1/push",
	map[string]string{"app": "checkout"}, logger.LevelInfo)

logger.NewDatadogSink("https://http-intake.logs.datadoghq.com/api/v2/logs",
	os.Getenv("DD_API_KEY"), logger.LevelInfo)

logger.NewElasticsearchSink("http://es:9200/_bulk", "app-logs", logger.LevelInfo)
```

All built on `HTTPBatchSink`: records are batched (`MaxBatch` / `Flush` interval) and delivered off the app's goroutine; failed batches increment `.Dropped()`.

### OTLP & Sentry

Via contrib modules: [`contrib/otel`](../contrib/otel) (OpenTelemetry Logs), [`contrib/sentry`](../contrib/sentry) (error events).

### Tamper-evident audit

```go
audit := logger.NewAuditSink(file, logger.NewJSONEncoder())
// each line hash-chained: sha256(prev || record)
res := logger.VerifyAudit(file) // detects edits, deletions, reordering
```

## Fan-out (per-sink level + isolation)

```go
log := logger.New(logger.WithSink(logger.NewFanout(
	logger.NewConsoleSink(os.Stdout, logger.LevelDebug),                       // dev: everything, pretty
	logger.NewWriterSink(file, logger.NewJSONEncoder(), logger.LevelInfo),     // prod file: info+, JSON
	sentrySink,                                                                // errors only
)))
```

Each sink filters at its own level; a dead sink can't block or crash the app or its siblings.

## Transports & backpressure

```go
sink := logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)

logger.NewSyncTransport(sink)                              // inline, lossless
logger.NewChannelTransport(sink, 4096, logger.DropNewest)  // bounded chan + worker
logger.NewDisruptorTransport(sink, 8192, logger.Block)     // lock-free ring, lossless
```

`OverflowPolicy`:

| Policy | Behavior |
|---|---|
| `Block` | caller waits for room — lossless, adds latency |
| `DropNewest` | drop the incoming record — protects latency |
| `DropOldest` | evict the oldest queued record |

Always `defer log.Close()` — it drains the queue and closes sinks. `t.Dropped()` and `log.Metrics()` expose loss; it is never silent.

See also: [architecture](./architecture.md), [processors](./processors.md).

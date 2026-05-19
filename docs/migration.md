# Migration guide

Switching to ubgo/logger is **mechanical**: keep your existing call sites, swap the engine underneath. Do it incrementally — old code keeps working through a shim while new code uses the native zero-allocation API.

## From `log/slog` (no extra dependency)

```go
import "log/slog"

core := logger.New(/* processors, sinks, … */)
slog.SetDefault(core.NewSlog())
```

Every `slog.Info(...)` (and every library that uses slog) now flows through ubgo/logger's pipeline. ubgo's handler passes the standard library's `testing/slogtest`, so behavior is correct including groups/attrs.

You can also wrap it with any `samber/slog-*` middleware — it composes, because ubgo is a real `slog.Handler`.

## From the standard `log` package

```go
restore := logger.RedirectStdLog(core, logger.LevelInfo)
defer restore() // reversible — handy in tests
log.Print("legacy line") // now structured, via ubgo
```

## From uber-go/zap

```bash
go get github.com/ubgo/logger/contrib/zap
```

```go
import zaplogger "github.com/ubgo/logger/contrib/zap"

zl := zaplogger.New(core, zapcore.InfoLevel) // *zap.Logger
zl.Info("unchanged call site", zap.String("k", "v"))
```

`zap.Field`s (including `Object`/`Array`) are converted faithfully. See [`contrib/zap`](../contrib/zap).

## From sirupsen/logrus

```bash
go get github.com/ubgo/logger/contrib/logrus
```

```go
import logruslogger "github.com/ubgo/logger/contrib/logrus"

logruslogger.Attach(logrus.StandardLogger(), core) // one line; silences logrus output
logrus.WithField("user", "ada").Warn("unchanged")
```

See [`contrib/logrus`](../contrib/logrus).

## From rs/zerolog

```bash
go get github.com/ubgo/logger/contrib/zerolog
```

ubgo becomes the pipeline; zerolog stays the writer:

```go
import zerologlogger "github.com/ubgo/logger/contrib/zerolog"

core := logger.New(logger.WithTransport(logger.NewSyncTransport(
	zerologlogger.New(zl, logger.LevelTrace),
)))
```

See [`contrib/zerolog`](../contrib/zerolog).

## From phuslu/log

```bash
go get github.com/ubgo/logger/contrib/phuslu
```

See [`contrib/phuslu`](../contrib/phuslu).

## From go-logr/logr (Kubernetes / controller-runtime)

```bash
go get github.com/ubgo/logger/contrib/logr
```

```go
import logrlogger "github.com/ubgo/logger/contrib/logr"

ctrl.SetLogger(logrlogger.New(core))
```

See [`contrib/logr`](../contrib/logr).

## Recommended path

1. **Adopt the shim** for your current logger so nothing breaks.
2. Wire `slog.SetDefault` so dependency logs unify.
3. Add a redactor + sampler + the OTEL enricher to the pipeline.
4. Migrate hot paths to the **native typed API** (`log.Info("msg", logger.String(...))`) for zero allocation.
5. Delete the shim once call sites are migrated.

## Field-type cheat sheet

| zap | zerolog | ubgo |
|---|---|---|
| `zap.String("k", v)` | `.Str("k", v)` | `logger.String("k", v)` |
| `zap.Int("k", v)` | `.Int("k", v)` | `logger.Int("k", v)` |
| `zap.Bool` | `.Bool` | `logger.Bool` |
| `zap.Duration` | `.Dur` | `logger.Dur` |
| `zap.Error(err)` | `.Err(err)` | `logger.Err(err)` |
| `zap.Any` | `.Interface` | `logger.Any` |
| `logger.With(...)` | `.With()...` | `log.With(...)` |

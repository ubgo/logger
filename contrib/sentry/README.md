# contrib/sentry — send error logs to Sentry

Ship `WARN`-and-above `ubgo/logger` records to [Sentry](https://sentry.io) as events: errors become Sentry exceptions, fields become event context, named events become tags. Sentry is for problems, not chatter — so the minimum level is clamped to `WARN`.

## Install

```bash
go get github.com/ubgo/logger/contrib/sentry
```

## Usage

```go
import (
	logger "github.com/ubgo/logger"
	sentrylogger "github.com/ubgo/logger/contrib/sentry"
	"github.com/getsentry/sentry-go"
)

sentry.Init(sentry.ClientOptions{Dsn: os.Getenv("SENTRY_DSN")})
defer sentry.Flush(2 * time.Second)

// Fan out: everything to JSON, errors additionally to Sentry.
jsonSink := logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)
sentrySink := sentrylogger.New(logger.LevelError)

log := logger.New(logger.WithSink(logger.NewFanout(jsonSink, sentrySink)))

log.Error("payment failed", logger.Err(err), logger.String("order", id))
// → Sentry event: level=error, exception from err, order in context
```

Use `sentrylogger.NewWithHub(hub, level)` for per-tenant hub isolation or tests.

## Mapping

| ubgo | Sentry |
|---|---|
| `error`-typed field | `sentry.Exception` |
| other fields | event `contexts.fields` |
| `EventName` | event message + `event` tag |
| `WARN` / `ERROR` / `FATAL` | `warning` / `error` / `fatal` |

`Sync()`/`Close()` call `hub.Flush`.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

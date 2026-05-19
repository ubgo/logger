# contrib/logrus — migrate from sirupsen/logrus to ubgo/logger

`logrus` is in maintenance mode. This shim lets you keep every `logrus` call site while the engine becomes [`ubgo/logger`](https://github.com/ubgo/logger) — gaining zero-allocation performance, redaction, sampling, FingersCrossed, and rotation with zero call-site edits.

## Install

```bash
go get github.com/ubgo/logger/contrib/logrus
```

## Usage

One line — `Attach` adds the hook and silences logrus's own output:

```go
import (
	logger "github.com/ubgo/logger"
	logruslogger "github.com/ubgo/logger/contrib/logrus"
	"github.com/sirupsen/logrus"
)

core := logger.New(
	logger.WithSink(logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)),
)

logruslogger.Attach(logrus.StandardLogger(), core)

logrus.WithField("user", "ada").Warn("legacy code, new engine")
```

Need only the hook (keep logrus output too)? Use `logruslogger.NewHook(core)` and `logger.AddHook(...)`.

## How it works

Implements `logrus.Hook` for all levels. Each `logrus.Entry`'s fields are converted to typed ubgo fields (errors become `error` fields); the entry context is propagated so trace correlation keeps working.

## Notes

- For **incremental migration**: new code should call `ubgo/logger` directly.
- Levels map onto ubgo's OTEL `SeverityNumber` model (Fatal/Panic → ubgo Fatal).

[← Back to ubgo/logger](https://github.com/ubgo/logger)

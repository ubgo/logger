# contrib/zerolog — bridge ubgo/logger ⇄ rs/zerolog

Forward `ubgo/logger` records through an existing `zerolog.Logger` — useful when a team has standardized shipping/encoding on zerolog but wants ubgo/logger's pipeline (FingersCrossed, redaction, sampling, dedup) in front of it.

## Install

```bash
go get github.com/ubgo/logger/contrib/zerolog
```

## Usage

```go
import (
	logger "github.com/ubgo/logger"
	zerologlogger "github.com/ubgo/logger/contrib/zerolog"
	"github.com/rs/zerolog"
)

zl := zerolog.New(os.Stderr).With().Timestamp().Logger()

core := logger.New(
	logger.WithProcessors(logger.NewPathRedactor(logger.Mask, "[REDACTED]", "*.password")),
	logger.WithTransport(logger.NewSyncTransport(
		zerologlogger.New(zl, logger.LevelTrace),
	)),
)

core.Info("through ubgo's pipeline, out via zerolog",
	logger.String("user", "ada"), logger.Int("n", 5))
```

## How it works

`zerologlogger.New(zl, minLevel)` returns a `logger.Sink` that re-emits each record via `zl.WithLevel(...)`. Fields are forwarded with `Interface`; named events become an `event` field. Levels map onto zerolog's level scale.

## Notes

- This is the **reverse direction** from the slog ecosystem: ubgo is the pipeline, zerolog is the writer.
- For the fastest path with no zerolog dependency, use the core JSON encoder directly.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

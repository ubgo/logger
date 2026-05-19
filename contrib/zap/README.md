# contrib/zap — migrate from uber-go/zap to ubgo/logger

A drop-in migration shim: keep every existing `*zap.Logger` call site, swap the engine to [`ubgo/logger`](https://github.com/ubgo/logger) so you gain FingersCrossed, redaction, sampling, rotation, and OTEL correlation without rewriting code.

## Install

```bash
go get github.com/ubgo/logger/contrib/zap
```

## Usage

```go
import (
	logger "github.com/ubgo/logger"
	zaplogger "github.com/ubgo/logger/contrib/zap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

core := logger.New(
	logger.WithProcessors(logger.NewPathRedactor(logger.Mask, "[REDACTED]", "*.password")),
	logger.WithSink(logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)),
)

// A *zap.Logger whose zapcore.Core forwards into ubgo/logger:
zl := zaplogger.New(core, zapcore.InfoLevel)

zl.Info("existing zap call site, new engine",
	zap.String("user", "ada"), zap.Int("status", 200))
```

Or attach to an existing core stack with `zaplogger.NewCore(core, level)` and `zapcore.NewTee(...)`.

## How it works

`zaplogger.NewCore` implements `zapcore.Core`. Fields are converted faithfully via zap's own `MapObjectEncoder` (so `zap.Object`, `zap.Array`, etc. survive). Levels map onto ubgo's OTEL `SeverityNumber` model. `With()` is preserved.

## Notes

- Use this for **incremental migration**. New code should call `ubgo/logger` directly for the zero-allocation typed path.
- `Sync()` flushes the underlying ubgo transport.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

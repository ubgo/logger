# contrib/phuslu — bridge ubgo/logger → phuslu/log

Forward `ubgo/logger` records into a `phuslu/log` writer — for teams on phuslu (one of the fastest Go loggers) that want ubgo/logger's pipeline (FingersCrossed, compiled redaction, sampling, dedup, spans) in front of phuslu's writer.

## Install

```bash
go get github.com/ubgo/logger/contrib/phuslu
```

## Usage

```go
import (
	logger "github.com/ubgo/logger"
	phulogger "github.com/ubgo/logger/contrib/phuslu"
	plog "github.com/phuslu/log"
)

pl := &plog.Logger{Level: plog.TraceLevel, Writer: &plog.ConsoleWriter{}}

core := logger.New(
	logger.WithProcessors(logger.NewSampleProcessor(100, 100)),
	logger.WithTransport(logger.NewSyncTransport(
		phulogger.New(pl, logger.LevelTrace),
	)),
)

core.Warn("disk slow", logger.String("dev", "sda"), logger.Int("pct", 92))
```

## How it works

`phulogger.New(pl, minLevel)` returns a `logger.Sink` that maps each record's level to the matching `phuslu/log` entry (`Trace`/`Debug`/`Info`/`Warn`/`Error`/`Fatal`) and forwards fields via `.Any(...)`; named events become an `event` field.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

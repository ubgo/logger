// Demo of the ubgo/logger core: typed fields, fan-out with per-sink
// level+encoder, a redaction processor, sampling, async transport, and the
// slog bridge.
package main

import (
	"os"

	logger "github.com/ubgo/logger"
)

func main() {
	// Fan-out: pretty colored console at Debug, JSON to a "file" at Info.
	console := logger.NewWriterSink(os.Stdout, func() logger.Encoder {
		e := logger.NewConsoleEncoder()
		e.Color = true
		return e
	}(), logger.LevelDebug)
	jsonSink := logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)
	fan := logger.NewFanout(console, jsonSink)

	lvl := logger.NewLevelVar(logger.LevelDebug)

	log := logger.New(
		logger.WithLeveler(lvl),
		logger.WithProcessors(
			logger.NewRedactProcessor("password", "token"),
			logger.NewSampleProcessor(3, 100),
		),
		logger.WithTransport(logger.NewChannelTransport(fan, 1024, logger.DropNewest)),
		logger.WithCaller(0),
	)
	defer log.Close()

	log.Info("service starting",
		logger.String("svc", "demo"),
		logger.Int("pid", os.Getpid()),
	)
	log.Debug("config loaded", logger.Bool("cache", true))

	// Secret is masked by the redaction processor before any sink sees it.
	log.Info("user login",
		logger.String("user", "ada"),
		logger.String("password", "hunter2"),
	)

	// slog bridge: the whole slog ecosystem can sit on top.
	sl := log.NewSlog()
	sl.Info("via slog", "order_id", 991, "amount", 42.50)

	log.Error("boom", logger.Err(os.ErrPermission))
}

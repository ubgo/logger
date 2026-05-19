# contrib/logr — run Kubernetes/controller-runtime code on ubgo/logger

[`go-logr/logr`](https://github.com/go-logr/logr) is the logging interface used across the Kubernetes ecosystem (controller-runtime, client-go, operator-sdk, klog). This module returns a `logr.Logger` backed by [`ubgo/logger`](https://github.com/ubgo/logger) so that code runs on ubgo's engine unchanged — with structured output, redaction, sampling, and OTEL correlation.

## Install

```bash
go get github.com/ubgo/logger/contrib/logr
```

## Usage

```go
import (
	logger "github.com/ubgo/logger"
	logrlogger "github.com/ubgo/logger/contrib/logr"
	ctrl "sigs.k8s.io/controller-runtime"
)

core := logger.New(
	logger.WithSink(logger.NewWriterSink(os.Stderr, logger.NewJSONEncoder(), logger.LevelInfo)),
)

log := logrlogger.New(core)            // logr.Logger
ctrl.SetLogger(log)                    // controller-runtime now logs via ubgo

log.WithName("reconciler").
	WithValues("pod", "nginx").
	Info("reconciling", "namespace", "default")
log.Error(err, "update failed")
```

## V-level mapping

`logr` uses integer verbosity (`V(n)`), not named severities:

| logr | ubgo level |
|---|---|
| `Error(...)` | `LevelError` |
| `V(0).Info` / `Info` | `LevelInfo` |
| `V(1).Info` | `LevelDebug` |
| `V(2+).Info` | `LevelTrace` |

`WithName` builds a dotted `logger` field; `WithValues` binds inherited fields.

[← Back to ubgo/logger](https://github.com/ubgo/logger)

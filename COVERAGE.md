# Test Coverage Report — github.com/ubgo/logger

Generated from `go test -covermode=atomic ./` per module (Go 1.24).
Regenerate any module's number with `task cover` in that module's directory.
CI (`test.yml`) enforces a per-module coverage floor, so these numbers cannot
silently regress. Every module is also `go test -race` clean and
`golangci-lint`-clean.

## Module summary

| Module | Coverage | CI floor |
|---|---:|---:|
| `github.com/ubgo/logger` (core) | **93.8%** | 90% |
| `contrib/logrus` | **100.0%** | 95% |
| `contrib/zap` | **91.7%** | 88% |
| `contrib/zerolog` | **100.0%** | 95% |
| `contrib/phuslu` | **100.0%** | 95% |
| `contrib/logr` | **100.0%** | 95% |
| `contrib/otel` | **100.0%** | 95% |
| `contrib/sentry` | **91.4%** | 88% |

**Average ≈ 97%** (unweighted mean of module totals). The core module is
additionally guarded by a zero-allocation regression gate
(`TestZeroAlloc*`, non-`-race`) so the typed hot path cannot regress to a
heap allocation.

## Core module — what is covered

The core's ~94% exercises, with real assertions (not smoke tests):

- **Every `Field` kind** — typed constructors + `Value()` round-trip + JSON /
  console (color + plain) / logfmt encoders, control-char escaping, the `Any`
  reflection and `json.Marshal`-error fallback.
- **slog conformance** — `testing/slogtest` (`TestSlogConformance`) + all attr
  kinds, nested/inline/empty groups, bound fields, below-level gate, the
  pipeline-drop path, zero-time handling.
- **Pipeline** — redaction (Mask/Hash/Drop, wildcards, `**`), sampler
  (first-N / 1-in-M / never-drop-ERROR / disabled), dedup, enrich + trace
  extractor, FingersCrossed (scoped + global, overflow), failed-processor
  drop + metric.
- **Transports** — Sync/Channel/Ring/Disruptor: Block lossless, DropNewest +
  DropOldest counted, drain-on-Close, concurrent no-loss under `-race`.
- **Sinks** — Fanout per-sink level + failure isolation; rotating file
  (rotate/prune/gzip/reopen/error); **TCP re-dial after a mid-stream
  connection drop**; UDP/TLS dropped accounting; HTTP-batch flush-timer +
  dropped (Loki/Datadog/ES); tamper-evident audit verify/tamper/delete +
  Sync/Close.
- **Ops & v2** — runtime HTTP level, signals, hot config reload, self-metrics;
  spans (tree/Fail/SetLevel/idempotent), templates, events; recover/Go;
  `logtest`.

## Justified-uncovered remainder

The residual few percent is **reachable only via real OS/IO faults or
platform state the test environment cannot deterministically produce**, with
no existing injection seam. No production seam was added purely to chase
100%, and no bug was found while reaching these numbers — all uncovered code
is error handling for conditions a CI runner cannot force.

Core (`github.com/ubgo/logger`):

- `sink_syslog_unix.go` `NewSyslogSink` — the *success* path needs a live
  local `syslogd`; CI runners and the sandbox have none. The dial-failure
  path *is* tested; the sink is build-tagged Unix.
- `sink_file.go` `openExisting` — `os.OpenFile`/`Stat` failing on a path that
  `MkdirAll` already succeeded for; OS-IO-fault-only. The `MkdirAll`-failure
  path *is* tested (`TestNewRotatingFileError`).
- `sink_net.go` `Emit` — the residual arm is "the second dial also fails
  immediately after a successful connection"; both the happy path and the
  re-dial path are tested, this last error-return is timing-dependent.

`contrib/zap`:

- `Check` with a `nil *CheckedEntry` and `mapLevel`'s `PanicLevel` arm —
  zap never calls `Core.Check` with a nil entry in practice; the level map
  is otherwise fully asserted.

`contrib/sentry`:

- `level()` `LevelInfo` arm — unreachable because `New()` clamps the minimum
  level to `Warn` (Sentry is for problems, not chatter), and `Emit` filters
  below `minLvl` before `level()` is called.

## Reproduce

```sh
# any module directory
task cover           # total + per-function
task cover:html      # line-by-line HTML

# or directly
go test -covermode=atomic -coverprofile=coverage.out ./
go tool cover -func=coverage.out | tail -1
```

CI runs `go test -race -coverprofile=cover.out ./...` for every module plus
the no-`-race` `TestZeroAlloc*` gate, and fails the build if a module drops
below its floor above.

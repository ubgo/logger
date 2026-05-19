# Test coverage report

**Core module statement coverage: ~94%** (`go test -cover ./`), all tests
green under `-race`, with a separate zero-allocation regression gate.

This project treats tests as a correctness tool, not a number to game. Every
test asserts real behavior — the suite has already caught a genuine data race
(`ConfigWatcher.OnReload`) and a non-deterministic retention bug
(`RotatingFile` prune) that shipped green before coverage was raised.

## Regenerate

```bash
task cover          # prints total + per-function, writes coverage.out
task cover:html     # opens an HTML line-by-line report
# or directly:
go test -coverprofile=coverage.out ./
go tool cover -func=coverage.out | tail -1
go tool cover -html=coverage.out
```

CI runs `go test -race -coverprofile=cover.out ./...` for every module
(`test.yml`) plus the no-race `TestZeroAlloc*` allocation gate.

## What the suite covers

- **Every `Field` kind** — typed constructors + `Value()` round-trip + all
  encoder branches (JSON / console color+plain / logfmt) including control-char
  escaping and the `Any` reflection / `json.Marshal`-error fallback.
- **Levels** — band names, `MarshalText`, `lower`, `SeverityNumber`,
  `appendNumeric`, `LevelVar`, `slogToLevel` all bands.
- **slog conformance** — `testing/slogtest` (`TestSlogConformance`) plus all
  attr kinds, nested + inline + empty groups, bound-fields, below-level gate,
  pipeline-drop, zero-time handling.
- **Pipeline** — `ProcessorFunc`, `AddField`, failed-processor drop +
  metric, redaction (Mask/Hash incl. non-string, wildcards, `**`), sampler
  (first-N keep / 1-in-M / never-drop-ERROR / `Thereafter=0` disable), dedup
  (default window, suppression count).
- **Transports** — Sync/Channel/Ring/Disruptor: Block lossless, DropNewest +
  DropOldest counted, drain-on-Close, concurrent no-loss under `-race`.
- **Sinks** — Fanout per-sink level + failure isolation; rotating file
  (rotate, prune/retention, gzip, reopen, error path); **TCP re-dial after a
  mid-stream connection drop** (resilience); UDP/TLS dropped accounting;
  HTTP-batch flush-timer + dropped-on-bad-URL (Loki/Datadog/ES); audit chain
  verify + tamper/deletion + Sync/Close delegation.
- **Ops** — runtime HTTP level (good/bad/method-not-allowed), `OnSIGHUP`,
  `CycleLevelOnSignal`, hot config reload (apply/`OnReload`/idempotent Stop),
  self-metrics endpoint, `RedirectStdLog`.
- **v2** — spans (tree path, Fail, SetLevel, idempotent End), message
  templates (escapes, unterminated, short-args, below-level), events.
- **Recovery / DX** — `Recover` re-panic, `RecoverAndContinue`, `Go`
  safe-goroutine, presets, `logtest` assertions, `isTTY` branches.

## What is intentionally *not* covered (and why)

These are the remaining ~6%. They are deliberately left rather than padded
with contrived tests:

| Area | Why uncovered |
|---|---|
| `SyslogSink` success path | Needs a live local `syslogd`; CI runners/sandbox have none. The error path *is* tested; the sink is build-tagged Unix. |
| `RotatingFile.openExisting` deep error arms | `os.OpenFile`/`Stat` failing on an already-`MkdirAll`'d path requires kernel-level fault injection; the `MkdirAll` failure path *is* tested. |
| Some `os.*` / network second-failure returns | Pure error-propagation lines (e.g. both dial attempts failing after a successful one) — low value vs. the fault-injection harness needed. |
| A few defensive `if x == nil` arms only reachable via unexported direct calls | Unreachable through the public API; covered where reachable. |

Pushing past ~94% would mean mock-heavy fault injection that inflates the
number without catching bugs. The line is drawn at "every test proves
something."

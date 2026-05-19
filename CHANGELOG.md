# Changelog

All notable changes to this project. Format loosely follows
[Keep a Changelog](https://keepachangelog.com/); this project uses semver
once v1.0.0 is tagged.

## [v0.1.0] — 2026-05-19

First tagged foundation. Zero-allocation typed hot path
(295 ns/op · 0 B · 0 allocs — verified by a CI alloc-regression gate),
slogtest-conformant, all four plan phases landed. See sections below.

## [Unreleased]

### Added — v2 (Phase: differentiators round 2)

- **Spans-as-context** (`StartSpan`/`End`/`Fail`): scoped structured
  context with an outcome; child logs inherit span identity + fields;
  Eliot-style hierarchical `span_path` (task_level) so a flat stream
  reconstructs the causal tree. Single duration+outcome `span.end`
  record; `End` idempotent.
- **Message-template preservation** (`Infot`/`Warnt`/`Errort`/`Logt`):
  Serilog-style `"processed {count} files for {user}"` keeps the
  template as a stable `msg_template` key, renders the human `msg`, and
  emits `count`/`user` as structured fields — in one call. `{{`/`}}`
  escape. Convenience tier (renders a string; typed API stays the
  zero-alloc path).

### Added — v1 foundation (Phase 1)

- slog-native core: correct `slog.Handler` passing `testing/slogtest`
  (nested + inline groups, dotted prefixes, `LogValuer` resolve,
  preformatted `WithAttrs`).
- Type-safe generic fields (`String`, `Int[T]`, `Float[T]`, `Bool`, `Dur`,
  `Time`, `Err`, `Any`); scalars stored unboxed.
- OTEL-`SeverityNumber` level model (1–24) + runtime `LevelVar`.
- Pooled `Record`; `Processor` pipeline as the single extension seam
  (`ErrDrop` = sampling).
- Pluggable `Transport`: `Sync` / `Channel` / `Ring`, explicit
  `OverflowPolicy` (`Block` / `DropNewest` / `DropOldest`) + dropped-count.
- Encoders: JSON, console (TTY/`NO_COLOR` auto-detect), logfmt.
- Per-sink level+encoder `Fanout` with failure isolation.
- Lazy caller resolution.

### Added — differentiators (Phase 2)

- `FingersCrossed` scoped debug-on-error buffering (`FCScope`).
- `PathRedactor`: compiled path-DSL redaction (`*`, `**` wildcards;
  Mask / Hash / Drop).
- `EnrichProcessor` + `ContextWith`: MDC-equivalent bound fields +
  pluggable `TraceExtractor`.
- `DedupProcessor` with honest `deduped_count`.
- Runtime ops: `LevelHandler` (HTTP), `OnSIGHUP`, `CycleLevelOnSignal`,
  self-`Metrics` (+ JSON endpoint).

### Added — adapters (Phase 3)

- `RotatingFile`: owned size rotation + age/count retention + gzip +
  `Reopen()` for logrotate.
- `shim_std`: `StdLogger` / reversible `RedirectStdLog`.
- `contrib/logrus`, `contrib/zap`, `contrib/otel` (OTEL Logs bridge +
  W3C trace extractor) — subpath modules, heavy deps isolated.

### Known follow-ups

- Hot config-file reload; per-key token-bucket; tail-based sampling.
- zerolog / phuslu backend adapters.
- True lock-free Disruptor ring (interface already swappable).
- v2: message-template preservation, causal action trees, tamper-evident
  audit mode.

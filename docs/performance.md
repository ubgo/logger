# Performance

ubgo/logger is in the zero-allocation tier alongside `zap` and `zerolog`, while remaining `slog`-native. The typed hot path allocating **0 objects** is enforced by a CI gate, so it cannot silently regress.

## Numbers

Apple M-series, Go 1.24, output to `io.Discard`:

| Path | ns/op | B/op | allocs/op |
|---|--:|--:|--:|
| Typed hot path (`String`/`Int[T]`/`Bool`, 5 fields) | ~295 | 0 | 0 |
| Disabled level (below threshold, gated out) | ~7 | 0 | 0 |
| Through the `slog` bridge (5 attrs) | ~698 | 320 | 1 |
| stdlib `slog` JSON (reference) | ~704 | 0 | 0 |

## The honest through-bridge cost

The slog-bridge row is **published, not hidden**. When you log via `slog` with more than 5 attributes, `slog` itself heap-allocates its `Record` attr storage — that 320 B / 1 alloc is slog's cost, not avoidable by any backend. Many "portable via slog" setups silently pay 10–40× because they pair slog with a slow backend; ubgo's bridge is ~698 ns (vs stdlib slog's own ~704 ns), so you pay slog's inherent cost and nothing extra.

**Takeaway:** use the native typed API (`log.Info("msg", logger.String(...))`) on hot paths for 0 allocations; use the slog bridge for portability and accept slog's own attr-allocation cost.

## Why it's zero-allocation

- Scalar fields (`String`, `Int[T]`, `Float[T]`, `Bool`, `Dur`, `Time`) are stored **unboxed** in the `Field` struct — no `interface{}` boxing. Only `Any` escapes to reflection.
- `Record` and encode buffers are **pooled** (`sync.Pool`).
- The level name is a **constant string** (`Level.lower()`), not an allocated `[]byte` — this was the single allocation found via memprofile and eliminated.
- Disabled levels short-circuit before any field evaluation.

## The CI allocation gate

`alloc_test.go` uses `testing.AllocsPerRun` to assert the typed path and the disabled-level path both allocate **exactly 0** objects. It runs as a dedicated non-`-race` CI step (the race detector instruments allocations, so the assertion is meaningless under `-race` and is skipped there via a build-tagged constant).

Any change that reintroduces an allocation fails CI before it ships.

## Reproduce

```bash
cd github.com/ubgo/logger
go test -run=^$ -bench=. -benchmem ./
go test -run TestZeroAlloc -count=1 ./   # the gate
```

Benchmarks live in `bench_test.go` (typed, disabled-level, through-slog-bridge, stdlib-slog reference).

## Choosing a transport for throughput

- `SyncTransport` — lowest latency per call, but the caller pays sink I/O.
- `ChannelTransport` — moves I/O off the caller; bounded channel; great default.
- `DisruptorTransport` — lock-free Vyukov MPMC ring; lowest overhead under many concurrent producers (race-tested at 8 producers × 2000 records on a 64-slot ring with zero loss under `Block`).

Pick `DropNewest`/`DropOldest` if predictable latency matters more than completeness under extreme load — and watch `transport.Dropped()` / `log.Metrics()`.

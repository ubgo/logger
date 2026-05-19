# Contributing to ubgo/logger

Thanks for your interest in improving ubgo/logger. This project aims to be the
last logging library a Go developer needs — correctness, zero-allocation
performance, and honest accounting are non-negotiable, so contributions are
held to a high bar.

## Ground rules

- **The core module has zero third-party dependencies.** Anything that needs a
  heavy dependency goes in a `contrib/<name>` submodule.
- **The typed hot path must stay zero-allocation.** It is enforced by
  `TestZeroAlloc*`. If your change adds an allocation there, it will fail CI.
- **Everything is race-tested.** New concurrent code ships with a `-race` test.
- **No silent loss.** Anything that drops a record (sampling, backpressure,
  delivery failure) must count it and expose it.
- **slog conformance is sacred.** The `slog.Handler` must keep passing
  `testing/slogtest` (`TestSlogConformance`).

## Development setup

```bash
git clone https://github.com/ubgo/logger
cd logger
go test ./...                       # core
go test -race ./...                 # race
go test -run TestZeroAlloc -count=1 .   # allocation gate (no -race)
go test -bench=. -benchmem -run=^$ ./   # benchmarks
```

A `Taskfile.yml` is provided if you use [Task](https://taskfile.dev):

```bash
task check   # gofmt + vet + race tests
task bench
```

Contrib modules are separate Go modules with a local `replace`:

```bash
cd contrib/zap && go test ./...
```

## Before you open a PR

1. `gofmt -w .` (CI fails on unformatted code).
2. `go vet ./...` clean.
3. `go test -race ./...` green for every module you touched.
4. `go test -run TestZeroAlloc -count=1 .` still passes if you touched the core
   hot path.
5. Add/extend tests — coverage should not regress (currently ~87%).
6. Update `CHANGELOG.md` under `## [Unreleased]`.
7. Update relevant docs (`README.md`, `docs/`, the contrib `README.md`).

## Commit & PR conventions

- Conventional-commit-style subjects (`feat:`, `fix:`, `perf:`, `docs:`,
  `test:`, `refactor:`).
- One logical change per PR. Explain the *why*, not just the *what*.
- If a change fixes a correctness bug, say so explicitly and add the
  regression test in the same PR.

## Adding a new sink or adapter

- A sink implements `logger.Sink` (`Emit`/`Sync`/`Close`).
- If it can lose records, add a `Dropped() uint64` and count.
- Heavy dependency → new `contrib/<name>` module with its own `go.mod`
  (`replace github.com/ubgo/logger => ../..`), `README.md`, and a test using
  `httptest`/an in-process listener.
- Add the module to the CI matrix in `.github/workflows/ci.yml`.

## Reporting bugs

Open an issue with a minimal reproduction. For anything involving data races,
include the `-race` output. For performance regressions, include
`go test -bench -benchmem` before/after.

## Security

Do not open public issues for vulnerabilities — see [`SECURITY.md`](./SECURITY.md).

## License

By contributing, you agree that your contributions are licensed under the
[Apache-2.0 License](./LICENSE).

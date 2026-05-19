# Security Policy

## Supported versions

ubgo/logger is pre-1.0; security fixes land on `main` and in the latest
release. Pin a commit and upgrade promptly until a `v1` is tagged.

## Reporting a vulnerability

**Do not open a public issue for security problems.**

Use GitHub's private vulnerability reporting:
**[Report a vulnerability](https://github.com/ubgo/logger/security/advisories/new)**

Please include:

- affected version / commit,
- a minimal reproduction,
- impact (e.g. log injection, PII leak through a redaction bypass, DoS via a
  sink/transport, audit-chain forgery),
- any suggested fix.

You will get an acknowledgement within a few days. Coordinated disclosure is
appreciated; we will credit reporters unless you prefer otherwise.

## Security-relevant areas

This library handles potentially sensitive data, so these areas are treated as
security-critical:

- **Redaction** (`PathRedactor`) — a bypass that lets a secret reach a sink is
  a vulnerability. Redaction runs in-process *before* any sink by design.
- **Tamper-evident audit** (`AuditSink`/`VerifyAudit`) — any way to mutate a
  chained log without `VerifyAudit` detecting it is a vulnerability.
- **Sinks** — network/cloud sinks must not leak credentials (headers, URLs)
  into other sinks or error output.
- **No silent loss** — sampling/backpressure dropping records without counting
  is treated as a correctness/security issue (audit completeness).

## Out of scope

- Misconfiguration (e.g. pointing a redactor at the wrong keys).
- Logging secrets you explicitly pass without a redactor.
- Third-party dependency advisories in `contrib/*` — report those upstream,
  though we will bump the dependency.

# Observability Baseline

Observability is intentionally vendor-neutral at project bootstrap.

When introduced, define:
- structured logs;
- request/correlation ID behavior;
- useful request metrics;
- traces for relevant external calls;
- health/readiness semantics;
- redaction rules.

Record vendor-specific decisions in an ADR rather than embedding them in general engineering rules.

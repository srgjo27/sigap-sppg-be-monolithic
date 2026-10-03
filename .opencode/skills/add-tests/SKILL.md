---
name: add-tests
description: Adds focused Go tests for changed behavior with deterministic fixtures and appropriate integration depth.
---

# Add tests

Choose the narrowest test level that proves the behavior:
- unit for pure business logic;
- handler/contract tests for HTTP mapping;
- integration tests for DB/external boundaries.

Prefer table-driven tests for related cases. Include failure paths and boundary values. Avoid time/network dependence unless the test is explicitly integration-oriented.

---
name: add-tests
description: Add idiomatic deterministic Go tests for changed behavior across domain, application, and Gin HTTP layers.
---

Prefer:
- table-driven tests;
- standard `testing` package;
- focused fixtures/builders;
- `httptest` for HTTP behavior;
- isolated business-rule tests.

Cover success, validation errors, not-found/conflict cases when relevant, and important edge cases.
Avoid real network calls and wall-clock dependence unless testing integration behavior intentionally.

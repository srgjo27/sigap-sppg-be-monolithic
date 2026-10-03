---
name: debug
description: Diagnose a Go + Gin defect from reproduction through root cause, minimal fix, regression test, and verification.
---

Workflow:
1. Reproduce or establish the failure path.
2. Inspect logs, stack traces, tests, and relevant code.
3. Form the smallest evidence-backed hypothesis.
4. Implement the minimal fix.
5. Add a regression test before broad refactoring.
6. Run focused verification, gofmt, go test, and go vet.
7. Explain root cause and why the regression test catches it.

---
name: debug
description: Investigates a Go defect systematically before making a minimal fix.
---

# Debug workflow

1. Reproduce the issue or verify the reported symptom.
2. Trace the execution path from boundary to failure.
3. Identify the earliest incorrect assumption/state.
4. Add or locate a regression test.
5. Make the smallest fix.
6. Run the regression test, then related tests.
7. Check for concurrency, nil, timeout, transaction, and error-wrapping edge cases when relevant.
8. Summarize root cause, fix, and regression coverage.

Do not mask symptoms with broad retries or swallowed errors.

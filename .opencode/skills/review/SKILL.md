---
name: review
description: Performs a structured Go code review focused on correctness, security, tests, and maintainability.
---

# Review checklist

1. Understand intent.
2. Inspect changed files and surrounding call sites.
3. Check correctness and error paths.
4. Check concurrency and resource lifecycle.
5. Check authorization/data exposure.
6. Check API compatibility.
7. Check migration/transaction implications.
8. Check tests and observability.
9. Report only actionable findings, ordered by severity.

---
name: review
description: Perform a structured review of current Go changes for correctness, security, API compatibility, tests, and maintainability.
---

Review in this order:
1. Correctness and behavior.
2. Security and data exposure.
3. API compatibility.
4. Error handling and edge cases.
5. Concurrency/resource handling.
6. Tests.
7. Package boundaries and maintainability.

Report findings first. Do not rewrite code during review unless explicitly asked.

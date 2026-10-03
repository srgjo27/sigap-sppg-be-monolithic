---
name: modify-domain
description: Changes business rules in a Go domain/application module while protecting boundaries and invariants.
---

# Modify domain behavior

1. Locate the invariant/business rule.
2. Identify which layer currently owns it.
3. Inspect domain tests and neighboring use cases.
4. Change the smallest unit that owns the behavior.
5. Add tests around the invariant and edge cases.
6. Verify transport/persistence callers still satisfy the contract.

Never move business rules into transport or repository code just because it is convenient.

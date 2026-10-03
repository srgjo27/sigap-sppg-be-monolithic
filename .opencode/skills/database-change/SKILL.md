---
name: database-change
description: Designs and implements Go persistence changes, migrations, indexes, transactions, and repository tests.
---

# Database change

1. Inspect current schema and migration tooling.
2. Check existing query patterns and indexes.
3. Define compatibility expectations.
4. Add migration/schema change.
5. Update repository port only if a new capability is required.
6. Implement repository behavior.
7. Add integration tests where persistence semantics matter.
8. Verify rollback/backout considerations when supported.

Do not mix unrelated schema cleanup with a feature migration.

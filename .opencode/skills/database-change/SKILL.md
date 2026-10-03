---
name: database-change
description: Introduce or modify persistence behavior after a concrete database technology exists in the project.
---

Workflow:
1. Read the database documentation already present in the repository.
2. Inspect existing schema, migrations, repositories, and transaction patterns.
3. Identify compatibility and rollback concerns.
4. Change schema/migrations before dependent application code when appropriate.
5. Keep persistence-specific code in infrastructure/repository packages.
6. Add integration or repository coverage where practical.
7. Run validation and inspect generated artifacts.

Do not select a database or migration tool as part of this skill.

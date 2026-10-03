---
name: modify-domain
description: Change Go domain or application behavior while protecting boundaries and business rules.
---

Workflow:
1. Locate the current business rule and existing tests.
2. Define the smallest domain/application change.
3. Keep Gin and infrastructure out of domain packages.
4. Add deterministic unit tests around the changed behavior.
5. Run focused and broader verification.
6. Update ADR/docs only when the design or public behavior changes.

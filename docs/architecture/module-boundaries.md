# Module Boundaries

A module represents a business capability, not merely a technical layer.

Recommended shape:

```text
internal/<module>/
  domain/
  application/
  repository/
  infrastructure/
  transport/
  AGENTS.md        # optional when the module needs special rules
```

## Rules

- A module owns its domain behavior.
- Cross-module access should go through explicit application/domain interfaces rather than reaching into another module's repository implementation.
- Shared utilities must stay small and generic; avoid creating a `common` package that becomes a dumping ground.
- Cyclic dependencies between business modules are a design smell and should be surfaced early.

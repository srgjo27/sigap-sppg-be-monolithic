# Module Boundaries

The default feature module shape is:

```text
internal/modules/<module>/
├── transport/http/
├── application/
├── domain/
├── repository/
└── infrastructure/
```

A module may omit directories that have no responsibility yet.

## Dependency direction

```text
transport/http -> application -> domain
                         \-> repository port
repository port <-> infrastructure implementation
```

Avoid imports that make domain code depend on Gin, SQL clients, or external provider SDKs.

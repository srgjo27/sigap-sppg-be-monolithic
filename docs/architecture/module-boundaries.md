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

## PostgreSQL implementations

`internal/modules/<nama_module>/infrastructure` keeps one `infrastructure` package,
with PostgreSQL implementations split by repository responsibility:

- If the code is more than 500 lines long, split it into several files like this `<nama_repository>_postgres.go`, `<nama_repository>_postgres.go`, and `<nama_repository>_postgres.go`
- if line code less than 500 line, use one file `postgres.go`

Each file contains its repository type, constructor, queries, and related scanners
or helpers. Common PostgreSQL error and nullable-value helpers live in
`postgres_helpers.go`. Receipt-specific transaction helpers stay in
`<nama_repository>_postgres.go`.

This file organization preserves the existing package API, dependency direction,
and modular monolith deployment model.

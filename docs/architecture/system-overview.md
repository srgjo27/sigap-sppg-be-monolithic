# System Overview

## Starting point

The application is a Go modular monolith with Gin as the HTTP transport framework.
Gin's official quickstart currently requires Go 1.25 or newer; the bootstrap script checks that prerequisite before initialization.

## Initial runtime flow

```text
HTTP request
    ↓
Gin router / middleware
    ↓
HTTP transport
    ↓
Application use case
    ↓
Domain rules
    ↓
Repository / infrastructure (when introduced)
```

## Principles

- Keep HTTP concerns at the edge.
- Keep business rules testable without Gin.
- Introduce infrastructure only where requirements need it.
- Keep dependencies explicit.
- Prefer simple packages over deep ceremony.

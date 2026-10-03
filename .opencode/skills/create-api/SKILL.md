---
name: create-api
description: Creates or changes a Go HTTP API endpoint using the project's transport, application, domain, and repository conventions.
---

# Create API

Use when implementing a new HTTP endpoint or changing an existing contract.

## Workflow
1. Find 1-2 analogous endpoints.
2. Identify module/bounded context.
3. Define request/response contract and validation.
4. Update application/use case behavior.
5. Update repository ports/implementations only when needed.
6. Wire the transport route and middleware.
7. Add focused tests for validation, business behavior, and HTTP mapping.
8. Run `gofmt`, focused tests, then broader tests when practical.
9. Update API documentation when the public contract changes.

## Guardrails
- Do not place business rules in HTTP handlers.
- Do not expose persistence models as the API contract unless that is the established pattern.
- Preserve existing error codes and response shape.
- Avoid unrelated refactors.

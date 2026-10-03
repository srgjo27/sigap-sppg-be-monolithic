---
name: create-api
description: Add a Gin REST endpoint using the project's module, transport, application, and domain conventions.
---

Workflow:
1. Inspect a similar endpoint.
2. Define request/response DTOs at the transport boundary when needed.
3. Validate and decode input.
4. Call application/use-case code.
5. Map domain/application outcomes into stable HTTP responses.
6. Add or update the route.
7. Add tests for success and relevant failure cases.
8. Run focused tests, gofmt, broader tests, and vet.

Never put business rules directly into the Gin handler.

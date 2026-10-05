# Go + Gin Backend — OpenCode Project Instructions

## Project status

This repository starts as a fresh Go backend for a web system. The HTTP framework is Gin.
The project must be initialized before application development begins.

Do not assume a module path, database, ORM, authentication provider, queue, or deployment platform until the repository or user explicitly defines them.

## Source of truth

Use this order when making decisions:
1. Existing source code and tests.
2. Nearby `AGENTS.md` files.
3. `docs/architecture/` for system and module boundaries.
4. `docs/api/` for HTTP/API behavior.
5. `docs/engineering/` for coding, testing, security, and operations.
6. `docs/adr/` for historical design decisions.
7. The current user request.

If sources conflict, identify the conflict before making a large architectural change.

## Initial bootstrap rules

The project starts with:
- Go as the backend language.
- Gin as the HTTP framework.
- A modular monolith as the default deployment model.
- `cmd/` for application entrypoints.
- `internal/` for private application packages.

Do not add PostgreSQL, Redis, JWT, ORM, migration tools, message brokers, Docker, or cloud-specific SDKs merely because they are common. Introduce them only when a requirement needs them.

## Architecture

Default dependency direction:

`transport -> application -> domain <- infrastructure`

Typical structure:

```text
cmd/
  api/
internal/
  server/
  config/
  modules/
    <module>/
      transport/http/
      application/
      domain/
      repository/
      infrastructure/
docs/
```

Rules:
- `transport` knows HTTP/Gin concerns.
- `application` orchestrates use cases and dependencies.
- `domain` contains business rules and should not depend on Gin.
- `repository` contains ports/interfaces when abstraction is useful.
- `infrastructure` contains concrete external-system implementations.
- Avoid creating layers that have no responsibility.
- Keep package APIs small.
- Define interfaces close to the consumer.
- Prefer composition and explicit dependencies.

## Database

The project uses PostgreSQL.

Database schema is documented in:
- db/schema.sql
- docs/database/schema.md

The database must be treated as an existing source of truth.

Before creating database models or queries:
1. Inspect the actual schema.
2. Do not invent columns or relationships.
3. Do not modify the database directly.
4. Database changes must be represented as migrations.

## Go rules

- Follow idiomatic Go.
- Run `gofmt` on changed Go files.
- Prefer the standard library when it solves the problem adequately.
- Return errors instead of hiding them.
- Wrap errors with `%w` when adding context.
- Do not panic for expected runtime failures.
- Avoid `any` unless there is a concrete reason.
- Avoid premature generics and abstractions.
- Keep functions cohesive and easy to test.
- Use context propagation for request-scoped work.
- Do not store `gin.Context` in long-lived services or domain objects.

## Gin rules

- Keep Gin-specific code at the transport boundary.
- Handlers decode input, invoke application logic, and map results/errors to HTTP.
- Do not put business policy in handlers.
- Use route groups for shared middleware or resource prefixes.
- Define stable HTTP error responses; do not expose internal implementation details.
- Prefer `net/http` status constants.
- Recover/panic handling, request IDs, logging, and authentication belong in middleware when those features are introduced.

## API contract

Until a project-specific API contract exists, use the conventions in `docs/api/`.
Do not invent versioning, pagination, envelope, or error semantics per endpoint.

## Testing

Every behavior change should have appropriate tests.
Prefer:
- domain/application unit tests for business rules;
- HTTP handler tests for transport contracts;
- integration tests when real infrastructure behavior matters.

Default verification:
```text
 gofmt ./...
 go test ./...
 go vet ./...
```

Run focused tests first when possible, then broader verification.

## Security

- Never commit credentials or secrets.
- Treat request input as untrusted.
- Never log passwords, tokens, API keys, or sensitive payloads.
- Keep authentication and authorization explicit.
- Avoid exposing internal errors in HTTP responses.
- Read `docs/engineering/security.md` before security-sensitive work.

## Documentation

Update documentation when you change:
- public API behavior;
- architecture boundaries;
- operational behavior;
- a meaningful design decision.

Create an ADR for significant architectural decisions.

## Git safety

Never create commits, push, reset, rebase, force-push, or rewrite history unless explicitly requested by the user.
Inspect before modifying risky areas.

## OpenCode behavior

The primary agent is an orchestrator. It should inspect first, delegate focused work when useful, and verify the result.
Use subagents for architecture, discovery, testing, security, persistence, code review, and documentation rather than putting all expertise into one prompt.
Use skills for repeatable workflows.
Keep durable project knowledge in `AGENTS.md` and `docs/`; do not hide important architecture rules inside a one-off prompt.

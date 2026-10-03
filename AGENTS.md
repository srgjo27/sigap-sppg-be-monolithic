# Go Backend Project — OpenCode Instructions

## Mission

You are working in a production-oriented Go backend for a web system. Treat the repository as a long-lived software product, not a throwaway prototype.

## Source of truth

Before changing code, use this order of authority:
1. Existing code and tests.
2. `docs/architecture/` and module-level `AGENTS.md` files.
3. `docs/api/` and `docs/engineering/`.
4. ADRs in `docs/adr/`.
5. The current user request.

When sources conflict, stop and explain the conflict before making a large architectural change. For small implementation details, preserve the dominant existing pattern.

## Required workflow

For any non-trivial task:
1. Understand the request and identify affected bounded contexts/modules.
2. Inspect similar existing implementations before creating new abstractions.
3. State a short implementation plan when the task involves multiple files or architectural decisions.
4. Make the smallest coherent change.
5. Run focused tests first, then broader verification when practical.
6. Report changed behavior, tests run, and any remaining risk.

## Go engineering rules

- Follow idiomatic Go and the repository's declared Go version.
- Run `gofmt` on changed Go files.
- Prefer standard library solutions when they are sufficient.
- Keep interfaces small and define them close to the consumer.
- Avoid premature abstraction and speculative generic helpers.
- Prefer explicit control flow over cleverness.
- Propagate errors; do not ignore returned errors.
- Wrap errors with context using `%w` where appropriate.
- Do not panic for expected runtime errors.
- Keep domain/business rules independent of transport concerns.
- Do not put HTTP, SQL, or framework-specific details inside domain logic unless the existing architecture explicitly requires it.

## Backend architecture

The default architecture is:

`transport -> application/usecase -> domain -> infrastructure`

Typical responsibilities:
- `cmd/`: process entrypoints and wiring.
- `internal/<module>/transport`: HTTP handlers, request decoding, response mapping.
- `internal/<module>/application`: use cases and orchestration.
- `internal/<module>/domain`: business entities, value objects, domain errors/rules.
- `internal/<module>/repository`: ports/interfaces used by application/domain.
- `internal/<module>/infrastructure`: concrete DB/external-service implementations.

Do not create layers that the repository does not need merely to satisfy a diagram. Follow existing modules first.

## API rules

- Keep transport DTOs separate from persistence models when doing so prevents coupling.
- Validate input at the transport boundary.
- Authentication and authorization must be explicit.
- Return stable error codes/messages according to `docs/api/error-model.md`.
- Do not leak SQL errors, stack traces, secrets, or internal identifiers unless explicitly required by the API contract.
- Preserve backward compatibility unless the task explicitly requests a breaking change.

## Database rules

- Never edit generated code directly.
- Schema changes must have a migration or migration-equivalent tracked in the repository.
- Do not silently change destructive behavior.
- Review transaction boundaries whenever multiple writes must be atomic.
- Repository implementations must not contain business policy that belongs in application/domain layers.

## Testing rules

Use the test strategy in `docs/engineering/testing.md`.

At minimum, for changed behavior:
- add/update unit tests for business rules;
- add handler/integration coverage when the transport contract changes;
- add repository/integration coverage when persistence behavior changes.

Prefer deterministic tests. Avoid tests that depend on wall-clock timing or network access unless explicitly designed as integration tests.

## Security rules

Read `docs/engineering/security.md` for security-sensitive work.

Never commit secrets. Treat `.env` as local configuration unless the repository explicitly provides a sanitized example file.

## Documentation rules

When changing public API behavior, architecture boundaries, operational behavior, or a meaningful design decision, update the relevant documentation/ADR as part of the same change.

## Git rules

Do not create commits, push, force-push, reset, or rewrite history unless the user explicitly asks.

Before risky Git operations, explain exactly what will change.

## Context loading

Use `opencode.json` instructions plus nearby `AGENTS.md` files for durable context. Use skills for repeatable workflows. Read deeper docs only when relevant to the task; do not load every document into context unnecessarily.

## Agent delegation

Use specialized subagents for focused work:
- architecture/design questions -> `architect`
- codebase discovery -> `explorer`
- code review -> `reviewer`
- testing strategy/failures -> `test-engineer`
- security audit -> `security-auditor`
- persistence/data modeling -> `db-specialist`
- documentation -> `docs-writer`

A subagent should return findings and recommended changes unless its task explicitly requires edits.

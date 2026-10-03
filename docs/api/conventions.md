# API Conventions

## HTTP

Use standard HTTP semantics and the repository's existing versioning strategy.

## Request lifecycle

1. Parse path/query/body.
2. Validate input.
3. Authenticate/authorize.
4. Call the use case.
5. Map domain/application outcomes to stable HTTP responses.

## Response design

Use the existing API envelope if one exists. Do not create a second response shape for a new endpoint without an explicit decision.

## Compatibility

Prefer additive changes. Removing or changing semantics of a public field requires impact analysis and, when significant, an ADR.

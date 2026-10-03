# Testing Strategy

## Unit tests

Use for deterministic business rules, validation, transformations, and error mapping.

## Integration tests

Use for repository/database behavior, transaction semantics, and meaningful HTTP/database integration.

## Race detection

For concurrency-sensitive packages, use `go test -race` as part of focused verification.

## Test quality

Tests should assert behavior, not implementation details. Prefer explicit fixtures and deterministic inputs.

# API Conventions

These are initial conventions only. Replace them with the actual product API contract when defined.

## Base behavior

- Use REST-style resource naming.
- Use `net/http` status constants.
- Use JSON for request/response bodies when the endpoint is JSON based.
- Validate request input at the transport boundary.
- Do not expose internal implementation errors.

## Versioning

No URL versioning strategy is selected yet. Record the decision in an ADR once the public API needs versioning.

## Pagination

No pagination envelope is selected yet. Introduce one consistently when the first paginated resource is designed.

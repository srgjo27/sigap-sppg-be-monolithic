# API Error Model

Errors exposed over HTTP should be stable and intentional.

Recommended concepts:
- machine-readable error code;
- safe human-readable message;
- optional field-level validation details;
- request/correlation ID when the system exposes one.

Do not expose raw SQL, stack traces, filesystem paths, tokens, or upstream credentials.

The implementation must use the actual response schema of this repository if it differs from this conceptual model.

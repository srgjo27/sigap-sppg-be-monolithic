# API Error Model

Until the product contract is finalized, transport errors should be stable enough for clients to distinguish categories without exposing internals.

Suggested shape:

```json
{
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "Resource not found"
  }
}
```

Rules:
- `code` is stable and machine-readable.
- `message` is safe for client display.
- Never include stack traces, SQL errors, secrets, or internal provider responses.

# Go Engineering Standards

- `gofmt` is mandatory for changed Go files.
- Keep functions cohesive and explicit.
- Prefer small consumer-defined interfaces.
- Use context propagation for request-scoped cancellation and deadlines.
- Do not store `context.Context` in long-lived structs.
- Ensure goroutines have a clear owner and shutdown path.
- Close resources deterministically.
- Wrap errors with meaningful context.
- Avoid global mutable state.
- Make time-dependent logic injectable or controllable in tests where practical.

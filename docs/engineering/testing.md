# Testing Strategy

## Unit

Use unit tests for domain rules and application behavior.

## HTTP

Use `net/http/httptest` for Gin transport behavior.

## Integration

Use integration tests when behavior depends on a real database, external service, or other infrastructure.
Keep them separate from fast unit tests where practical.

## Default verification

```text
gofmt ./...
go test ./...
go vet ./...
```

# Runtime Baseline

The initial application is a Go HTTP service using Gin.

## Entry point

`cmd/api/main.go` owns process startup and dependency wiring.

## Server construction

`internal/server` constructs the Gin engine and registers global middleware/routes.

## Configuration

`internal/config` owns initial process configuration. Environment-backed configuration should be expanded only as requirements appear.

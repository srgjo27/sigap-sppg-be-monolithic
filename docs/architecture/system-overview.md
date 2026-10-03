# System Overview

## Goal

Provide a maintainable Go backend for a web application.

## Default request flow

`HTTP -> transport -> application/usecase -> domain -> repository port -> infrastructure -> database/external service`

## Dependency rule

Inner business logic should not depend on HTTP, database drivers, or vendor-specific clients.

## Composition root

Application wiring belongs in `cmd/` or the repository's established composition root. Dependencies should be explicit.

## Cross-cutting concerns

Authentication, authorization, logging, request IDs, metrics, tracing, rate limiting, and recovery should be implemented at the appropriate boundary and not duplicated in individual business functions.

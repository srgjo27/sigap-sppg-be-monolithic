# ADR-0001: Use a Modular Monolith as the Initial Backend Shape

- Status: accepted
- Date: 2026-10-03

## Context

The project is fresh and does not yet need independently deployed services.

## Decision

Start as a single Go service organized by application modules under `internal/`, with Gin kept at the HTTP transport boundary.

## Alternatives considered

- Package-by-technical-layer only: rejected for the initial scaffold because feature ownership becomes less obvious as the codebase grows.
- Microservices: deferred because there is no current requirement for independent deployment or scaling.

## Consequences

The repository stays simple to run and test. Module boundaries are explicit, while deployment remains one service. Splitting services later remains possible if a real operational boundary emerges.

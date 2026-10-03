# ADR-0001 — Module-Oriented Backend

- Status: Accepted
- Date: 2026-10-01

## Context

A growing Go backend needs clear ownership boundaries while avoiding a large collection of purely technical packages.

## Decision

Organize business capabilities into modules, each with transport/application/domain/repository/infrastructure responsibilities where those responsibilities are actually needed.

## Alternatives considered

- Pure horizontal layers across the whole application.
- A single package containing handlers, services, and database logic.
- A highly granular package-per-function structure.

## Consequences

- Business ownership is easier to locate.
- Module boundaries must be actively protected.
- Some shared infrastructure remains cross-cutting and must be carefully isolated.

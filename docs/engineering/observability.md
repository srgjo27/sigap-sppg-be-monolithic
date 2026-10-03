# Observability

Every request should be diagnosable through appropriate structured logs and correlation/request identifiers where the application supports them.

Metrics and traces should represent business-important boundaries, external calls, and failure classes without leaking sensitive payloads.

Avoid adding logging to every function. Prefer boundary-level instrumentation plus targeted domain diagnostics.

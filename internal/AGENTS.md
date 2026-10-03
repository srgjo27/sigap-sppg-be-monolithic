# Internal Package Rules

Everything under `internal/` is private application code.

Keep framework, transport, configuration, and infrastructure dependencies out of domain packages.
Prefer dependency injection through constructors.
Do not make package-level mutable state the default mechanism for application state.

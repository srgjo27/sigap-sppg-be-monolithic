# Security Engineering

## Secrets

Never commit credentials, API tokens, private keys, or production connection strings.

## Authentication/authorization

Authentication answers who the caller is. Authorization answers whether the caller may perform the action on the target resource. Check authorization at the use-case boundary as well as at transport when the business rule requires it.

## Input handling

Validate and normalize untrusted input. Use parameterized queries or the repository's safe query builder/ORM. Never concatenate untrusted values into SQL, shell commands, or URLs.

## Logging

Do not log passwords, tokens, session cookies, private keys, or sensitive personal data.

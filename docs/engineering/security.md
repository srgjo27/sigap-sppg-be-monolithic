# Security Baseline

- Never commit credentials.
- Read secrets from secure runtime configuration.
- Validate all untrusted input.
- Keep authorization checks close to the operation being protected.
- Do not log credentials, tokens, session secrets, or sensitive payloads.
- Return safe public errors.
- Keep third-party SDKs at infrastructure boundaries.
- Treat dependency upgrades as reviewable changes.

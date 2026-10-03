# AI Task Protocol

## Every feature

### Phase 1 — Discover
- Find the relevant module.
- Find analogous implementation.
- Read the minimum required docs.
- Identify tests and dependency wiring.

### Phase 2 — Plan
- State the intended change.
- Identify interfaces/contracts affected.
- Identify risks and verification steps.

### Phase 3 — Implement
- Make the smallest coherent change.
- Preserve existing patterns.
- Update tests and docs in the same change when required.

### Phase 4 — Verify
- `gofmt` changed files.
- Focused `go test`.
- `go vet` and broader tests when appropriate.
- Review diff for accidental changes.

### Phase 5 — Report
Return:
- what changed;
- why;
- tests run;
- known limitations/risks;
- follow-up only when genuinely needed.

# Testing and Verification Policy

## Minimum
- Add/modify tests for behavior changes.
- Prefer black-box tests at `app.Run` or CLI boundary.

## Required Commands
- `mise run lint`
- `NO_COLOR= go test ./...`

## For Bug Fixes
- Reproduce with a failing test first when feasible.
- Keep regression tests permanently.

## Anti-patterns
- No "works locally" without command output summary.
- No merging PTY/interactive changes without interactive-path coverage.

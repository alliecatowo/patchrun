# Prompt Hygiene

## Prompt shape
- Goal
- Constraints
- Files/areas
- Acceptance tests
- Non-goals

## Good prompt example
Fix PTY child hang after interactive command exits. Scope: `internal/run` + tests. Keep non-interactive deterministic. Add regression test covering child exit + blocked stdin. Verify with `NO_COLOR= go test ./...`.

## Avoid
- Vague asks ("make it better")
- Missing success criteria
- Hidden constraints not written in repo

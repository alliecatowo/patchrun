# AGENTS.md

## Mission
Patch-focused Go CLI reliability. Optimize for deterministic behavior, clear diagnostics, and test-backed changes.

## Fast Repo Map
- CLI entry: `cmd/patchrun/main.go`
- App orchestration: `internal/app`
- Command execution/PTY: `internal/run`
- Git/worktree logic: `internal/gitx`
- Integration tests: `tests/`

## Required Workflow
1. Read `codex/rules/00-priority.md` then applicable rule files.
2. For non-trivial edits, write a short plan in the task thread.
3. Implement the smallest safe change.
4. Run verification commands exactly:
   - `mise run lint`
   - `NO_COLOR= go test ./...`
5. For interactive/PTY changes, also run targeted checks from `codex/rules/40-interactive-pty.md`.

## Done Criteria
- Behavior change has at least one regression test when feasible.
- No regressions in existing suite.
- User-facing CLI text updated when flags/errors change.
- Summary includes: root cause, fix, tests, residual risk.

## Escalation
If behavior involves TTY, signals, stdin ownership, or process lifecycle, treat as high risk and add/extend coverage before merge.

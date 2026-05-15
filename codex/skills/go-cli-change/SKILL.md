# Skill: go-cli-change

Use for routine CLI behavior changes.

1. Reproduce issue with a test or command.
2. Locate boundary (`cmd/patchrun` -> `internal/app` -> `internal/run`).
3. Implement minimal fix.
4. Add regression tests.
5. Run `mise run lint` and `NO_COLOR= go test ./...`.
6. Summarize root cause + fix + evidence.

+++
title = "Agents"
template = "page.html"
+++

patchrun suits AI coding agents: whatever an agent runs, the result is a reviewable patch instead of an edit in place.

```sh
patchrun --json --no-interactive -- npm install
```

`--json` prints a machine-readable result (changed files, summary, patch path) and `--no-interactive` makes the run fail predictably instead of waiting for a prompt.

## Working on patchrun itself

The contribution workflow for agents is short: open an issue with a reproduction, plan, implement the smallest patch, pass the verification commands, and report them in the PR with before/after behaviour. Human review is required for PTY and input lifecycle changes and for git/worktree mutation semantics.

Test pyramid: unit tests for parsing, policy and execution primitives; integration tests in disposable git repos; interactive acceptance tests for PTY and terminal lifecycle. A change is blocked if behaviour changed without a regression test, if PTY code changed without interactive-path assertions, or if verification commands were not reported.

Commands that prompt (for example `mise install` asking to trust a config) must: show the prompt and accept input, not leak raw escape sequences, return immediately when the child finishes, and fail with guidance in non-interactive mode.

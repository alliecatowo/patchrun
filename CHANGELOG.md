# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `--color auto|always|never` to control ANSI output explicitly.
- `--completion bash|zsh|fish` to print a completion script to stdout.
- `--git-bin <path>` to override the `git` executable.
- `--cwd <path>` to run from a different directory without changing the shell's working dir.
- `--list-runs` and `--prune` to inspect and clean up kept worktrees.
- `--no-sidecar` to suppress the JSON metadata file written next to saved patches.
- Patch saves now write a `.meta.json` sidecar with the commit, command, exit code, duration, and diffstat totals.
- Initial release of `patchrun`.
- Disposable Git worktree workflow with baseline replay for dirty trees.
- Interactive prompt (`a/s/v/k/d`) and non-interactive modes (`--apply`, `--save`, `--stdout`, `--json`, `--no-interactive`).
- Pathspec filtering (`--include`, `--exclude`).
- Ignored-file handling (`--include-ignored`).
- Drift detection before apply.
- Optional 3-way apply fallback (`--apply-3way`).
- Command timeout with process-group kill on Unix.
- Cross-platform binary support (Linux, macOS, best-effort Windows).
- Documented exit codes (0–9) and `PATCHRUN_*` environment variables for child commands.

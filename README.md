# patchrun

[![CI](https://github.com/alliecatowo/patchrun/actions/workflows/ci.yml/badge.svg)](https://github.com/alliecatowo/patchrun/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> Run any command as a patch before it touches your repo.

```bash
patchrun -- npm install
patchrun -- npx shadcn@latest add button
patchrun -- python scripts/codemod.py
```

`patchrun` runs your command in a disposable Git worktree and gives you the
resulting patch.

No config. No containers. No agent framework. Just: **command in, patch out.**

## Why

Package installs, code generators, codemods, formatters, framework CLIs, and
AI-suggested commands can all mutate your repo. Some have dry-run flags. Many
don't. `patchrun` gives you the same workflow for all of them: run the command
first, inspect the patch, then apply (or discard) it.

## Install

```bash
go install github.com/alliecatowo/patchrun/cmd/patchrun@latest
```

Requires `git` on `PATH` (any version that supports `git worktree`).

## Examples

```bash
patchrun -- npm install
patchrun -- pnpm dlx shadcn@latest add button
patchrun -- rails generate migration CreateUsers
patchrun --apply -- prettier . --write
patchrun --save changes.patch -- python scripts/codemod.py
patchrun --json -- npm install
patchrun --keep -- some-generator
patchrun --allow-dirty -- npm install
patchrun --exclude package-lock.json -- npm install
```

## What you see

```
patchrun
repo: /Users/you/project
base: 3f91abc main
temp: /tmp/patchrun/patchrun-project-20260512-143022-a1b2c3

Running:
  npm install

[child stdout/stderr stream live...]

Command exited: 0 (4.2s)
Changed 3 files:
  M package.json
  M package-lock.json
  A .npmrc

Summary:
  3 files changed, 120 insertions, 42 deletions

Actions:
  [a] apply patch
  [s] save patch
  [v] view patch
  [k] keep worktree
  [d] discard
```

## How it works

1. Snapshots the current repo state into a detached Git worktree.
2. Replays any dirty changes (staged, unstaged, untracked) as a baseline commit.
3. Runs your command inside the worktree.
4. Stages everything the command produced and runs `git diff --binary` from the
   baseline.
5. Lets you view, save, apply, or discard the resulting patch.
6. Removes the worktree (unless `--keep`).

The baseline replay step is what makes `patchrun` work on a dirty tree: existing
changes are subtracted out so the final patch shows only what the command did.

## Patchrun is not a sandbox

**The command still runs on your machine with your user permissions.** It can
access your network, home directory, environment variables, credentials, and
files outside the repo if it wants to. `patchrun` only protects your Git
working tree from repo-local file mutations by running inside a disposable copy
and returning a patch.

Don't use `patchrun` for untrusted code. Use a container or VM for that.

## Options

| Flag | Description |
| --- | --- |
| `--apply` | Apply patch to original repo after command succeeds |
| `--apply-3way` | Use `git apply --3way` if normal apply fails |
| `--save <path>` | Save patch to path |
| `--stdout` | Print patch to stdout |
| `--json` | Print machine-readable JSON result to stdout |
| `--keep` | Keep the disposable worktree (prints its path) |
| `--worktree-dir <path>` | Parent directory for temporary worktrees |
| `--name <label>` | Human label for this run |
| `--allow-dirty` | Include the current working tree as baseline |
| `--fail-on-dirty` | Refuse to run on a dirty working tree |
| `--include-ignored` | Include ignored files created by command |
| `--include <pathspec>` | Include only pathspec (repeatable) |
| `--exclude <pathspec>` | Exclude pathspec (repeatable) |
| `--diff` | Show the patch after the command |
| `--stat` / `--no-stat` | Show or hide diffstat (default: show) |
| `--interactive` / `--no-interactive` | Force or disable the prompt |
| `--command-timeout <duration>` | Kill the command after duration (`30s`, `5m`, `1h`) |
| `--reverse` | Print/save/apply the reverse of the captured patch |
| `--check` | Verify the patch applies cleanly; do not modify the working tree |
| `--exec <cmd>` | Run additional command in the worktree (repeatable) |
| `--snapshot <dir>` | Dump the post-run worktree (minus `.git`) into `<dir>` |
| `--ignore-whitespace` | Pass `--ignore-whitespace` to `git apply` |
| `--color <mode>` | `auto` (default), `always`, or `never` |
| `--no-sidecar` | Skip the `.meta.json` file written next to saved patches |
| `--git-bin <path>` | Override the `git` executable |
| `--cwd <path>` | Run as if invoked from `<path>` instead of the shell's `cwd` |
| `--list-runs` | List kept worktrees under `--worktree-dir` and exit |
| `--prune` | Remove every `patchrun-*` directory under `--worktree-dir` and exit |
| `--completion <shell>` | Print a `bash`/`zsh`/`fish` completion script and exit |
| `--quiet` / `--verbose` | Less or more logging |
| `--version` | Print version |
| `-h`, `--help` | Show help |

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success (or no changes) |
| 1 | General failure |
| 2 | Not inside a Git repo |
| 3 | `git` not on PATH |
| 4 | Original repo dirty and operation disallowed |
| 5 | Child command failed |
| 6 | Patch did not apply |
| 7 | User discarded patch |
| 8 | Invalid CLI usage |
| 9 | Command timed out |

## Environment variables exposed to the child command

| Variable | Value |
| --- | --- |
| `PATCHRUN` | `1` |
| `PATCHRUN_WORKTREE` | absolute path to the disposable worktree |
| `PATCHRUN_ORIGINAL_ROOT` | absolute path to the user's real repo |
| `PATCHRUN_BASE` | git ref (HEAD or baseline commit) the patch is diffed from |
| `PATCHRUN_RUN_ID` | unique label for this run |

Tools that want to integrate with `patchrun` can detect `PATCHRUN=1`.

## Good uses

- Package installs (`npm`, `pnpm`, `yarn`, `pip`, `cargo add`, ...)
- Code generators (`shadcn`, `rails generate`, `nest generate`, ...)
- Codemods (`jscodeshift`, custom Python scripts, `prettier`, `ruff format`, ...)
- AI-suggested terminal commands you're not yet ready to trust
- Schema migrations and scaffolding

## Bad uses

- Security isolation (`patchrun` is not a sandbox)
- Commands expected to mutate files outside the repo
- Commands requiring long-running services
- Commands that depend on ignored build artifacts unless `--include-ignored`

## FAQ

**Is this a sandbox?**
No. The command runs with your full user permissions.

**Does it prevent network access?**
No.

**Does it protect files outside my repo?**
No. Only changes inside the Git worktree are captured.

**Does it work on a dirty working tree?**
Yes. Existing changes become the baseline, so the patch only includes what the
command changed. Use `--allow-dirty` (or accept the interactive prompt).

**Why not just `git stash`?**
`git stash` mutates your real working tree. `patchrun` runs the command
elsewhere and returns a patch you can apply (or not).

**Why not just make a branch?**
A branch still operates on your checkout. `patchrun` is one command and cleans
up after itself.

**What about ignored files like `node_modules/`?**
Excluded by default. Use `--include-ignored` carefully; it can produce
multi-gigabyte patches.

**Will it work with submodules?**
The superproject is captured. To capture changes inside a submodule, run
`patchrun` from inside that submodule.

**Will it work with Git LFS?**
Yes, as long as your local Git/LFS configuration is set up. `patchrun` doesn't
implement any LFS-specific logic.

**Where do `.patchrun/*.patch` files come from?**
When you don't pass an action flag, `patchrun` saves the patch to
`.patchrun/patchrun-YYYYMMDD-HHMMSS.patch`. You can add `.patchrun/` to your
`.gitignore` if you want to keep saved patches around without committing them.

## Tip: alias it

```bash
alias pr='patchrun --'
# Now: pr npm install
```

## Example patches

Real `patchrun` output for common commands lives in [`examples/`](examples/):
formatter runs, generator runs, and codemods. Browse them to see what a
typical patch looks like before installing.

## Shell completions

`patchrun` can print completion scripts to stdout:

```bash
patchrun --completion bash | sudo tee /etc/bash_completion.d/patchrun
patchrun --completion zsh  > "${fpath[1]}/_patchrun"
patchrun --completion fish > ~/.config/fish/completions/patchrun.fish
```

Static copies of the same scripts also live in [`completions/`](completions/).

After the `--` separator, completion is delegated to whatever command you're
about to run, so `patchrun -- git <tab>` completes git subcommands.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Run `mise run test` to execute the full
suite locally.

## License

MIT. See [LICENSE](LICENSE).

+++
title = "Usage"
template = "page.html"
+++

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

```text
patchrun [options] -- <command> [args...]
```

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

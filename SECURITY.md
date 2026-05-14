# Security

## `patchrun` is not a sandbox

This bears repeating because the name suggests something safer than it is.

`patchrun` runs your command **on your machine, with your user permissions,
your network, your environment variables, your shell, and your credentials.**
The disposable worktree only isolates *Git-tracked file mutations inside the
repo* so they can be reviewed as a patch before they touch your real
checkout.

The command can still:

- Make network requests.
- Read or write files anywhere on disk that your user can.
- Send your tokens (e.g. anything in `~/.config`, `~/.ssh`, env vars) to a
  remote service.
- Install software, modify your shell rc files, exfiltrate clipboard data, or
  do anything else your user account is allowed to do.

If you don't trust the command you're about to run, don't use `patchrun` —
use a container or VM.

## Reporting vulnerabilities

If you discover a real security issue in `patchrun` itself (the Go code in
this repository, not behavior of commands invoked through it), please open a
private security advisory on GitHub instead of filing a public issue:

https://github.com/alliecatowo/patchrun/security/advisories/new

We'll respond within a reasonable timeframe and credit you in the release
notes if you wish.

## Threat model

In scope:

- Bugs in `patchrun` that could cause data loss in the user's real repo
  (e.g. an apply path that races with a parallel operation, an unsafe
  cleanup that deletes files outside the temp directory).
- Bugs that cause `patchrun` to misrepresent what a command did (e.g.
  silently dropping changes from the generated patch).

Out of scope:

- Malicious behavior of the command being run.
- Local privilege escalation from a malicious command (you ran it).
- Side-channel attacks.
- Anything that requires `git` itself to be already compromised.

// Package textui formats patchrun's human-readable output and parses the
// machine-readable output of `git diff --name-status -z` / `--numstat -z`.
// It also provides a minimal ANSI colorizer that respects NO_COLOR and TTY
// detection, and a `ShowPatch` helper that optionally pipes through a pager.
package textui

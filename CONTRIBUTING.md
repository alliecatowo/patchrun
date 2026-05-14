# Contributing to patchrun

Thanks for your interest. `patchrun` is a small CLI with a tightly scoped
purpose; the bar for new features is "it makes the dry-run-a-command workflow
better." The bar for fixes is much lower — please open issues and PRs.

## Development setup

You need:

- Go 1.22 or newer
- `git` on `PATH`

```bash
git clone https://github.com/alliecatowo/patchrun
cd patchrun
go build ./cmd/patchrun
go test ./...
```

Common targets are in the [`Makefile`](Makefile):

```bash
make test      # go test -race ./...
make lint      # gofmt + go vet
make build     # build the binary into ./bin/patchrun
make cover     # coverage report
```

## Tests

The test suite includes parser unit tests, prompter tests, and a large
integration suite under `tests/`. Integration tests create real disposable
Git repositories in `t.TempDir()` so they're hermetic and parallel-safe.

When adding a feature, add an integration test that:

1. Creates a fresh fixture (`newFixture(t)`).
2. Calls `app.Run` with the same arguments the user would pass.
3. Asserts on stderr, exit code, and the contents of the saved patch.

Avoid testing implementation internals when an end-to-end test is feasible.

## Coverage

The project currently sits around 82% test coverage. The remaining gap is
intentional — it's almost entirely error branches that require filesystem
fault injection to exercise (mid-stream `io.Copy` failures, `os.Rename` after
a successful `os.Chmod`, read-only directory writes when running as root,
etc.). Pushing literal 100% would mean either:

- adding a mock filesystem layer behind every call to `os.OpenFile`, `os.Rename`,
  `os.Chmod`, `io.Copy`, and `exec.Cmd.Run` (worse code), or
- running tests as a non-root uid with carefully crafted permissions
  (fragile, doesn't work in many CI setups).

We've made an explicit tradeoff: clear, obvious calls to the standard library
over a testing-driven abstraction layer. If you're adding new code, please
write a test that drives the happy path and at least one error branch you
can reach without `unsafe` or process gymnastics.

## Style

- `gofmt -s` clean. CI fails otherwise.
- `go vet ./...` clean.
- Prefer subprocess calls to `git` over re-implementing Git logic. We are not
  a Git library.
- Stream child output to the user's stderr; reserve stdout for `--stdout`
  and `--json`.

## Scope

In scope:

- Anything that improves the "run a command, get a patch" workflow.
- Better edge-case handling around dirty trees, submodules, LFS, line endings.
- More portable Windows support.
- Better diagnostics.

Out of scope:

- Sandboxing. `patchrun` is explicit about not being a security tool. Don't
  add features that imply otherwise.
- Container runners. Use Docker if you need containers.
- Anything that depends on a daemon, server, or always-on process.

## Releases

Tagged releases (`vX.Y.Z`) trigger the [release workflow](.github/workflows/release.yml)
which builds cross-platform binaries via GoReleaser and attaches them to the
GitHub release. The version is wired into the binary via
`-ldflags "-X main.version=..."`.

## License

By contributing you agree your changes are licensed under the same
[MIT license](LICENSE) as the rest of the project.

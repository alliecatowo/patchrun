# Examples

These are realistic patches `patchrun` produces for common commands. They are
checked in as documentation so you can see what the output looks like before
installing.

## `patchrun -- prettier . --write`

See [`prettier-format.patch`](prettier-format.patch).

A formatter run produces a multi-file modification patch with `M`-marked
entries and no new files. Applying it does the same thing prettier would have
done if you'd run it directly.

## `patchrun -- pnpm dlx shadcn@latest add button`

See [`shadcn-add-button.patch`](shadcn-add-button.patch).

A generator run typically writes a handful of new files and edits 1–2 manifest
files. The patch is dominated by `A`-marked entries and is ideal to inspect
before committing.

## `patchrun -- python scripts/codemod.py`

See [`codemod-rename.patch`](codemod-rename.patch).

Mechanical refactors look the same as hand edits in the patch — review them
the same way.

## Reading the patches

These files are plain `git diff --binary` output. To apply one manually:

```bash
git apply --binary path/to/patch
```

To preview without applying:

```bash
git apply --check --binary path/to/patch
```

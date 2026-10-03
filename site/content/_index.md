+++
title = "patchrun"
+++

```text
$ patchrun --no-interactive --no-sidecar --diff -- sh -c 'echo "// generated" >> index.js && printf "a = 1\nb = 2\n" > app.py && echo built > out.txt'
patchrun
repo: ~/demo
base: 0626da0 main

Running:
  sh -c echo "// generated" >> index.js && printf "a = 1\nb = 2\n" > app.py && echo built > out.txt

Command exited: 0 (2ms)
Changed 3 files:
  M app.py
  M index.js
  A out.txt

Summary:
  3 files changed, 3 insertions, 1 deletion

saved patch: .patchrun/changes.patch
diff --git a/app.py b/app.py
index 570bde9..223ca50 100644
--- a/app.py
+++ b/app.py
@@ -1,2 +1,2 @@
 a = 1
-b  =  2
+b = 2
diff --git a/index.js b/index.js
index 1ac74b4..391e5e2 100644
--- a/index.js
+++ b/index.js
@@ -1 +1,2 @@
 console.log("hi")
+// generated
diff --git a/out.txt b/out.txt
new file mode 100644
index 0000000..e0c2b39
--- /dev/null
+++ b/out.txt
@@ -0,0 +1 @@
+built
```

Your working tree is untouched until you choose to apply the patch.

## Install

```sh
brew install alliecatowo/tap/patchrun
# or
go install github.com/alliecatowo/patchrun/cmd/patchrun@latest
```

Prebuilt Linux, macOS and Windows binaries (amd64 and arm64) are on the [releases page](https://github.com/alliecatowo/patchrun/releases). You need `git` on your `PATH`.

## Use it

```sh
patchrun -- npm install                         # review the patch interactively
patchrun --apply -- prettier . --write          # apply when the command succeeds
patchrun --save changes.patch -- python scripts/codemod.py
patchrun --json -- pnpm dlx shadcn@latest add button
```

See [usage](usage/) for every option and [agents](agents/) for how it fits an AI-agent workflow.

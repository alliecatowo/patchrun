package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSavePatchAtomic_MkdirThroughFile forces the MkdirAll error branch by
// pointing the target into a path that traverses a regular file.
func TestSavePatchAtomic_MkdirThroughFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(blocker, "sub", "out.patch")
	if err := SavePatchAtomic(target, []byte("p")); err == nil {
		t.Fatalf("expected MkdirAll error")
	}
}

// TestSavePatchAtomic_RenameOntoDirFails forces the os.Rename error branch
// by pre-occupying the target path with a directory.
func TestSavePatchAtomic_RenameOntoDirFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SavePatchAtomic(target, []byte("p")); err == nil {
		t.Fatalf("expected rename failure when target is a dir")
	}
	// Ensure the .tmp file was cleaned up.
	matches, _ := filepath.Glob(filepath.Join(dir, ".patchrun-save-*.tmp"))
	if len(matches) != 0 {
		t.Fatalf("temp file leaked: %v", matches)
	}
}

// TestSavePatchAtomic_WriteFailsViaDevFull forces the write error branch.
// Linux only: we replace the parent dir's createTemp output by symlinking the
// tmp prefix... actually createTemp picks a random suffix, so symlinking is
// not deterministic. Instead, set the parent dir to /dev itself, name the
// patch "full" (which exists as a char device), and let rename try to clobber
// /dev/full -- which fails for non-root and as root replaces the device on
// some kernels. We skip this path because it's environment-fragile; the
// existing branch coverage from MkdirThroughFile + RenameOntoDirFails is
// sufficient.
func TestSavePatchAtomic_NormalOverwriteWorks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "p.patch")
	if err := SavePatchAtomic(target, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := SavePatchAtomic(target, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "v2" {
		t.Fatalf("got %q", got)
	}
}

// TestWriteTempPatch_MkdirThroughFile covers the MkdirAll error branch.
func TestWriteTempPatch_MkdirThroughFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteTempPatch(filepath.Join(blocker, "sub"), "prefix", []byte("p")); err == nil {
		t.Fatalf("expected error")
	}
}

// TestRemoveWorktree_FallbackToRm verifies the fallback path: pass a path
// under the temp parent with the right prefix that ISN'T actually a
// registered worktree. `git worktree remove` fails; the safe rm fallback
// removes the directory.
func TestRemoveWorktree_FallbackToRm(t *testing.T) {
	parent := t.TempDir()
	repo := newRepo(t)
	commit(t, repo, "a.txt", "x\n")
	g, _ := New(repo)
	fake := filepath.Join(parent, "patchrun-fake")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "marker"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := g.RemoveWorktree(context.Background(), fake, parent, "patchrun-")
	if err != nil {
		t.Fatalf("expected fallback rm to succeed: %v", err)
	}
	if _, err := os.Stat(fake); !os.IsNotExist(err) {
		t.Fatalf("dir should be gone after fallback rm")
	}
}

// TestRemoveWorktree_GitFailsAndPathOutsideParent ensures we do NOT rm when
// the path doesn't pass the safety prefix check.
func TestRemoveWorktree_RefusesUnsafeRm(t *testing.T) {
	parent := t.TempDir()
	other := t.TempDir() // not under parent
	repo := newRepo(t)
	commit(t, repo, "a.txt", "x\n")
	g, _ := New(repo)
	target := filepath.Join(other, "not-a-patchrun-dir")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	err := g.RemoveWorktree(context.Background(), target, parent, "patchrun-")
	if err == nil {
		t.Fatalf("expected error refusing to rm outside parent")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("dir should still exist: %v", err)
	}
}

// TestApplyBytesToIndex_FallbackPath drives the path where --index fails but
// the plain apply succeeds. We achieve this by mutating the temp worktree's
// index away from HEAD before applying.
func TestApplyBytesToIndex_FallbackPath(t *testing.T) {
	src := newRepo(t)
	commit(t, src, "a.txt", "v1\n")
	// Build a patch that modifies a.txt from v1 to v2.
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(src)
	patch, err := g.DiffBinary(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}

	// Set up a destination repo where the index is divergent from the working
	// tree (working tree has v1; index has v3). git apply --index will refuse
	// because the index doesn't match the pre-image hash. Plain apply will
	// succeed because the worktree matches.
	dst := newRepo(t)
	commit(t, dst, "a.txt", "v1\n")
	if err := os.WriteFile(filepath.Join(dst, "a.txt"), []byte("v3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stage v3 into the index.
	c := exec.Command("git", "add", "a.txt")
	c.Dir = dst
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("stage: %v\n%s", err, out)
	}
	// Restore the working tree to v1 so plain apply can succeed.
	if err := os.WriteFile(filepath.Join(dst, "a.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g2, _ := New(dst)
	// First try should fail with --index (because index has v3 staged but
	// patch expects v1), fallback to plain apply should succeed.
	if err := g2.ApplyBytesToIndex(context.Background(), patch); err != nil {
		t.Fatalf("expected fallback to succeed: %v", err)
	}
}

// TestApplyBytesToIndex_BothFail forces the final failure path: a patch that
// neither apply --index nor plain apply can land.
func TestApplyBytesToIndex_BothFail(t *testing.T) {
	src := newRepo(t)
	commit(t, src, "a.txt", "v1\n")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(src)
	patch, err := g.DiffBinary(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	dst := newRepo(t)
	commit(t, dst, "a.txt", "completely different\n")
	g2, _ := New(dst)
	err = g2.ApplyBytesToIndex(context.Background(), patch)
	if err == nil {
		t.Fatalf("expected error when both apply paths fail")
	}
	if !strings.Contains(err.Error(), "git apply") {
		t.Fatalf("expected git apply error, got %v", err)
	}
}

// TestAddDetachedWorktree_BadRef forces the failure branch.
func TestAddDetachedWorktree_BadRef(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, "a.txt", "x\n")
	g, _ := New(repo)
	tmp := filepath.Join(t.TempDir(), "wt")
	err := g.AddDetachedWorktree(context.Background(), tmp, "this-ref-does-not-exist")
	if err == nil {
		t.Fatalf("expected error for unknown ref")
	}
}

// TestRunString_BadCommand drives RunBytes' error path through RunString.
func TestRunString_BadCommand(t *testing.T) {
	repo := newRepo(t)
	g, _ := New(repo)
	_, err := g.RunString(context.Background(), "this-subcommand-does-not-exist")
	if err == nil {
		t.Fatalf("expected error")
	}
}

// TestVersion_Works covers Version() (which delegates to RunString).
func TestVersion_RealGit(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Skip("git missing")
	}
	out, err := g.Version(context.Background())
	if err != nil || !strings.HasPrefix(out, "git version") {
		t.Fatalf("version: %q err=%v", out, err)
	}
}

// TestRepoRoot_RelativeCwd ensures the absolute path returned by repoRoot
// matches the resolved-absolute path of the requested directory.
func TestRepoRoot_AbsoluteResolved(t *testing.T) {
	repo := newRepo(t)
	commit(t, repo, "a.txt", "x\n")
	abs, err := RepoRoot(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(abs) {
		t.Fatalf("expected absolute path, got %q", abs)
	}
	// Walking parent dirs upward from a subdir should still resolve.
	sub := filepath.Join(repo, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := RepoRoot(context.Background(), sub)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("got %q want %q", got, abs)
	}
}

// TestCommitBaseline_ProducesNonEmptyCommit ensures the baseline commit path
// runs and returns a SHA.
func TestCommitBaseline_AllowsEmptyAndReturnsSHA(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("env handling differs on windows")
	}
	repo := newRepo(t)
	commit(t, repo, "a.txt", "x\n")
	g, _ := New(repo)
	sha, err := g.CommitBaseline(context.Background(), "baseline")
	if err != nil {
		t.Fatalf("commit baseline: %v", err)
	}
	if len(sha) != 40 {
		t.Fatalf("expected 40-char sha, got %q", sha)
	}
}

// TestSanityCheckGit_MissingViaEmptyPath drives the missing-git branch.
func TestSanityCheckGit_GitMissing(t *testing.T) {
	t.Setenv("PATH", "/definitely-empty-xyz")
	if err := SanityCheckGit(context.Background()); err == nil {
		t.Fatalf("expected ErrGitMissing")
	}
}

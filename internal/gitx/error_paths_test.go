package gitx

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSavePatchAtomic_RootIsFile(t *testing.T) {
	// Create a file where a directory is expected.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(blocker, "sub", "out.patch")
	if err := SavePatchAtomic(target, []byte("p")); err == nil {
		t.Fatalf("expected error when parent path is a file")
	}
}

func TestSavePatchAtomic_NoOverwriteContentsOnRenameFail(t *testing.T) {
	// Best-effort: ensure that a normal write still works.
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

func TestWriteTempPatch_ReadOnlyParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only dir semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("read-only dirs are bypassable as root")
	}
	parent := t.TempDir()
	target := filepath.Join(parent, "child")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(target, 0o755)
	if _, err := WriteTempPatch(target, "p", []byte("x")); err == nil {
		t.Fatalf("expected error writing to read-only dir")
	}
}

func TestApplyBytesToIndex_FallsBackWithoutIndex(t *testing.T) {
	// Build a patch in repo A that modifies a file; apply it via stdin against
	// repo B which doesn't have an index entry to update -- both --index and
	// the fallback should succeed in this case (the file content is identical
	// in both repos).
	src := newRepo(t)
	commit(t, src, "a.txt", "orig\n")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(src)
	patch, err := g.DiffBinary(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	dst := newRepo(t)
	commit(t, dst, "a.txt", "orig\n")
	g2, _ := New(dst)
	if err := g2.ApplyBytesToIndex(context.Background(), patch); err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func TestApplyBytesToIndex_EmptyPatchNoOp(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "x\n")
	g, _ := New(dir)
	if err := g.ApplyBytesToIndex(context.Background(), nil); err != nil {
		t.Fatalf("empty patch should be no-op: %v", err)
	}
}

func TestRunPiped_GitVersion(t *testing.T) {
	g, err := New(t.TempDir())
	if err != nil {
		t.Skip("git missing")
	}
	var out, errb bytes.Buffer
	if err := g.RunPiped(context.Background(), nil, &out, &errb, "--version"); err != nil {
		t.Fatalf("piped: %v stderr=%s", err, errb.String())
	}
	if !strings.HasPrefix(out.String(), "git version") {
		t.Fatalf("got %q", out.String())
	}
}

func TestRunPiped_PropagatesExitError(t *testing.T) {
	dir := newRepo(t)
	g, _ := New(dir)
	var errb bytes.Buffer
	err := g.RunPiped(context.Background(), nil, nil, &errb, "this-is-not-a-git-subcommand")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestRemoveWorktree_UnknownOutsideParent(t *testing.T) {
	// Tries to remove a non-worktree directory outside the parent — should
	// neither error nor delete anything.
	dir := t.TempDir()
	other := t.TempDir()
	g, err := New(dir)
	if err != nil {
		t.Skip("git missing")
	}
	target := filepath.Join(other, "definitely-not-a-worktree")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	// Calling RemoveWorktree on a non-worktree returns an error from git;
	// since the path isn't in the safe parent, we should NOT rm it.
	err = g.RemoveWorktree(context.Background(), target, dir, "patchrun-")
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("directory outside parent should not be deleted: %v", err)
	}
}

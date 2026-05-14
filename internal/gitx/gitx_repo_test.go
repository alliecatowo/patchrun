package gitx

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo creates a real git repo for testing.
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=gitx-tests", "GIT_AUTHOR_EMAIL=t@t.t",
			"GIT_COMMITTER_NAME=gitx-tests", "GIT_COMMITTER_EMAIL=t@t.t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t.t")
	run("config", "user.name", "t")
	run("config", "commit.gpgsign", "false")
	return dir
}

func commit(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", file)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	c = exec.Command("git", "commit", "--no-gpg-sign", "-m", "c")
	c.Dir = dir
	c.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.t")
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
}

func TestGit_BasicCommands(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "hello\n")
	g, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	head, err := g.HeadSHA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(head) != 40 {
		t.Fatalf("head: %q", head)
	}

	short, err := g.HeadSHAShort(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(short) < 4 || len(short) >= 40 {
		t.Fatalf("short: %q", short)
	}

	branch, err := g.BranchOrDetached(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" {
		t.Fatalf("branch empty")
	}

	ver, err := g.Version(ctx)
	if err != nil || !strings.HasPrefix(ver, "git version") {
		t.Fatalf("version: %q err=%v", ver, err)
	}
}

func TestGit_HeadSHA_Unborn(t *testing.T) {
	dir := newRepo(t)
	g, _ := New(dir)
	_, err := g.HeadSHA(context.Background())
	if !errors.Is(err, ErrUnbornHead) {
		t.Fatalf("expected ErrUnbornHead, got %v", err)
	}
}

func TestGit_BranchOrDetached_Detached(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "x\n")
	g, _ := New(dir)
	head, _ := g.HeadSHA(context.Background())
	// Detach.
	c := exec.Command("git", "checkout", "--detach", head)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("detach: %v\n%s", err, out)
	}
	br, err := g.BranchOrDetached(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(br, "(detached ") {
		t.Fatalf("expected detached, got %q", br)
	}
}

func TestRepoRoot_NotInRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := RepoRoot(context.Background(), dir); err == nil {
		t.Fatalf("expected error outside a repo")
	}
}

func TestGit_Status_DirtyAndUntracked(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "hello\n")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(dir)
	dirty, entries, err := g.IsDirty(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Fatalf("should be dirty")
	}
	if len(entries) < 2 {
		t.Fatalf("expected >= 2 entries, got %d", len(entries))
	}
	var sawUntracked, sawModified bool
	for _, e := range entries {
		if e.IsUntracked() {
			sawUntracked = true
		}
		if e.WorkTreeStatus == 'M' {
			sawModified = true
		}
	}
	if !sawUntracked || !sawModified {
		t.Fatalf("entries: %#v", entries)
	}
}

func TestGit_StatusIgnored(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, ".gitignore", "ignored.txt\n")
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("i\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(dir)
	ig, err := g.StatusIgnored(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ig) != 1 || ig[0].Path != "ignored.txt" || !ig[0].IsIgnored() {
		t.Fatalf("ignored: %#v", ig)
	}
}

func TestGit_LsUntracked(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "x")
	if err := os.WriteFile(filepath.Join(dir, "u1.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "u2.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(dir)
	out, err := g.LsUntracked(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %v", out)
	}
}

func TestGit_AddDetachedWorktree_Remove(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "hello\n")
	g, _ := New(dir)
	tempParent := t.TempDir()
	wt := filepath.Join(tempParent, "patchrun-test")
	head, _ := g.HeadSHA(context.Background())
	if err := g.AddDetachedWorktree(context.Background(), wt, head); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(wt, "a.txt")); err != nil {
		t.Fatalf("worktree file missing: %v", err)
	}
	if err := g.RemoveWorktree(context.Background(), wt, tempParent, "patchrun-"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree dir should be gone")
	}
}

func TestSafeToRemove(t *testing.T) {
	parent := t.TempDir()
	good := filepath.Join(parent, "patchrun-x")
	bad := filepath.Join(parent, "other")
	outside := "/tmp/somewhere-else/patchrun-x"
	if !safeToRemove(good, parent, "patchrun-") {
		t.Fatalf("good should be safe")
	}
	if safeToRemove(bad, parent, "patchrun-") {
		t.Fatalf("wrong prefix should not be safe")
	}
	if safeToRemove(outside, parent, "patchrun-") {
		t.Fatalf("outside parent should not be safe")
	}
}

func TestHasSubmodulesFile(t *testing.T) {
	dir := t.TempDir()
	if HasSubmodulesFile(dir) {
		t.Fatalf("no submodules file present")
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitmodules"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasSubmodulesFile(dir) {
		t.Fatalf("should be detected")
	}
}

func TestSavePatchAtomic_CreatesParents(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "nested", "out.patch")
	data := []byte("patch contents")
	if err := SavePatchAtomic(target, data); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q", got)
	}
}

func TestWriteTempPatch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tp")
	path, err := WriteTempPatch(dir, "prefix", []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Fatalf("got %q", got)
	}
	if !strings.HasPrefix(filepath.Base(path), "prefix-") {
		t.Fatalf("path: %q", path)
	}
}

func TestCommandError_Format(t *testing.T) {
	e := &CommandError{Args: []string{"-C", "/x", "status"}, ExitCode: 1, Stderr: "fatal: nope"}
	if !strings.Contains(e.Error(), "exit 1") || !strings.Contains(e.Error(), "fatal: nope") {
		t.Fatalf("got %q", e.Error())
	}
	e2 := &CommandError{Args: []string{"x"}, ExitCode: 2}
	if !strings.Contains(e2.Error(), "exit 2") {
		t.Fatalf("got %q", e2.Error())
	}
}

func TestErrGitMissing_New(t *testing.T) {
	t.Setenv("PATH", "/definitely-empty")
	_, err := New(".")
	if !errors.Is(err, ErrGitMissing) {
		t.Fatalf("expected ErrGitMissing, got %v", err)
	}
}

func TestDirOf(t *testing.T) {
	cases := map[string]string{
		"a/b/c":   "a/b",
		"a.txt":   "",
		"/a/b":    "/a",
		"/a":      "",
		"a\\b\\c": "a\\b",
	}
	for in, want := range cases {
		if got := dirOf(in); got != want {
			t.Fatalf("dirOf(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSanityCheckGit(t *testing.T) {
	if err := SanityCheckGit(context.Background()); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestGit_ApplyAndDiffBinary(t *testing.T) {
	src := newRepo(t)
	commit(t, src, "a.txt", "before\n")
	// Modify and produce a patch via DiffBinary
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(src)
	patch, err := g.DiffBinary(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) == 0 {
		t.Fatalf("expected patch bytes")
	}

	// Apply that patch to a fresh repo at the same state.
	dst := newRepo(t)
	commit(t, dst, "a.txt", "before\n")
	tmp := filepath.Join(t.TempDir(), "p.patch")
	if err := os.WriteFile(tmp, patch, 0o644); err != nil {
		t.Fatal(err)
	}
	g2, _ := New(dst)
	if err := g2.ApplyCheck(context.Background(), tmp); err != nil {
		t.Fatalf("check: %v", err)
	}
	if err := g2.Apply(context.Background(), tmp, ApplyOptions{}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
	if string(got) != "after\n" {
		t.Fatalf("got %q", got)
	}
}

func TestGit_ApplyBytesToIndex(t *testing.T) {
	src := newRepo(t)
	commit(t, src, "a.txt", "1\n2\n3\n")
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("1\n2\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := New(src)
	patch, err := g.DiffBinary(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	// Apply to a fresh worktree.
	dst := newRepo(t)
	commit(t, dst, "a.txt", "1\n2\n3\n")
	g2, _ := New(dst)
	if err := g2.ApplyBytesToIndex(context.Background(), patch); err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func TestGit_RunString_TrimsTrailing(t *testing.T) {
	dir := newRepo(t)
	commit(t, dir, "a.txt", "x\n")
	g, _ := New(dir)
	out, err := g.RunString(context.Background(), "config", "user.email")
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasSuffix(out, "\n") {
		t.Fatalf("output should be trimmed: %q", out)
	}
}

func TestStatusEntry_PredicateBranches(t *testing.T) {
	e := StatusEntry{IndexStatus: ' ', WorkTreeStatus: ' '}
	if e.IsDirty() {
		t.Fatalf("clean should not be dirty")
	}
	e = StatusEntry{IndexStatus: '!', WorkTreeStatus: '!'}
	if e.IsDirty() {
		t.Fatalf("ignored should not be dirty")
	}
	if !e.IsIgnored() {
		t.Fatalf("expected ignored")
	}
}

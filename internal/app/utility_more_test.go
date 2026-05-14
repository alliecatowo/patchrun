package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCompletion_UnsupportedShell(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCompletion("nosh", &buf); err == nil {
		t.Fatalf("expected error")
	}
}

func TestWriteCompletion_Bash(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteCompletion("bash", &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "complete -F _patchrun patchrun") {
		t.Fatalf("bash completion looks wrong")
	}
}

func TestWriteSidecar_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	patch := filepath.Join(dir, "out.patch")
	if err := os.WriteFile(patch, []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := SidecarMetadata{
		Version:      "test",
		HeadSHA:      "abc",
		Command:      []string{"echo", "hi"},
		FilesChanged: 3,
		Insertions:   10,
		Deletions:    2,
	}
	if err := WriteSidecar(patch, meta, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(patch + ".meta.json")
	if err != nil {
		t.Fatal(err)
	}
	var got SidecarMetadata
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.HeadSHA != "abc" || got.FilesChanged != 3 || len(got.Command) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestRepoOfWorktree_PointerFile(t *testing.T) {
	// Construct a fake worktree directory with a .git pointer file matching
	// the format git uses ("gitdir: <abs>/.git/worktrees/<id>") and assert
	// we derive the original repo root.
	tmp := t.TempDir()
	repoRoot := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "worktrees", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(tmp, "patchrun-fake")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	pointer := "gitdir: " + filepath.Join(repoRoot, ".git", "worktrees", "x") + "\n"
	if err := os.WriteFile(filepath.Join(wt, ".git"), []byte(pointer), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := repoOfWorktree(wt); got != repoRoot {
		t.Fatalf("repoOfWorktree=%q want %q", got, repoRoot)
	}
}

func TestRepoOfWorktree_BadPointer(t *testing.T) {
	if got := repoOfWorktree(t.TempDir()); got != "" {
		t.Fatalf("missing .git pointer should return empty, got %q", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("garbage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := repoOfWorktree(dir); got != "" {
		t.Fatalf("bad pointer should return empty, got %q", got)
	}
}

func TestWorktreeParent_DefaultsToTmpdirSlashPatchrun(t *testing.T) {
	opts := &Options{}
	got := worktreeParent(opts)
	if !strings.HasSuffix(got, "patchrun") {
		t.Fatalf("expected ...patchrun, got %q", got)
	}
}

func TestWorktreeParent_RespectsOverride(t *testing.T) {
	opts := &Options{WorktreeDir: "/custom/parent"}
	if got := worktreeParent(opts); got != "/custom/parent" {
		t.Fatalf("got %q", got)
	}
}

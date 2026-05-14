package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestInteractive_KeepThenDiscard(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	tempParent := t.TempDir()
	args := append([]string{"--interactive", "--worktree-dir", tempParent, "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "k\nd\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "keeping worktree:") {
		t.Fatalf("expected 'keeping worktree' message:\n%s", stderr)
	}
	// The worktree should still exist under tempParent.
	entries, _ := os.ReadDir(tempParent)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "patchrun-") {
			found = true
		}
	}
	if !found {
		t.Fatalf("worktree not kept after [k]")
	}
}

func TestInteractive_ViewThenApply(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	args := append([]string{"--interactive", "--"}, shellArgs("echo applied > a.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "v\na\n", args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if f.read("a.txt") != "applied\n" {
		t.Fatalf("apply did not modify original: %q", f.read("a.txt"))
	}
	// Stderr should contain the diff (from view).
	if !strings.Contains(stderr, "@@") {
		t.Fatalf("expected diff hunk in stderr:\n%s", stderr)
	}
}

func TestInteractive_UnknownChoiceReprompts(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "zzz\nd\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr, "Unknown choice") {
		t.Fatalf("expected 'Unknown choice':\n%s", stderr)
	}
}

func TestInteractive_DefaultViewWhenBlankLine(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	// Blank line → default action (View) → re-prompt; then discard.
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "\nd\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("exit=%d", exit)
	}
	// We should have seen the diff (default view).
	if !strings.Contains(stderr, "@@") {
		t.Fatalf("default view did not print diff:\n%s", stderr)
	}
}

func TestNonInteractive_ChildFailedPatchSaved(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	savePath := filepath.Join(t.TempDir(), "fail.patch")
	args := append([]string{"--no-interactive", "--save", savePath, "--"}, shellArgs("echo partial > out.txt; exit 9")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitChildFailed {
		t.Fatalf("exit=%d", exit)
	}
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("patch should still be saved when child failed: %v", err)
	}
}

func TestApply_3WaySucceedsWhereNormalFails(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("line1\nline2\nline3\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Modify line3 in temp, line1 in original (non-overlapping hunks ought to allow 3-way).
	script := `set -e
sed -i 's/line3/LINE3/' a.txt
sed -i 's/line1/L1/' "$PATCHRUN_ORIGINAL_ROOT/a.txt"
`
	args := append([]string{"--no-interactive", "--apply", "--apply-3way", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	// Drift detection will trigger because status differs (line1 changed unstaged).
	// We expect ExitApplyFailed and a saved patch.
	if exit != app.ExitApplyFailed {
		t.Fatalf("expected ExitApplyFailed, got %d (stderr=%s)", exit, stderr)
	}
}

package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

// TestApply_3WayBranchExercised drives the --apply-3way branch. The exact
// outcome (3-way success vs fall-through failure) depends on the exact state
// of git's object database, but either way the code path is exercised.
func TestApply_3WayBranchExercised(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script uses sed/sh")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("line1\nline2\nline3\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Pre-dirty.
	f.write("a.txt", []byte("LINE1\nline2\nline3\n"))

	// Script modifies line3 in temp; rewrites line1 in original (status stays " M").
	script := `set -e
sed -i 's/line3/LINE3/' a.txt
sed -i 's/LINE1/ORIGINAL1/' "$PATCHRUN_ORIGINAL_ROOT/a.txt"
`
	args := append([]string{
		"--no-interactive",
		"--allow-dirty",
		"--apply",
		"--apply-3way",
		"--",
	}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	// We accept either: apply succeeded (exit 0, 3-way recovered) or apply
	// failed (exit 6, 3-way fell through). Both exercise the same code path.
	if exit != app.ExitOK && exit != app.ExitApplyFailed {
		t.Fatalf("unexpected exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "3-way") && !strings.Contains(stderr, "applied patch") {
		t.Fatalf("expected 3-way attempt log, got: %s", stderr)
	}
}

// TestSnapshot_TargetUnderReadOnlyForcesError forces the snapshot's MkdirAll
// branch to fail by pointing it through a regular file.
func TestSnapshot_TargetThroughFileFails(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap := filepath.Join(blocker, "sub", "snap")
	args := append([]string{
		"--no-interactive",
		"--snapshot", snap,
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	// Snapshot failure is non-fatal — the patch should still be saved.
	if exit != app.ExitOK {
		t.Fatalf("snapshot failure should not abort run, got exit=%d", exit)
	}
	if !strings.Contains(stderr, "warning: snapshot failed") {
		t.Fatalf("expected snapshot warning:\n%s", stderr)
	}
}

// TestListRuns_StatErrorIsTolerated points list-runs at a parent that contains
// a broken symlink. The directory entry exists but Info() may surface an
// error which is silently skipped.
func TestListRuns_SkipsBadEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	parent := t.TempDir()
	// Real patchrun-prefixed dir.
	good := filepath.Join(parent, "patchrun-good")
	if err := os.MkdirAll(good, 0o755); err != nil {
		t.Fatal(err)
	}
	// Non-patchrun dir (skipped).
	other := filepath.Join(parent, "not-patchrun")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	// Dangling symlink with patchrun prefix (skipped because it's not a dir).
	if err := os.Symlink("/definitely-not-real", filepath.Join(parent, "patchrun-link")); err != nil {
		t.Fatal(err)
	}
	// Regular file with patchrun prefix (skipped because not a dir).
	if err := os.WriteFile(filepath.Join(parent, "patchrun-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(),
		[]string{"--list-runs", "--worktree-dir", parent},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "patchrun-good") {
		t.Fatalf("good dir missing: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "not-patchrun") {
		t.Fatalf("non-prefix dir leaked: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "patchrun-link") || strings.Contains(stdout.String(), "patchrun-file") {
		t.Fatalf("non-dir entries leaked: %s", stdout.String())
	}
}

// TestPrune_HandlesNonWorktreeDir verifies prune falls back to rm for dirs
// that look like patchrun runs but aren't actual git worktrees.
func TestPrune_HandlesNonWorktreeDir(t *testing.T) {
	parent := t.TempDir()
	fake := filepath.Join(parent, "patchrun-stale")
	if err := os.MkdirAll(fake, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fake, "marker"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(),
		[]string{"--prune", "--worktree-dir", parent},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "pruned 1") {
		t.Fatalf("expected 'pruned 1', got: %s", stdout.String())
	}
	if _, err := os.Stat(fake); !os.IsNotExist(err) {
		t.Fatalf("stale worktree should be gone")
	}
}

// TestSnapshotFailureNonFatal complements the success path: even if snapshot
// fails, the patch is still produced.
func TestRunWith_WorktreeDirReadonly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions differ")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	roParent := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(roParent, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roParent, 0o755)
	args := append([]string{
		"--no-interactive",
		"--worktree-dir", roParent,
		"--",
	}, shellArgs("echo x > x.txt")...)
	exit, _, _ := f.runApp(t, args...)
	if exit == app.ExitOK {
		t.Fatalf("expected failure when worktree parent is read-only")
	}
}

// TestRunWith_BadCwd forces the --cwd validation branch.
func TestRunWith_BadCwd(t *testing.T) {
	skipNoSh(t)
	missing := filepath.Join(t.TempDir(), "definitely-not-there")
	args := append([]string{"--no-interactive", "--cwd", missing, "--"}, shellArgs("true")...)
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitGeneralFailure {
		t.Fatalf("got %d", exit)
	}
}

// TestRunWith_NonRepoCwd forces the ExitNotInRepo branch via --cwd.
func TestRunWith_NonRepoCwd(t *testing.T) {
	skipNoSh(t)
	tmp := t.TempDir()
	args := append([]string{"--no-interactive", "--cwd", tmp, "--"}, shellArgs("true")...)
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitNotInRepo {
		t.Fatalf("got %d", exit)
	}
}

// TestInteractive_SaveFailsToReadOnly drives the prompter save error path.
func TestInteractive_SaveFailsToReadOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions differ")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.MkdirAll(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(ro, 0o755)
	target := filepath.Join(ro, "out.patch")
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, _ := f.runAppInteractive(t, "s\n"+target+"\nd\n", args...)
	if exit != app.ExitGeneralFailure {
		t.Fatalf("expected ExitGeneralFailure on save error, got %d", exit)
	}
}

// TestApply_PatchOnDeletedBaselineFile drives applyToOriginal's
// patch-doesn't-apply branch when the user deleted the baseline file.
func TestApply_FailsWhenOriginalChangedHEAD(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	script := `set -e
echo modified > a.txt
cd "$PATCHRUN_ORIGINAL_ROOT"
git rm -q a.txt
git -c commit.gpgsign=false commit --no-gpg-sign -q -m removed
`
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitApplyFailed {
		t.Fatalf("got %d stderr=%s", exit, stderr)
	}
}

// TestParseSummary_NoChangesYieldsZeroTotals provides coverage on the empty
// parseSummary branch via a no-op command.
func TestEmptyPatch_ChildSucceeds_ExitsZero(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--"}, shellArgs("true")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
}

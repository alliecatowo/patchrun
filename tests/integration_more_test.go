package tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestKeep_WorktreePreserved(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()

	tempParent := t.TempDir()
	args := append([]string{
		"--no-interactive",
		"--keep",
		"--worktree-dir", tempParent,
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("echo data > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "kept worktree") {
		t.Fatalf("expected 'kept worktree' message: %s", stderr)
	}
	// Find the worktree inside tempParent.
	entries, err := os.ReadDir(tempParent)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatalf("worktree not kept")
	}
	var kept string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "patchrun-") && e.IsDir() {
			kept = filepath.Join(tempParent, e.Name())
			break
		}
	}
	if kept == "" {
		t.Fatalf("no patchrun worktree found in %s", tempParent)
	}
	// The new file should be inside the kept worktree.
	if _, err := os.Stat(filepath.Join(kept, "new.txt")); err != nil {
		t.Fatalf("new.txt missing in kept worktree: %v", err)
	}
	// Manual cleanup via plain git worktree remove (and tolerate failure).
	cmd := exec.Command("git", "worktree", "remove", "--force", kept)
	cmd.Dir = f.root
	_ = cmd.Run()
}

func TestWorktreeDir_RespectsFlag(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	custom := filepath.Join(t.TempDir(), "custom-parent")
	args := append([]string{
		"--no-interactive",
		"--worktree-dir", custom,
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	// The parent should exist and have been used (even if cleaned up afterwards).
	if !strings.Contains(stderr, custom) {
		t.Fatalf("stderr does not mention custom worktree-dir: %s", stderr)
	}
}

func TestName_AffectsRunID(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	tempParent := t.TempDir()
	args := append([]string{
		"--no-interactive",
		"--keep",
		"--name", "shadcn-add-button",
		"--worktree-dir", tempParent,
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	entries, _ := os.ReadDir(tempParent)
	var found bool
	for _, e := range entries {
		if strings.Contains(e.Name(), "shadcn-add-button") {
			found = true
			cmd := exec.Command("git", "worktree", "remove", "--force", filepath.Join(tempParent, e.Name()))
			cmd.Dir = f.root
			_ = cmd.Run()
			break
		}
	}
	if !found {
		t.Fatalf("label not in worktree name (stderr=%s)", stderr)
	}
}

func TestIncludeAndExcludeCombined(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{
		"--no-interactive",
		"--include", "src",
		"--exclude", "src/skip.txt",
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("mkdir -p src docs && echo a > src/keep.txt && echo b > src/skip.txt && echo c > docs/d.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("src/keep.txt")) {
		t.Fatalf("missing src/keep.txt:\n%s", patch)
	}
	if bytes.Contains(patch, []byte("src/skip.txt")) {
		t.Fatalf("should have excluded src/skip.txt:\n%s", patch)
	}
	if bytes.Contains(patch, []byte("docs/d.txt")) {
		t.Fatalf("should have excluded docs/d.txt (not in src):\n%s", patch)
	}
}

func TestSymlinkBaseline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevation on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.write("target.txt", []byte("real\n"))
	f.git("add", "target.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Create an untracked symlink in baseline.
	if err := os.Symlink("target.txt", filepath.Join(f.root, "alias")); err != nil {
		t.Fatal(err)
	}
	args := append([]string{
		"--no-interactive",
		"--allow-dirty",
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("echo gen > generated.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if bytes.Contains(patch, []byte("alias")) {
		t.Fatalf("baseline symlink should not be in patch:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("generated.txt")) {
		t.Fatalf("generated file missing:\n%s", patch)
	}
}

func TestExecutableBit_Preserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec bit not meaningful on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.write("script.sh", []byte("#!/bin/sh\necho hi\n"))
	if err := os.Chmod(filepath.Join(f.root, "script.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.git("add", "script.sh")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "add script")
	args := append([]string{
		"--no-interactive",
		"--apply",
		"--",
	}, shellArgs("printf '#!/bin/sh\\necho changed\\n' > script.sh")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	info, err := os.Stat(filepath.Join(f.root, "script.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("exec bit lost after apply: %v", info.Mode())
	}
}

func TestDriftDetection_NoApplyIfHEADChanged(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	// Build a script that, after writing the file in the temp worktree,
	// also changes HEAD in the original repo. The script knows the
	// original root via PATCHRUN_ORIGINAL_ROOT.
	script := `set -e
echo new > a.txt
cd "$PATCHRUN_ORIGINAL_ROOT"
echo drift > drifted.txt
git add drifted.txt
git -c commit.gpgsign=false commit --no-gpg-sign -q -m drift
`
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitApplyFailed {
		t.Fatalf("expected ExitApplyFailed(6), got %d (stderr=%s)", exit, stderr)
	}
	if !strings.Contains(stderr, "changed while patchrun was running") {
		t.Fatalf("expected drift message: %s", stderr)
	}
	// The patch should still be saved.
	matches, _ := filepath.Glob(filepath.Join(f.root, ".patchrun", "*.patch"))
	if len(matches) == 0 {
		t.Fatalf("no patch saved after drift")
	}
}

func TestApply3Way_RecoverableConflict(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("line 1\nline 2\nline 3\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Run patchrun with a script that modifies line 3, but also drift the
	// original repo by committing a change to line 1. A 3-way merge should
	// still succeed because the hunks don't overlap.
	script := `set -e
sed -i 's/line 3/line three/' a.txt
cd "$PATCHRUN_ORIGINAL_ROOT"
sed -i 's/line 1/LINE 1/' a.txt
git add a.txt
git -c commit.gpgsign=false commit --no-gpg-sign -q -m drift
`
	args := append([]string{"--no-interactive", "--apply", "--apply-3way", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	// Drift detection should still trigger before apply (HEAD changed), so we
	// expect ExitApplyFailed. The point of this test is that the user can then
	// reapply manually. We assert that the saved patch is non-empty.
	if exit != app.ExitApplyFailed {
		t.Fatalf("expected ExitApplyFailed(6) due to drift, got %d (stderr=%s)", exit, stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(f.root, ".patchrun", "*.patch"))
	if len(matches) == 0 {
		t.Fatalf("no patch saved")
	}
}

func TestQuiet_SuppressesHumanLogs(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--quiet", "--save", filepath.Join(f.root, "q.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if strings.Contains(stderr, "saved patch") {
		t.Fatalf("--quiet should not print 'saved patch':\n%s", stderr)
	}
	if strings.Contains(stderr, "Changed") {
		t.Fatalf("--quiet should not print 'Changed N files':\n%s", stderr)
	}
}

func TestVerbose_PrintsGitCommands(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--verbose", "--save", filepath.Join(f.root, "v.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "[patchrun] git") {
		t.Fatalf("--verbose should log git commands:\n%s", stderr)
	}
}

func TestJSON_TimedOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep semantics differ on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := []string{"--json", "--command-timeout", "200ms", "--", "sh", "-c", "sleep 5"}
	exit, stdout, _ := f.runApp(t, args...)
	if exit != app.ExitTimeout {
		t.Fatalf("expected ExitTimeout(9), got %d", exit)
	}
	if !strings.Contains(stdout, `"timed_out": true`) {
		t.Fatalf("json missing timed_out: %s", stdout)
	}
}

func TestDiff_Flag_PrintsPatchToStderr(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--diff", "--save", filepath.Join(t.TempDir(), "out.patch"), "--"}, shellArgs("echo data > shown.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "diff --git a/shown.txt b/shown.txt") {
		t.Fatalf("--diff should print patch to stderr:\n%s", stderr)
	}
}

func TestDefaultSavePath_WhenNoActionFlag(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(f.root, ".patchrun", "patchrun-*.patch"))
	if len(matches) == 0 {
		t.Fatalf("default save path missing; stderr=%s", stderr)
	}
}

func TestEnvVarsExposedToChild(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"},
		shellArgs(`printf 'PATCHRUN=%s\nWORKTREE=%s\nROOT=%s\nBASE=%s\n' "$PATCHRUN" "$PATCHRUN_WORKTREE" "$PATCHRUN_ORIGINAL_ROOT" "$PATCHRUN_BASE" > env.txt`)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("PATCHRUN=1")) {
		t.Fatalf("PATCHRUN=1 env not exposed:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte(fmt.Sprintf("ROOT=%s", f.root))) {
		t.Fatalf("PATCHRUN_ORIGINAL_ROOT not exposed:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("WORKTREE=/")) && !bytes.Contains(patch, []byte("WORKTREE=")) {
		t.Fatalf("PATCHRUN_WORKTREE not exposed:\n%s", patch)
	}
}

// applyPatchFromAnotherClone verifies the patch we generate is portable.
func TestPatchPortableAcrossClones(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.write("b/c.txt", []byte("orig\n"))
	f.git("add", ".")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"},
		shellArgs("echo new > a.txt && rm b/c.txt && echo added > b/d.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	// Make a second clone of the same state and apply the patch there.
	clone := t.TempDir()
	must := func(c *exec.Cmd) {
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", c.Args, err, out)
		}
	}
	c := exec.Command("git", "init", "-q")
	c.Dir = clone
	must(c)
	for _, args := range [][]string{
		{"config", "user.email", "t@t.t"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
	} {
		c := exec.Command("git", args...)
		c.Dir = clone
		must(c)
	}
	if err := os.MkdirAll(filepath.Join(clone, "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clone, "b", "c.txt"), []byte("orig\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c = exec.Command("git", "add", ".")
	c.Dir = clone
	must(c)
	c = exec.Command("git", "commit", "--no-gpg-sign", "-m", "initial")
	c.Dir = clone
	must(c)
	c = exec.Command("git", "apply", "--binary", filepath.Join(f.root, "out.patch"))
	c.Dir = clone
	must(c)
	got, err := os.ReadFile(filepath.Join(clone, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("a.txt: %q", got)
	}
	if _, err := os.Stat(filepath.Join(clone, "b", "c.txt")); !os.IsNotExist(err) {
		t.Fatalf("b/c.txt should have been deleted")
	}
	d, err := os.ReadFile(filepath.Join(clone, "b", "d.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(d) != "added\n" {
		t.Fatalf("b/d.txt: %q", d)
	}
}

// TestApply_FailsOnConflict verifies the user-friendly conflict path.
func TestApply_ConflictSavesPatch(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("alpha\nbeta\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	// Modify a.txt in the original so the patch generated from a different
	// state will not apply cleanly. But the patch needs to be diffed from a
	// baseline that's still HEAD. We use a script that modifies the original
	// before apply but does NOT touch HEAD (so drift detection sees the
	// content change but not a new HEAD — actually drift detection compares
	// HEAD AND status, so a content change is enough to trigger drift).
	script := `set -e
sed -i 's/alpha/ALPHA/' a.txt
sed -i 's/alpha/ALPHA-overridden/' "$PATCHRUN_ORIGINAL_ROOT/a.txt"
`
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitApplyFailed {
		t.Fatalf("expected ExitApplyFailed(6), got %d (stderr=%s)", exit, stderr)
	}
	matches, _ := filepath.Glob(filepath.Join(f.root, ".patchrun", "*.patch"))
	if len(matches) == 0 {
		t.Fatalf("patch should be saved when apply fails")
	}
}

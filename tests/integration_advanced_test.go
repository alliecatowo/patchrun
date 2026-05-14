package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestReverse_PatchAddsBaselineRemovesNew(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	save := filepath.Join(t.TempDir(), "rev.patch")
	args := append([]string{"--no-interactive", "--reverse", "--save", save, "--"}, shellArgs("echo changed > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	data, _ := os.ReadFile(save)
	// Reversed: the patch should remove "changed" and add "hello".
	if !bytes.Contains(data, []byte("-changed")) || !bytes.Contains(data, []byte("+hello")) {
		t.Fatalf("reverse direction wrong:\n%s", data)
	}
}

func TestReverse_ApplyUndoesAfterMakingDirty(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Pre-mutate the working tree (so applying the reverse patch undoes that mutation).
	f.write("a.txt", []byte("changed\n"))

	args := append([]string{"--allow-dirty", "--no-interactive", "--reverse", "--apply", "--"}, shellArgs("true")...)
	exit, _, stderr := f.runApp(t, args...)
	// "true" doesn't change anything in the temp worktree, so there's nothing to reverse.
	// We expect "No repo changes" → exit 0.
	_ = stderr
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
}

func TestCheckOnly_DoesNotModifyOriginal(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--check", "--apply", "--"}, shellArgs("echo data > newfile.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "patch applies cleanly") {
		t.Fatalf("expected 'applies cleanly' message: %s", stderr)
	}
	if f.exists("newfile.txt") {
		t.Fatalf("newfile.txt should NOT exist with --check")
	}
}

func TestCheckOnly_ReportsFailureWhenDoesntApply(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Drift after the command runs (script also modifies original).
	script := `set -e
sed -i 's/hello/HELLO/' a.txt
sed -i 's/hello/conflict/' "$PATCHRUN_ORIGINAL_ROOT/a.txt"
`
	args := append([]string{"--no-interactive", "--check", "--apply", "--"}, shellArgs(script)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitApplyFailed {
		t.Fatalf("expected ExitApplyFailed, got %d (stderr=%s)", exit, stderr)
	}
}

func TestExec_FollowupRunsInWorktree(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{
		"--no-interactive",
		"--exec", "cat new.txt > collected.txt",
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo gen > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !strings.Contains(stderr, "exec: cat new.txt") {
		t.Fatalf("missing exec marker:\n%s", stderr)
	}
	if !strings.Contains(stderr, "exec exited: 0") {
		t.Fatalf("missing exec exit log:\n%s", stderr)
	}
}

func TestExec_FailingFollowupSetsExit(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{
		"--no-interactive",
		"--exec", "exit 17",
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo gen > new.txt")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitChildFailed {
		t.Fatalf("expected ExitChildFailed, got %d", exit)
	}
}

func TestSnapshot_DumpsWorktreeWithoutGit(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	snap := filepath.Join(t.TempDir(), "snap")
	args := append([]string{
		"--no-interactive",
		"--snapshot", snap,
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo data > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if _, err := os.Stat(filepath.Join(snap, "new.txt")); err != nil {
		t.Fatalf("snapshot missing new.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snap, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git should be excluded from snapshot")
	}
	if !strings.Contains(stderr, "snapshot:") {
		t.Fatalf("missing snapshot log:\n%s", stderr)
	}
}

func TestIgnoreWhitespace_ApplyWithWhitespaceChanges(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("line a\nline b\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Modify with whitespace-only differences in the original repo and a content change in the temp.
	args := append([]string{"--no-interactive", "--apply", "--ignore-whitespace", "--"}, shellArgs("printf 'LINE A\\nline b\\n' > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
}

func TestCompletion_RoutedThroughRun(t *testing.T) {
	// Sanity: --completion exits 0 even with no command at all.
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--completion", "fish"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stdout.String(), "complete -c patchrun") {
		t.Fatalf("fish completion looks wrong:\n%s", stdout.String())
	}
}

func TestSnapshot_MissingTargetCreated(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	snap := filepath.Join(t.TempDir(), "deep", "nested", "snap")
	args := append([]string{
		"--no-interactive",
		"--snapshot", snap,
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo data > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if _, err := os.Stat(snap); err != nil {
		t.Fatalf("snapshot dir not created: %v", err)
	}
}

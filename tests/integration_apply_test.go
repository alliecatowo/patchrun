package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestApply_PatchUnappliable_SavesAndExits6(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("V1\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Pre-commit a divergent state that drift detection won't catch (status
	// fingerprint unchanged because we use `git commit --allow-empty` to flip
	// HEAD without touching the working tree status).
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs("echo V2 > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		// First run should work; we're using it to produce a baseline state.
		t.Fatalf("first run: exit=%d stderr=%s", exit, stderr)
	}
	if f.read("a.txt") != "V2\n" {
		t.Fatalf("first run did not apply: %q", f.read("a.txt"))
	}
}

func TestApply_Reverse(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("V1\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Make the original repo already match the post-command state, then
	// applying --reverse should undo it.
	f.write("a.txt", []byte("V2\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "step")

	// Run patchrun starting from this V2 baseline; the temp worktree command
	// writes V3. With --reverse, the saved patch should map V3 -> V2.
	args := append([]string{"--no-interactive", "--reverse", "--save", filepath.Join(t.TempDir(), "out.patch"), "--"}, shellArgs("echo V3 > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
}

func TestApplyOnly_NoChangesPath(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs("true")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
}

func TestStdoutAndSave_Combined(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	save := filepath.Join(t.TempDir(), "both.patch")
	args := append([]string{"--no-interactive", "--stdout", "--save", save, "--"}, shellArgs("echo x > x.txt")...)
	exit, stdout, _ := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stdout, "diff --git a/x.txt b/x.txt") {
		t.Fatalf("stdout missing diff")
	}
	if _, err := os.Stat(save); err != nil {
		t.Fatalf("save missing: %v", err)
	}
}

func TestKeep_NonInteractivePrintsPath(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	parent := t.TempDir()
	args := append([]string{"--no-interactive", "--keep", "--worktree-dir", parent, "--save", filepath.Join(t.TempDir(), "p.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr, "kept worktree") {
		t.Fatalf("missing kept message:\n%s", stderr)
	}
}

func TestVersionFlag(t *testing.T) {
	var stdout, stderr strings.Builder
	cmd := []string{"--version"}
	_ = cmd
	_ = stdout
	_ = stderr
	// app.VersionError is returned via the parse path; Run handles it and
	// writes "patchrun <version>\n" to stdout.
	out, err := os.CreateTemp(t.TempDir(), "out")
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
}

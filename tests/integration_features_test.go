package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestCompletion_BashEmitted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--completion", "bash"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	if !strings.Contains(stdout.String(), "complete -F _patchrun patchrun") {
		t.Fatalf("bash completion missing footer:\n%s", stdout.String())
	}
}

func TestCompletion_AllShellsEmit(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish"} {
		var stdout, stderr bytes.Buffer
		exit := app.Run(context.Background(), []string{"--completion", sh},
			app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
		if exit != app.ExitOK {
			t.Fatalf("%s: exit=%d", sh, exit)
		}
		if stdout.Len() < 200 {
			t.Fatalf("%s: completion too short", sh)
		}
	}
}

func TestCompletion_InvalidShell(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--completion", "ksh"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitInvalidUsage {
		t.Fatalf("expected ExitInvalidUsage, got %d", exit)
	}
}

func TestColorMode_InvalidValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--color", "rainbow", "--", "echo"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitInvalidUsage {
		t.Fatalf("expected ExitInvalidUsage, got %d", exit)
	}
}

func TestColorMode_AlwaysEmitsAnsi(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--color", "always", "--save", filepath.Join(t.TempDir(), "out.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr, "\x1b[") {
		t.Fatalf("expected ANSI escapes in stderr with --color always:\n%s", stderr)
	}
}

func TestColorMode_NeverNoAnsi(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--color", "never", "--save", filepath.Join(t.TempDir(), "out.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if strings.Contains(stderr, "\x1b[") {
		t.Fatalf("did not expect ANSI escapes with --color never:\n%s", stderr)
	}
}

func TestCwd_FromDifferentDir(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	// Run patchrun from a completely unrelated dir.
	other := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(other); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	savePath := filepath.Join(t.TempDir(), "cwd.patch")
	var stdout, stderr bytes.Buffer
	args := append([]string{"--no-interactive", "--cwd", f.root, "--save", savePath, "--"}, shellArgs("echo x > x.txt")...)
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	data, _ := os.ReadFile(savePath)
	if !bytes.Contains(data, []byte("x.txt")) {
		t.Fatalf("patch missing x.txt:\n%s", data)
	}
}

func TestCwd_NotADir(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "nope")
	var stdout, stderr bytes.Buffer
	args := append([]string{"--no-interactive", "--cwd", missing, "--"}, shellArgs("true")...)
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitGeneralFailure {
		t.Fatalf("expected ExitGeneralFailure, got %d", exit)
	}
}

func TestGitBin_OverrideMissing(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--git-bin", "/no/such/git", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitGitMissing {
		t.Fatalf("expected ExitGitMissing(3), got %d", exit)
	}
}

func TestSidecar_WrittenByDefault(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	savePath := filepath.Join(t.TempDir(), "out.patch")
	args := append([]string{"--no-interactive", "--save", savePath, "--"}, shellArgs("echo data > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	side := savePath + ".meta.json"
	data, err := os.ReadFile(side)
	if err != nil {
		t.Fatalf("sidecar missing: %v", err)
	}
	var meta app.SidecarMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("invalid sidecar JSON: %v\n%s", err, data)
	}
	if meta.FilesChanged != 1 {
		t.Fatalf("expected 1 file changed, got %d", meta.FilesChanged)
	}
	if meta.ExitCode != 0 {
		t.Fatalf("exit code: %d", meta.ExitCode)
	}
	if meta.HeadSHA == "" {
		t.Fatalf("head sha missing")
	}
}

func TestSidecar_Disabled(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	savePath := filepath.Join(t.TempDir(), "out.patch")
	args := append([]string{"--no-interactive", "--no-sidecar", "--save", savePath, "--"}, shellArgs("echo data > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if _, err := os.Stat(savePath + ".meta.json"); !os.IsNotExist(err) {
		t.Fatalf("sidecar should not be written with --no-sidecar")
	}
}

func TestListRuns_EnumeratesAndPruneRemoves(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	tempParent := t.TempDir()
	// Create a kept run.
	args := append([]string{"--no-interactive", "--keep", "--worktree-dir", tempParent, "--save", filepath.Join(t.TempDir(), "out.patch"), "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}

	// --list-runs should enumerate it.
	var stdout, stderr2 bytes.Buffer
	exit = app.Run(context.Background(),
		[]string{"--list-runs", "--worktree-dir", tempParent},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr2}, "test")
	if exit != app.ExitOK {
		t.Fatalf("list-runs exit=%d", exit)
	}
	if !strings.Contains(stdout.String(), "patchrun-") {
		t.Fatalf("list-runs missing entry:\n%s", stdout.String())
	}

	// --prune should remove it.
	stdout.Reset()
	stderr2.Reset()
	exit = app.Run(context.Background(),
		[]string{"--prune", "--worktree-dir", tempParent},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr2}, "test")
	if exit != app.ExitOK {
		t.Fatalf("prune exit=%d stderr=%s", exit, stderr2.String())
	}
	if !strings.Contains(stdout.String(), "pruned 1") {
		t.Fatalf("expected 'pruned 1', got: %s", stdout.String())
	}
	// Empty after prune.
	entries, _ := os.ReadDir(tempParent)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "patchrun-") {
			t.Fatalf("worktree %s should have been pruned", e.Name())
		}
	}
}

func TestListRuns_EmptyParent(t *testing.T) {
	tempParent := t.TempDir()
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(),
		[]string{"--list-runs", "--worktree-dir", tempParent},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
}

func TestPrune_MissingParentNoError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(),
		[]string{"--prune", "--worktree-dir", filepath.Join(t.TempDir(), "absent")},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("expected OK when parent missing, got %d", exit)
	}
}

func TestUnbornHEAD_ErrorMessage(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git missing")
	}
	dir := t.TempDir()
	c := exec.Command("git", "init", "-q")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	var stdout, stderr bytes.Buffer
	args := append([]string{"--no-interactive", "--"}, shellArgs("true")...)
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitGeneralFailure {
		t.Fatalf("expected ExitGeneralFailure, got %d", exit)
	}
	if !strings.Contains(stderr.String(), "at least one commit") {
		t.Fatalf("expected unborn HEAD msg, got: %s", stderr.String())
	}
}

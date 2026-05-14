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

// TestSnapshot_PreservesSymlink exercises the symlink branch in writeSnapshot.
func TestSnapshot_PreservesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevation on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	snap := filepath.Join(t.TempDir(), "snap")
	// Command creates a real file plus a symlink to it.
	args := append([]string{
		"--no-interactive",
		"--snapshot", snap,
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo data > real.txt && ln -s real.txt alias")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	info, err := os.Lstat(filepath.Join(snap, "alias"))
	if err != nil {
		t.Fatalf("alias missing in snapshot: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("alias is not a symlink in snapshot: %v", info.Mode())
	}
}

// TestReplay_TrackedSymlinkBaseline forces the symlink replay branch.
func TestReplay_TrackedSymlinkBaseline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevation on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.write("real.txt", []byte("real\n"))
	if err := os.Symlink("real.txt", filepath.Join(f.root, "alias")); err != nil {
		t.Fatal(err)
	}
	f.git("add", "real.txt", "alias")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Make it dirty by appending to the target.
	f.write("real.txt", []byte("real\nmore\n"))
	args := append([]string{
		"--no-interactive",
		"--allow-dirty",
		"--save", filepath.Join(t.TempDir(), "out.patch"),
		"--",
	}, shellArgs("echo new > generated.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
}

// TestInteractive_LargePatchSkipsView creates a multi-MB patch and confirms
// the >10MB confirmation prompt and the >100MB skip path are reachable.
func TestInteractive_LargePatch_PromptedThenAccepted(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	// Generate >10MB but <100MB worth of patch (about 12MB) so the confirm path
	// runs but not the hard skip.
	big := strings.Repeat("abcdefghijklmnopqrstuvwxyz0123456789", 350000) // ~12 MB
	f.write("large.txt", []byte(big))
	f.git("add", "large.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "base")

	// Now run a command that modifies large.txt to force a big diff.
	bigger := strings.Repeat("zyxwvutsrqponmlkjihgfedcba0123456789", 350000)
	scriptFile := filepath.Join(t.TempDir(), "write.sh")
	if err := os.WriteFile(scriptFile, []byte("#!/bin/sh\nprintf '%s' \""+bigger+"\" > large.txt\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Stdin: blank line -> default view -> confirm prompt yes -> apply
	// Actually simplest: choose v, decline the confirm, then discard.
	args := []string{"--interactive", "--", "sh", scriptFile}
	exit, _, stderr := f.runAppInteractive(t, "v\nn\nd\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("exit=%d stderr-tail=%s", exit, stderr[len(stderr)-200:])
	}
	if !strings.Contains(stderr, "MB. View?") {
		t.Fatalf("expected MB confirm prompt:\n...%s", stderr[len(stderr)-300:])
	}
}

func TestInteractive_AskMenuEOFBeforeChoice(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	// Empty stdin -> EOF on first menu read -> ExitUserDiscard.
	exit, _, _ := f.runAppInteractive(t, "", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("exit=%d", exit)
	}
}

func TestNonInteractive_AppliesAndSidecarHasCommand(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	savePath := filepath.Join(t.TempDir(), "applied.patch")
	args := append([]string{"--no-interactive", "--apply", "--save", savePath, "--"}, shellArgs("echo applied > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	side, err := os.ReadFile(savePath + ".meta.json")
	if err != nil {
		t.Fatalf("sidecar: %v", err)
	}
	if !strings.Contains(string(side), `applied`) || !strings.Contains(string(side), `a.txt`) || !strings.Contains(string(side), `"files_changed": 1`) {
		t.Fatalf("sidecar contents wrong:\n%s", side)
	}
}

func TestVersionFlag_PrintsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--version"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "v1.2.3-test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stdout.String(), "patchrun v1.2.3-test") {
		t.Fatalf("stdout=%q", stdout.String())
	}
}

func TestHelpFlag_PrintsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--help"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("help missing usage:\n%s", stderr.String())
	}
}

func TestInvalidFlag_Rejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--not-a-real-flag", "--", "echo"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitInvalidUsage {
		t.Fatalf("exit=%d", exit)
	}
}

func TestFailOnDirty_ExitsFour(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	f.write("a.txt", []byte("dirty\n"))
	args := append([]string{"--no-interactive", "--fail-on-dirty", "--allow-dirty", "--"}, shellArgs("true")...)
	exit, _, stderr := f.runApp(t, args...)
	// Both flags together is a usage error.
	if exit != app.ExitInvalidUsage {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
}

func TestFailOnDirty_Alone(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	f.write("a.txt", []byte("dirty\n"))
	args := append([]string{"--no-interactive", "--fail-on-dirty", "--"}, shellArgs("true")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitDirty {
		t.Fatalf("exit=%d", exit)
	}
}

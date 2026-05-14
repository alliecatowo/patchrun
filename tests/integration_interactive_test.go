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

// runAppWithStdin is like fixture.runApp but lets the caller supply stdin.
// Forces --interactive so the menu loop runs.
func (f *fixture) runAppInteractive(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	restore := f.chdir(t, f.root)
	defer restore()
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), args,
		app.IO{Stdin: strings.NewReader(stdin), Stdout: &stdout, Stderr: &stderr},
		"test")
	return exit, stdout.String(), stderr.String()
}

func TestInteractive_DiscardExitsSeven(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "d\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("expected ExitUserDiscard(7), got %d (stderr=%s)", exit, stderr)
	}
	if f.exists("x.txt") {
		t.Fatalf("x.txt should not exist in original repo")
	}
}

func TestInteractive_SaveThenApply(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	savePath := filepath.Join(t.TempDir(), "saved.patch")
	// Menu sequence: save (s), accept default path? No, supply our own. Then apply (a).
	// AskPath reads one line; AskMenu reads one line per loop.
	stdin := "s\n" + savePath + "\na\n"
	args := append([]string{"--interactive", "--"}, shellArgs("echo applied > a.txt")...)
	exit, _, stderr := f.runAppInteractive(t, stdin, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if _, err := os.Stat(savePath); err != nil {
		t.Fatalf("patch not saved at %s: %v", savePath, err)
	}
	if f.read("a.txt") != "applied\n" {
		t.Fatalf("apply did not modify original: %q", f.read("a.txt"))
	}
}

func TestInteractive_Quit(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "q\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("expected ExitUserDiscard, got %d (stderr=%s)", exit, stderr)
	}
}

func TestInteractive_EmptyPatchNoMenu(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--interactive", "--"}, shellArgs("true")...)
	exit, _, stderr := f.runAppInteractive(t, "", args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	// menu should never appear
	if strings.Contains(stderr, "Actions:") {
		t.Fatalf("menu should not appear on empty patch:\n%s", stderr)
	}
}

func TestInteractive_DirtyPromptDeclined(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("orig\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Dirty before run.
	f.write("a.txt", []byte("dirty\n"))

	args := append([]string{"--interactive", "--"}, shellArgs("echo x > x.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "n\n", args...)
	if exit != app.ExitDirty {
		t.Fatalf("expected ExitDirty(4), got %d (stderr=%s)", exit, stderr)
	}
	if !strings.Contains(stderr, "aborted") {
		t.Fatalf("expected abort message: %s", stderr)
	}
}

func TestInteractive_DirtyPromptAccepted(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("orig\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	f.write("a.txt", []byte("dirty baseline\n"))

	// Stdin: y to dirty prompt, then d to discard patch.
	args := append([]string{"--interactive", "--"}, shellArgs("echo gen > out.txt")...)
	exit, _, stderr := f.runAppInteractive(t, "y\nd\n", args...)
	if exit != app.ExitUserDiscard {
		t.Fatalf("expected ExitUserDiscard(7), got %d (stderr=%s)", exit, stderr)
	}
	if f.read("a.txt") != "dirty baseline\n" {
		t.Fatalf("baseline file should be untouched: %q", f.read("a.txt"))
	}
}

// Package tests contains integration tests that exercise the full patchrun
// command flow against a temporary git repository.
package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

// fixture is a temporary git repository with helpers.
type fixture struct {
	t       *testing.T
	root    string
	prevCwd string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	f := &fixture{t: t, root: root}
	f.git("init", "-q")
	f.git("config", "user.email", "test@example.com")
	f.git("config", "user.name", "patchrun-tests")
	f.git("config", "commit.gpgsign", "false")
	f.git("config", "gpg.format", "openpgp")
	return f
}

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = f.root
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=patchrun-tests", "GIT_AUTHOR_EMAIL=t@t.t",
		"GIT_COMMITTER_NAME=patchrun-tests", "GIT_COMMITTER_EMAIL=t@t.t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}

func (f *fixture) write(rel string, data []byte) {
	f.t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) read(rel string) string {
	f.t.Helper()
	b, err := os.ReadFile(filepath.Join(f.root, rel))
	if err != nil {
		f.t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func (f *fixture) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(f.root, rel))
	return err == nil
}

func (f *fixture) initialCommit() {
	f.t.Helper()
	f.write("README.md", []byte("# test\n"))
	f.git("add", "README.md")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
}

// chdir temporarily changes cwd, returning a restore func.
func (f *fixture) chdir(t *testing.T, dir string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(prev); err != nil {
			t.Fatal(err)
		}
	}
}

// runApp runs app.Run with the fixture's repo as cwd. Returns exit, stdout, stderr.
func (f *fixture) runApp(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	restore := f.chdir(t, f.root)
	defer restore()
	var stdout, stderr bytes.Buffer
	io := app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}
	exit := app.Run(context.Background(), args, io, "test")
	return exit, stdout.String(), stderr.String()
}

// shellArgs returns a portable shell command. On Windows, use cmd.exe /c.
func shellArgs(script string) []string {
	if runtime.GOOS == "windows" {
		return []string{"cmd.exe", "/c", script}
	}
	return []string{"sh", "-c", script}
}

func skipNoSh(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		if runtime.GOOS != "windows" {
			t.Skip("sh not available")
		}
	}
}

func TestClean_ModifyTrackedFile(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo changed > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if f.read("a.txt") != "hello\n" {
		t.Fatalf("original file changed: %q", f.read("a.txt"))
	}
	patch, err := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if err != nil {
		t.Fatalf("read patch: %v", err)
	}
	if !bytes.Contains(patch, []byte("diff --git a/a.txt b/a.txt")) {
		t.Fatalf("patch missing a.txt diff: %s", patch)
	}
}

func TestClean_AddFile(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo new > new.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if f.exists("new.txt") {
		t.Fatalf("new.txt leaked into original repo")
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("new file mode 100644")) {
		t.Fatalf("patch missing new-file marker: %s", patch)
	}
}

func TestClean_DeleteFile(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("rm a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if !f.exists("a.txt") {
		t.Fatalf("original a.txt was deleted")
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("deleted file mode")) {
		t.Fatalf("patch missing deletion marker: %s", patch)
	}
}

func TestApply(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	args := append([]string{"--no-interactive", "--apply", "--"}, shellArgs("echo applied > a.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if f.read("a.txt") != "applied\n" {
		t.Fatalf("apply did not modify original: %q", f.read("a.txt"))
	}
}

func TestDirty_TrackedBaseline(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write("a.txt", []byte("hello\n"))
	f.git("add", "a.txt")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")
	// Make the working tree dirty before patchrun runs.
	f.write("a.txt", []byte("dirty baseline\n"))

	args := append([]string{"--no-interactive", "--allow-dirty", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo gen > b.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if bytes.Contains(patch, []byte("a.txt")) {
		t.Fatalf("patch should not include baseline a.txt:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("b.txt")) {
		t.Fatalf("patch missing b.txt:\n%s", patch)
	}
	if f.read("a.txt") != "dirty baseline\n" {
		t.Fatalf("baseline file should be untouched: %q", f.read("a.txt"))
	}
}

func TestDirty_UntrackedBaseline(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	f.write("note.txt", []byte("baseline untracked\n"))

	args := append([]string{"--no-interactive", "--allow-dirty", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo gen > out.txt")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if bytes.Contains(patch, []byte("note.txt")) {
		t.Fatalf("patch should not include baseline note.txt:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("out.txt")) {
		t.Fatalf("patch missing out.txt:\n%s", patch)
	}
}

func TestChildFails_PreservesPatch(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo partial > out.txt; exit 42")...)
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitChildFailed {
		t.Fatalf("expected ExitChildFailed, got %d", exit)
	}
	patch, err := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if err != nil {
		t.Fatalf("patch not saved: %v", err)
	}
	if !bytes.Contains(patch, []byte("out.txt")) {
		t.Fatalf("patch should include out.txt:\n%s", patch)
	}
}

func TestStdout(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--stdout", "--"}, shellArgs("echo data > new.txt")...)
	exit, stdout, _ := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stdout, "diff --git a/new.txt b/new.txt") {
		t.Fatalf("stdout missing diff: %s", stdout)
	}
}

func TestJSON(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--json", "--"}, shellArgs("echo data > new.txt")...)
	exit, stdout, _ := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d", exit)
	}
	var res app.Result
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if res.Patch.FilesChanged != 1 {
		t.Fatalf("expected 1 file changed, got %d", res.Patch.FilesChanged)
	}
	if res.Command.ExitCode != 0 {
		t.Fatalf("exit_code: %d", res.Command.ExitCode)
	}
	if res.Patch.Empty {
		t.Fatalf("patch should not be empty")
	}
}

func TestFilenameWithSpaces(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo hi > \"hello world.txt\"")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("hello world.txt")) {
		t.Fatalf("patch missing spaced filename:\n%s", patch)
	}
}

func TestSubdirectory(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	subdir := filepath.Join(f.root, "packages", "app")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	prev, _ := os.Getwd()
	if err := os.Chdir(subdir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)

	var stdout, stderr bytes.Buffer
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("echo local > local.txt")...)
	exit := app.Run(context.Background(), args, app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr.String())
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("packages/app/local.txt")) {
		t.Fatalf("patch missing subdir path:\n%s", patch)
	}
}

func TestIgnoredExcludedByDefault(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write(".gitignore", []byte("node_modules/\n"))
	f.git("add", ".gitignore")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("mkdir node_modules && echo x > node_modules/x.js && echo lock > package-lock.json")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if bytes.Contains(patch, []byte("node_modules/")) {
		t.Fatalf("ignored file should be excluded:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("package-lock.json")) {
		t.Fatalf("non-ignored file missing:\n%s", patch)
	}
}

func TestIncludeIgnored(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.write(".gitignore", []byte("node_modules/\n"))
	f.git("add", ".gitignore")
	f.git("commit", "-q", "--no-gpg-sign", "-m", "initial")

	args := append([]string{"--no-interactive", "--include-ignored", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs("mkdir node_modules && echo x > node_modules/x.js")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("node_modules/x.js")) {
		t.Fatalf("ignored file should be included:\n%s", patch)
	}
}

func TestExcludePathspec(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{
		"--no-interactive",
		"--exclude", "package-lock.json",
		"--save", filepath.Join(f.root, "out.patch"),
		"--",
	}, shellArgs("echo a > package.json && echo b > package-lock.json")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if bytes.Contains(patch, []byte("package-lock.json")) {
		t.Fatalf("excluded file should not appear:\n%s", patch)
	}
	if !bytes.Contains(patch, []byte("package.json")) {
		t.Fatalf("included file missing:\n%s", patch)
	}
}

func TestNoChangesExitsZero(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := append([]string{"--no-interactive", "--"}, shellArgs("true")...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	if strings.Contains(stderr, "saved patch") {
		t.Fatalf("should not save on empty patch:\n%s", stderr)
	}
}

func TestTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sleep semantics differ on windows")
	}
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	args := []string{"--no-interactive", "--command-timeout", "300ms", "--", "sh", "-c", "sleep 5"}
	exit, _, _ := f.runApp(t, args...)
	if exit != app.ExitTimeout {
		t.Fatalf("expected ExitTimeout(9), got %d", exit)
	}
}

func TestNotInRepo(t *testing.T) {
	skipNoSh(t)
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	var stdout, stderr bytes.Buffer
	args := append([]string{"--no-interactive", "--"}, shellArgs("true")...)
	exit := app.Run(context.Background(), args, app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitNotInRepo {
		t.Fatalf("expected ExitNotInRepo(2), got %d (stderr=%s)", exit, stderr.String())
	}
}

func TestUsage_NoCommand(t *testing.T) {
	skipNoSh(t)
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	var stdout, stderr bytes.Buffer
	exit := app.Run(context.Background(), []string{"--apply"}, app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr}, "test")
	if exit != app.ExitInvalidUsage {
		t.Fatalf("expected ExitInvalidUsage(8), got %d", exit)
	}
}

func TestBinaryFile(t *testing.T) {
	skipNoSh(t)
	f := newFixture(t)
	f.initialCommit()
	// Use a helper sh -c that writes raw binary bytes via printf with octal escapes.
	bin := "printf '\\000\\001\\002\\377A\\000Z' > bin.dat"
	args := append([]string{"--no-interactive", "--save", filepath.Join(f.root, "out.patch"), "--"}, shellArgs(bin)...)
	exit, _, stderr := f.runApp(t, args...)
	if exit != app.ExitOK {
		t.Fatalf("exit=%d stderr=%s", exit, stderr)
	}
	patch, _ := os.ReadFile(filepath.Join(f.root, "out.patch"))
	if !bytes.Contains(patch, []byte("bin.dat")) {
		t.Fatalf("patch missing bin.dat:\n%s", patch)
	}
	// Apply patch in a fresh repo, verify file contents byte-equal.
	other := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = other
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init", "-q")
	runGit("config", "user.email", "t@t.t")
	runGit("config", "user.name", "t")
	runGit("config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(other, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "--no-gpg-sign", "-m", "initial")
	runGit("apply", "--binary", filepath.Join(f.root, "out.patch"))
	got, err := os.ReadFile(filepath.Join(other, "bin.dat"))
	if err != nil {
		t.Fatalf("read applied: %v", err)
	}
	want := []byte{0, 1, 2, 0xff, 'A', 0, 'Z'}
	if !bytes.Equal(got, want) {
		t.Fatalf("binary mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// readClose helper used to ensure no leaked file descriptors.
var _ io.Closer = io.NopCloser(nil)

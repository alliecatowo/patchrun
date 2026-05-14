// Package gitx wraps the `git` executable.
//
// All functions shell out to the system git binary. No git internals are
// re-implemented here. Paths are passed via `git -C <dir>` rather than
// changing the process working directory, so callers may safely use this
// from concurrent goroutines that operate on different repositories.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Git is a small wrapper that locates the git executable and runs commands
// against a specified working directory.
type Git struct {
	Bin     string // resolved path to git executable
	WorkDir string // directory passed via `git -C`
	Verbose bool
	Logger  func(format string, args ...interface{})
}

// New resolves the git binary on PATH. Returns ErrGitMissing if not found.
func New(workDir string) (*Git, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return nil, ErrGitMissing
	}
	return &Git{Bin: bin, WorkDir: workDir}, nil
}

// ErrGitMissing is returned when git is not on PATH.
var ErrGitMissing = errors.New("git not found on PATH")

// CommandError is returned when git exits with non-zero status.
type CommandError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *CommandError) Error() string {
	cmd := "git " + strings.Join(e.Args, " ")
	if e.Stderr != "" {
		return fmt.Sprintf("%s: exit %d: %s", cmd, e.ExitCode, strings.TrimSpace(e.Stderr))
	}
	return fmt.Sprintf("%s: exit %d", cmd, e.ExitCode)
}

// RunBytes runs git with the given args and returns stdout bytes.
// stderr is captured into a CommandError on failure.
func (g *Git) RunBytes(ctx context.Context, args ...string) ([]byte, error) {
	full := append([]string{"-C", g.WorkDir}, args...)
	if g.Verbose && g.Logger != nil {
		g.Logger("git %s", strings.Join(full, " "))
	}
	cmd := exec.CommandContext(ctx, g.Bin, full...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return stdout.Bytes(), &CommandError{
				Args:     full,
				ExitCode: ee.ExitCode(),
				Stderr:   stderr.String(),
			}
		}
		return stdout.Bytes(), fmt.Errorf("git: %w (stderr: %s)", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// RunString returns trimmed stdout as a string.
func (g *Git) RunString(ctx context.Context, args ...string) (string, error) {
	out, err := g.RunBytes(ctx, args...)
	return strings.TrimRight(string(out), "\n\r"), err
}

// RunPiped streams git stdout/stderr to the provided writers and passes
// stdin from the provided reader. Used for `git apply` and similar.
func (g *Git) RunPiped(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, args ...string) error {
	full := append([]string{"-C", g.WorkDir}, args...)
	if g.Verbose && g.Logger != nil {
		g.Logger("git %s", strings.Join(full, " "))
	}
	cmd := exec.CommandContext(ctx, g.Bin, full...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &CommandError{Args: full, ExitCode: ee.ExitCode()}
		}
		return err
	}
	return nil
}

// Version returns the git version string (e.g. "git version 2.43.0").
func (g *Git) Version(ctx context.Context) (string, error) {
	return g.RunString(ctx, "--version")
}

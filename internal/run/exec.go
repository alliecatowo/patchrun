// Package run executes external commands with optional timeouts.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Result describes the outcome of running an external command.
type Result struct {
	ExitCode int
	Duration time.Duration
	TimedOut bool
	Err      error
}

// Spec describes a command invocation.
type Spec struct {
	Args    []string
	Dir     string
	Env     []string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Timeout time.Duration
}

// Run executes spec and returns its result. ctx cancellation also terminates the command.
func Run(ctx context.Context, spec Spec) Result {
	if len(spec.Args) == 0 {
		return Result{ExitCode: 127, Err: errors.New("no command specified")}
	}
	start := time.Now()

	runCtx, cancel := ctx, func() {}
	if spec.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, spec.Timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, spec.Args[0], spec.Args[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = spec.Env
	cmd.Stdin = spec.Stdin
	cmd.Stdout = spec.Stdout
	cmd.Stderr = spec.Stderr
	configurePlatform(cmd)
	cmd.Cancel = func() error {
		return killProcessGroup(cmd)
	}

	err := cmd.Run()
	dur := time.Since(start)

	res := Result{Duration: dur}
	timedOut := false
	if runCtx.Err() == context.DeadlineExceeded {
		timedOut = true
	}
	res.TimedOut = timedOut

	if err == nil {
		res.ExitCode = 0
		return res
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		res.ExitCode = exitErr.ExitCode()
		if timedOut {
			res.Err = fmt.Errorf("command timed out after %s", spec.Timeout)
		}
		return res
	}
	if errors.Is(err, exec.ErrNotFound) || isNotFoundLike(err) {
		res.ExitCode = 127
		res.Err = fmt.Errorf("command not found: %s", spec.Args[0])
		return res
	}
	res.ExitCode = 1
	res.Err = err
	return res
}

func isNotFoundLike(err error) bool {
	if err == nil {
		return false
	}
	var ee *exec.Error
	if errors.As(err, &ee) {
		return errors.Is(ee.Err, exec.ErrNotFound)
	}
	return false
}

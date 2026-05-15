//go:build windows

package run

import (
	"context"
	"errors"
	"os/exec"
)

func runWithPTY(_ context.Context, _ *exec.Cmd, _ Spec) error {
	return errors.New("pty mode is not supported on windows")
}

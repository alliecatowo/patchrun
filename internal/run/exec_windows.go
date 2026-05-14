//go:build windows

package run

import "os/exec"

func configurePlatform(cmd *exec.Cmd) {
}

func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

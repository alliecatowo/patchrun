//go:build !windows

package run

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func runWithPTY(ctx context.Context, cmd *exec.Cmd, spec Spec) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("start pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	// If our stdin is a real terminal, switch it to raw mode so keys like
	// arrows/tab are delivered to the child app instead of being line-edited.
	var (
		restoreRaw func()
		stopResize func()
	)
	if inFile, ok := spec.Stdin.(*os.File); ok && term.IsTerminal(int(inFile.Fd())) {
		oldState, rawErr := term.MakeRaw(int(inFile.Fd()))
		if rawErr == nil {
			restoreRaw = func() {
				_ = term.Restore(int(inFile.Fd()), oldState)
			}
		}
		_ = pty.InheritSize(inFile, ptmx)
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGWINCH)
		go func() {
			for range ch {
				_ = pty.InheritSize(inFile, ptmx)
			}
		}()
		stopResize = func() {
			signal.Stop(ch)
			close(ch)
		}
	}
	if stopResize != nil {
		defer stopResize()
	}
	if restoreRaw != nil {
		defer restoreRaw()
	}

	// stdin -> PTY can remain blocked waiting for user input even after the
	// child exits. Do not wait on it during shutdown.
	if spec.Stdin != nil {
		go func() {
			_, _ = io.Copy(ptmx, spec.Stdin)
			// Best effort: PTY may already be closed during shutdown.
			_ = ptmx.Close()
		}()
	}

	outDone := make(chan struct{}, 1)
	if spec.Stdout != nil {
		go func() {
			_, _ = io.Copy(spec.Stdout, ptmx)
			outDone <- struct{}{}
		}()
	} else {
		go func() {
			_, _ = io.Copy(io.Discard, ptmx)
			outDone <- struct{}{}
		}()
	}

	waitErr := cmd.Wait()
	// Closing PTY unblocks any remaining copies and allows terminal restore.
	_ = ptmx.Close()
	<-outDone

	select {
	case <-ctx.Done():
		_ = killProcessGroup(cmd)
	default:
	}

	return waitErr
}

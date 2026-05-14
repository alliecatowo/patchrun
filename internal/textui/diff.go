package textui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// ShowPatch displays patchBytes to w, optionally piping through a pager.
//
// useTTYPager: if true and stdout is a TTY, attempt to pipe through $PAGER or less.
// Falls back to plain write.
func ShowPatch(patchBytes []byte, w io.Writer, useTTYPager bool) error {
	if !useTTYPager {
		_, err := w.Write(patchBytes)
		return err
	}
	if !isTerminal(w) {
		_, err := w.Write(patchBytes)
		return err
	}

	pagerCmd := os.Getenv("PAGER")
	if pagerCmd == "" {
		if _, err := exec.LookPath("less"); err == nil {
			pagerCmd = "less -R"
		}
	}
	if pagerCmd == "" {
		_, err := w.Write(patchBytes)
		return err
	}

	parts := strings.Fields(pagerCmd)
	if len(parts) == 0 {
		_, err := w.Write(patchBytes)
		return err
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = bytes.NewReader(patchBytes)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pager: %w", err)
	}
	return nil
}

package run

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func skipIfNoSh(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("sh-dependent test")
	}
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
}

func TestRun_ExitCodeZero(t *testing.T) {
	skipIfNoSh(t)
	var out bytes.Buffer
	res := Run(context.Background(), Spec{
		Args:   []string{"sh", "-c", "echo hello"},
		Stdout: &out,
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit %d", res.ExitCode)
	}
	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestRun_ExitNonZero(t *testing.T) {
	skipIfNoSh(t)
	res := Run(context.Background(), Spec{
		Args: []string{"sh", "-c", "exit 42"},
	})
	if res.ExitCode != 42 {
		t.Fatalf("exit %d, want 42", res.ExitCode)
	}
}

func TestRun_NotFound(t *testing.T) {
	res := Run(context.Background(), Spec{
		Args: []string{"definitely-not-a-real-command-xyz"},
	})
	if res.ExitCode != 127 {
		t.Fatalf("exit %d, want 127", res.ExitCode)
	}
	if res.Err == nil {
		t.Fatalf("expected error")
	}
}

func TestRun_Timeout(t *testing.T) {
	skipIfNoSh(t)
	start := time.Now()
	res := Run(context.Background(), Spec{
		Args:    []string{"sh", "-c", "sleep 5"},
		Timeout: 200 * time.Millisecond,
	})
	if !res.TimedOut {
		t.Fatalf("expected TimedOut")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("took too long: %v", time.Since(start))
	}
}

func TestRun_EmptyArgs(t *testing.T) {
	res := Run(context.Background(), Spec{Args: nil})
	if res.ExitCode != 127 {
		t.Fatalf("exit %d", res.ExitCode)
	}
}

func TestRun_StdinPiped(t *testing.T) {
	skipIfNoSh(t)
	var out bytes.Buffer
	res := Run(context.Background(), Spec{
		Args:   []string{"sh", "-c", "cat"},
		Stdin:  strings.NewReader("piped"),
		Stdout: &out,
	})
	if res.ExitCode != 0 {
		t.Fatalf("exit %d", res.ExitCode)
	}
	if out.String() != "piped" {
		t.Fatalf("stdout: %q", out.String())
	}
}

func TestRun_ContextCancel(t *testing.T) {
	skipIfNoSh(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	res := Run(ctx, Spec{Args: []string{"sh", "-c", "sleep 5"}})
	if res.ExitCode == 0 {
		t.Fatalf("expected non-zero exit")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("cancel took too long: %v", time.Since(start))
	}
}

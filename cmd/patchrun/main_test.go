package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alliecatowo/patchrun/internal/app"
)

func TestRealMain_HelpExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exit := realMain([]string{"--help"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("help missing usage")
	}
}

func TestRealMain_HelpColorAlways(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var stdout, stderr bytes.Buffer
	exit := realMain([]string{"--color", "always", "--help"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("expected ANSI color codes in help output, got:\n%s", stderr.String())
	}
}

func TestRealMain_HelpColorNever(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	var stdout, stderr bytes.Buffer
	exit := realMain([]string{"--color", "never", "--help"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d", exit)
	}
	if strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("did not expect ANSI color codes in --color never output")
	}
}

func TestRealMain_Version(t *testing.T) {
	old := version
	version = "test-version"
	defer func() { version = old }()
	var stdout, stderr bytes.Buffer
	exit := realMain([]string{"--version"},
		app.IO{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
	if exit != 0 {
		t.Fatalf("exit=%d", exit)
	}
	if !strings.Contains(stdout.String(), "patchrun test-version") {
		t.Fatalf("got %q", stdout.String())
	}
}

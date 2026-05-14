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

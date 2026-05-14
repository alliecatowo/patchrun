package app

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func parse(t *testing.T, args ...string) (*Options, error) {
	t.Helper()
	var buf bytes.Buffer
	return ParseOptions(args, &buf, "test")
}

func TestParseOptions_HappyPath(t *testing.T) {
	opts, err := parse(t, "--apply", "--", "echo", "hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !opts.Apply {
		t.Fatalf("apply not set")
	}
	if len(opts.Command) != 2 || opts.Command[0] != "echo" || opts.Command[1] != "hi" {
		t.Fatalf("command: %#v", opts.Command)
	}
	if !opts.Stat {
		t.Fatalf("stat should default to true")
	}
}

func TestParseOptions_MissingSeparator(t *testing.T) {
	_, err := parse(t, "--apply", "echo", "hi")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "before '--'") {
		t.Fatalf("err msg: %v", err)
	}
}

func TestParseOptions_NoCommand(t *testing.T) {
	_, err := parse(t, "--apply", "--")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "missing command") {
		t.Fatalf("err msg: %v", err)
	}
}

func TestParseOptions_NoStat(t *testing.T) {
	opts, err := parse(t, "--no-stat", "--", "echo", "hi")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if opts.Stat {
		t.Fatalf("expected stat=false with --no-stat")
	}
}

func TestParseOptions_Timeout(t *testing.T) {
	opts, err := parse(t, "--command-timeout", "5s", "--", "echo", "hi")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if opts.CommandTimeout != 5*time.Second {
		t.Fatalf("timeout: %v", opts.CommandTimeout)
	}
}

func TestParseOptions_IncludeExclude(t *testing.T) {
	opts, err := parse(t, "--include", "src/", "--exclude", "package-lock.json", "--", "echo")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(opts.Includes) != 1 || opts.Includes[0] != "src/" {
		t.Fatalf("includes: %#v", opts.Includes)
	}
	if len(opts.Excludes) != 1 || opts.Excludes[0] != "package-lock.json" {
		t.Fatalf("excludes: %#v", opts.Excludes)
	}
}

func TestParseOptions_Mutex(t *testing.T) {
	for _, args := range [][]string{
		{"--quiet", "--verbose", "--", "echo"},
		{"--allow-dirty", "--fail-on-dirty", "--", "echo"},
		{"--interactive", "--no-interactive", "--", "echo"},
	} {
		_, err := parse(t, args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
	}
}

func TestParseOptions_Version(t *testing.T) {
	_, err := parse(t, "--version")
	if _, ok := err.(VersionError); !ok {
		t.Fatalf("expected VersionError, got %T %v", err, err)
	}
}

func TestParseOptions_Help(t *testing.T) {
	_, err := parse(t, "--help")
	if _, ok := err.(HelpError); !ok {
		t.Fatalf("expected HelpError, got %T %v", err, err)
	}
}

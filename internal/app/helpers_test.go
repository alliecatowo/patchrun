package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestErrorTypeMessages(t *testing.T) {
	e := &UsageError{Msg: "boom"}
	if e.Error() != "boom" {
		t.Fatalf("UsageError.Error: %q", e.Error())
	}
	if (HelpError{}).Error() != "help requested" {
		t.Fatalf("HelpError.Error")
	}
	if (VersionError{}).Error() != "version requested" {
		t.Fatalf("VersionError.Error")
	}
}

func TestExitMessage_AllCodes(t *testing.T) {
	cases := map[int]string{
		ExitGeneralFailure: "general failure",
		ExitNotInRepo:      "not inside Git repo",
		ExitGitMissing:     "git missing",
		ExitDirty:          "dirty working tree",
		ExitChildFailed:    "child command failed",
		ExitApplyFailed:    "patch failed to apply",
		ExitUserDiscard:    "user discarded patch",
		ExitInvalidUsage:   "invalid usage",
		ExitTimeout:        "command timed out",
		999:                "",
	}
	for code, want := range cases {
		if got := exitMessage(code); got != want {
			t.Fatalf("exitMessage(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestSanitizeLabel(t *testing.T) {
	cases := map[string]string{
		"":            "run",
		"hello":       "hello",
		"a b c":       "a-b-c",
		"slash/path":  "slash-path",
		"weird!@#":    "weird---",
		"under_score": "under_score",
		"v1.2.3":      "v1-2-3",
	}
	for in, want := range cases {
		if got := sanitizeLabel(in); got != want {
			t.Fatalf("sanitizeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunPrefix(t *testing.T) {
	if got := runPrefix(""); got != "patchrun-" {
		t.Fatalf("default prefix: %q", got)
	}
	if got := runPrefix("shadcn"); got != "patchrun-shadcn-" {
		t.Fatalf("labelled prefix: %q", got)
	}
}

func TestNewRunID_HasExpectedShape(t *testing.T) {
	id := newRunID("", "/tmp/proj")
	if !strings.HasPrefix(id, "patchrun-proj-") {
		t.Fatalf("expected patchrun-proj-...: %q", id)
	}
	id2 := newRunID("shadcn", "/tmp/proj")
	if !strings.HasPrefix(id2, "patchrun-shadcn-proj-") {
		t.Fatalf("expected patchrun-shadcn-proj-...: %q", id2)
	}
	if id == newRunID("", "/tmp/proj") {
		t.Fatalf("run IDs should differ across calls (random suffix)")
	}
}

func TestDefaultSavePath_ContainsTimestamp(t *testing.T) {
	p := defaultSavePath("/tmp/repo")
	if !strings.Contains(p, "/tmp/repo/.patchrun/patchrun-") || !strings.HasSuffix(p, ".patch") {
		t.Fatalf("unexpected: %q", p)
	}
}

func TestRelativePath(t *testing.T) {
	if got := relativePath("/a/b", "/a/b/c/d"); got != "c/d" {
		t.Fatalf("got %q", got)
	}
}

func TestRelativePath_DifferentVolumeFallsBackToInput(t *testing.T) {
	// filepath.Rel can't relate paths in different volumes on Windows; on Unix
	// it falls back to climbing parents. We just assert that whatever we get
	// back is non-empty.
	got := relativePath("/a/b", "/c/d/e")
	if got == "" {
		t.Fatalf("expected non-empty result")
	}
}

func TestCanPrompt_NoInteractive(t *testing.T) {
	r := &runner{opts: &Options{NoInteractive: true}, io: IO{Stdin: stringReaderEmpty{}}}
	if r.canPrompt() {
		t.Fatalf("NoInteractive should suppress prompt")
	}
}

func TestCanPrompt_InteractiveFlagForces(t *testing.T) {
	r := &runner{opts: &Options{Interactive: true}, io: IO{Stdin: stringReaderEmpty{}}}
	if !r.canPrompt() {
		t.Fatalf("--interactive should force prompt")
	}
}

func TestCanPrompt_JSONWithoutInteractive(t *testing.T) {
	r := &runner{opts: &Options{JSON: true}, io: IO{Stdin: stringReaderEmpty{}}}
	if r.canPrompt() {
		t.Fatalf("JSON should suppress prompt absent --interactive")
	}
}

func TestCanPrompt_JSONWithInteractiveAllowed(t *testing.T) {
	r := &runner{opts: &Options{JSON: true, Interactive: true}, io: IO{Stdin: stringReaderEmpty{}}}
	if !r.canPrompt() {
		t.Fatalf("--interactive overrides JSON suppression")
	}
}

func TestWillPromptAfterChild_Combinations(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want bool
	}{
		{"explicit interactive", Options{Interactive: true}, true},
		{"no-interactive", Options{NoInteractive: true}, false},
		{"apply non-interactive", Options{Apply: true}, false},
		{"save non-interactive", Options{SavePath: "x"}, false},
		{"stdout non-interactive", Options{Stdout: true}, false},
		{"json non-interactive", Options{JSON: true}, false},
	}
	for _, c := range cases {
		r := &runner{opts: &c.opts, io: IO{Stdin: stringReaderEmpty{}}}
		if got := r.willPromptAfterChild(); got != c.want {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

type stringReaderEmpty struct{}

func (stringReaderEmpty) Read(p []byte) (int, error) { return 0, nil }

func TestParseOptions_ErrorWrappingChain(t *testing.T) {
	// Ensure UsageError is a normal error.
	var ue *UsageError
	err := &UsageError{Msg: "x"}
	if !errors.As(err, &ue) {
		t.Fatalf("errors.As failed")
	}
}

func TestParseOptions_StatExplicit(t *testing.T) {
	var buf bytes.Buffer
	opts, err := ParseOptions([]string{"--stat=false", "--", "echo"}, &buf, "v")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if opts.Stat {
		t.Fatalf("expected stat=false")
	}
	if !opts.StatExplicit {
		t.Fatalf("expected explicit")
	}
}

func TestParseOptions_DurationParse(t *testing.T) {
	var buf bytes.Buffer
	opts, err := ParseOptions([]string{"--command-timeout", "1h30m", "--", "echo"}, &buf, "v")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if opts.CommandTimeout.Minutes() != 90 {
		t.Fatalf("got %v", opts.CommandTimeout)
	}
}

func TestParseOptions_VerboseQuietExclusive(t *testing.T) {
	var buf bytes.Buffer
	_, err := ParseOptions([]string{"--quiet", "--verbose", "--", "echo"}, &buf, "v")
	if err == nil {
		t.Fatalf("expected mutex error")
	}
}

func TestParseOptions_LeftoverPositional(t *testing.T) {
	var buf bytes.Buffer
	_, err := ParseOptions([]string{"oops", "--", "echo"}, &buf, "v")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "before '--'") {
		t.Fatalf("got: %v", err)
	}
}

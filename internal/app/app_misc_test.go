package app

import (
	"bytes"
	"os"
	"testing"
)

func TestDefaultIO(t *testing.T) {
	got := DefaultIO()
	if got.Stdin != os.Stdin {
		t.Fatalf("stdin")
	}
	if got.Stdout != os.Stdout {
		t.Fatalf("stdout")
	}
	if got.Stderr != os.Stderr {
		t.Fatalf("stderr")
	}
}

func TestVerboseLog_QuietWhenNotVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := &runner{opts: &Options{Verbose: false}, io: IO{Stderr: &buf}}
	r.verboseLog("hello %s", "world")
	if buf.Len() != 0 {
		t.Fatalf("non-verbose should suppress: %q", buf.String())
	}
}

func TestVerboseLog_WritesWhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	r := &runner{opts: &Options{Verbose: true}, io: IO{Stderr: &buf}}
	r.verboseLog("hello %s", "world")
	if buf.String() != "[patchrun] hello world\n" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestLogHuman_QuietSuppresses(t *testing.T) {
	var buf bytes.Buffer
	r := &runner{opts: &Options{Quiet: true}, io: IO{Stderr: &buf}}
	r.logHuman("hi")
	if buf.Len() != 0 {
		t.Fatalf("quiet should suppress: %q", buf.String())
	}
}

func TestLogHuman_WritesWhenNoQuiet(t *testing.T) {
	var buf bytes.Buffer
	r := &runner{opts: &Options{}, io: IO{Stderr: &buf}}
	r.logHuman("hello %s", "world")
	if buf.String() != "hello world\n" {
		t.Fatalf("got %q", buf.String())
	}
}

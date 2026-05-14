package textui

import (
	"bytes"
	"os"
	"runtime"
	"testing"
)

func TestShowPatch_PagerPathViaDevNull(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("/dev/null TTY semantics differ on windows")
	}
	// /dev/null is a char device, so fileIsTerminal returns true, which routes
	// us through the pager branch. PAGER=cat is harmless and verifies the
	// fork/exec path runs without error.
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	t.Setenv("PAGER", "cat")
	patch := []byte("diff --git a/x b/x\n@@ -0,0 +1 @@\n+x\n")
	if err := ShowPatch(patch, f, true); err != nil {
		t.Fatalf("show: %v", err)
	}
}

func TestShowPatch_PagerEmptyFallsBack(t *testing.T) {
	// With PAGER="" and `less` likely absent in a stripped test env,
	// ShowPatch falls back to a direct write. Use a buffer to confirm.
	t.Setenv("PAGER", "")
	t.Setenv("PATH", "/definitely-empty")
	var buf bytes.Buffer
	if err := ShowPatch([]byte("hi"), &buf, true); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "hi" {
		t.Fatalf("got %q", buf.String())
	}
}

func TestSplitZ_EmptyAndTrailingNul(t *testing.T) {
	if got := splitZ(nil); got != nil {
		t.Fatalf("nil should yield nil, got %#v", got)
	}
	if got := splitZ([]byte("a\x00b\x00")); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("trailing NUL: %#v", got)
	}
	if got := splitZ([]byte("only\x00")); len(got) != 1 || got[0] != "only" {
		t.Fatalf("single: %#v", got)
	}
}

func TestFileIsTerminal_NilAndClosed(t *testing.T) {
	if fileIsTerminal(nil) {
		t.Fatalf("nil should not be terminal")
	}
	f, err := os.CreateTemp(t.TempDir(), "x")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// Stat on a closed *os.File returns an error → branch covered.
	if fileIsTerminal(f) {
		t.Fatalf("closed file should not be terminal")
	}
}

func TestFileIsTerminal_DevNullIsCharDevice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("device semantics differ on windows")
	}
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	if !fileIsTerminal(f) {
		t.Fatalf("/dev/null should be a char device on linux/darwin")
	}
}

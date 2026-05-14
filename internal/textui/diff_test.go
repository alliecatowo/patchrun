package textui

import (
	"bytes"
	"testing"
)

func TestShowPatch_NoPager(t *testing.T) {
	var buf bytes.Buffer
	patch := []byte("diff --git a/x b/x\n@@ -0,0 +1 @@\n+x\n")
	if err := ShowPatch(patch, &buf, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), patch) {
		t.Fatalf("got %q want %q", buf.Bytes(), patch)
	}
}

func TestShowPatch_NonTTY_PagerSkipped(t *testing.T) {
	// useTTYPager true but writer is a buffer (non-TTY) -> falls back to direct write.
	var buf bytes.Buffer
	patch := []byte("hello")
	if err := ShowPatch(patch, &buf, true); err != nil {
		t.Fatalf("err: %v", err)
	}
	if buf.String() != "hello" {
		t.Fatalf("got %q", buf.String())
	}
}

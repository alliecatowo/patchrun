package copyx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFilePreserve_SourceMissing(t *testing.T) {
	err := CopyFilePreserve(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestCopyFilePreserve_DirectoryAsFile(t *testing.T) {
	srcDir := t.TempDir()
	err := CopyFilePreserve(srcDir, filepath.Join(t.TempDir(), "out"))
	if err == nil {
		t.Fatalf("expected error copying directory as a file")
	}
}

func TestCopyTree_WithSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevation on windows")
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "real.txt"), []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.txt", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(dst, "link"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link not preserved")
	}
}

func TestEnsureWritableDir(t *testing.T) {
	target := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureWritableDir(target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	// Idempotent
	if err := EnsureWritableDir(target); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
}

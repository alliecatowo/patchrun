package copyx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFilePreserve_ParentIsFile(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	// blocker is a file, so blocker/sub/out cannot be a path.
	if err := CopyFilePreserve(src, filepath.Join(blocker, "sub", "out")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestCopyRegular_ReadOnlyDest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permissions differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permission checks")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	roDir := filepath.Join(dir, "ro")
	if err := os.MkdirAll(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(roDir, 0o755)
	// MkdirAll inside a 0o555 dir to make a deeper subdir should fail.
	if err := CopyFilePreserve(src, filepath.Join(roDir, "deep", "x")); err == nil {
		t.Fatalf("expected error writing under read-only dir")
	}
}

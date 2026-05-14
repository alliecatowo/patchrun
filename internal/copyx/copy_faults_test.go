package copyx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestCopyRegular_OpenSrcFails forces the src-open error branch by passing a
// path that doesn't exist (Lstat already passed for the directory but Open
// would fail when called on a missing file).
func TestCopyRegular_OpenSrcFails(t *testing.T) {
	dir := t.TempDir()
	if err := copyRegular(filepath.Join(dir, "missing"), filepath.Join(dir, "dst"), 0o644); err == nil {
		t.Fatalf("expected error opening missing source")
	}
}

// TestCopyRegular_OpenDstFails forces the dst-open branch by pointing dst into
// a path that goes through a regular file (ENOTDIR on Linux/macOS regardless
// of user).
func TestCopyRegular_OpenDstFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(blocker, "sub", "dst")
	err := copyRegular(src, dst, 0o644)
	if err == nil {
		t.Fatalf("expected ENOTDIR when dst path goes through a file")
	}
}

// TestCopyRegular_CopyWriteFails forces io.Copy to fail by pre-creating the
// tmp destination as a symlink to /dev/full. The OpenFile follows the symlink
// and any subsequent write returns ENOSPC.
func TestCopyRegular_CopyWriteFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full is Linux-specific")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full not available")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")
	// The tmp path copyRegular uses is dst + ".patchrun.tmp" — symlink it to
	// /dev/full so the write fails.
	if err := os.Symlink("/dev/full", dst+".patchrun.tmp"); err != nil {
		t.Fatal(err)
	}
	err := copyRegular(src, dst, 0o644)
	if err == nil {
		t.Fatalf("expected ENOSPC writing to /dev/full")
	}
}

// TestCopyRegular_RenameFails forces os.Rename to fail by pre-occupying the
// destination path with a directory. Rename(file, dir) returns EEXIST/EISDIR
// on Linux for everyone, including root.
func TestCopyRegular_RenameFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")
	// Pre-occupy dst as a directory.
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	// Add a sentinel file so it's a non-empty dir.
	if err := os.WriteFile(filepath.Join(dst, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := copyRegular(src, dst, 0o644)
	if err == nil {
		t.Fatalf("expected rename to fail when dst is a directory")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unexpected ErrNotExist (test setup wrong): %v", err)
	}
}

// TestCopyTree_SourceMissing covers the WalkDir error branch.
func TestCopyTree_SourceMissing(t *testing.T) {
	if err := CopyTree("/definitely-not-a-real-path-xyz", t.TempDir()); err == nil {
		t.Fatalf("expected error")
	}
}

// TestCopyFilePreserve_UnsupportedFileMode forces the "unsupported file mode"
// branch by passing a named pipe (fifo).
func TestCopyFilePreserve_UnsupportedFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fifos are POSIX")
	}
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo")
	// syscall.Mkfifo is in syscall package. We'd need to import it; do it via
	// a small helper.
	if err := makeFifo(fifo); err != nil {
		t.Skip(err)
	}
	defer os.Remove(fifo)
	err := CopyFilePreserve(fifo, filepath.Join(dir, "dst"))
	if err == nil {
		t.Fatalf("expected error on fifo")
	}
}

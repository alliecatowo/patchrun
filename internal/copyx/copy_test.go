package copyx

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCopyFilePreserve_Regular(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src")
	dst := filepath.Join(t.TempDir(), "nested", "dst")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CopyFilePreserve(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("contents: %q", got)
	}
}

func TestCopyFilePreserve_ExecBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable bit not meaningful on windows")
	}
	src := filepath.Join(t.TempDir(), "src.sh")
	dst := filepath.Join(t.TempDir(), "dst.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CopyFilePreserve(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected exec bit, got mode %v", info.Mode())
	}
}

func TestCopyFilePreserve_Symlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "link")
	if err := os.Symlink("target", src); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out", "link")
	if err := CopyFilePreserve(src, dst); err != nil {
		t.Fatalf("copy: %v", err)
	}
	info, err := os.Lstat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dst is not a symlink: %v", info.Mode())
	}
	got, err := os.Readlink(dst)
	if err != nil {
		t.Fatal(err)
	}
	if got != "target" {
		t.Fatalf("link target: %q", got)
	}
}

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a", "b", "c.txt"), []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyTree(src, dst); err != nil {
		t.Fatalf("copy tree: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "a", "b", "c.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "c" {
		t.Fatalf("contents: %q", got)
	}
}

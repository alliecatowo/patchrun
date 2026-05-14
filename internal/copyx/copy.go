// Package copyx provides filesystem copy helpers that preserve permissions,
// executable bits, and symlinks where supported by the host OS.
package copyx

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// CopyFilePreserve copies srcPath to dstPath, preserving mode bits.
// If src is a symlink, the link itself is replicated (not followed).
// Parent directories of dstPath are created with mode 0o755 if missing.
func CopyFilePreserve(srcPath, dstPath string) error {
	info, err := os.Lstat(srcPath)
	if err != nil {
		return fmt.Errorf("lstat %s: %w", srcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(srcPath)
		if err != nil {
			return fmt.Errorf("readlink %s: %w", srcPath, err)
		}
		_ = os.Remove(dstPath)
		if err := os.Symlink(target, dstPath); err != nil {
			return fmt.Errorf("symlink %s: %w", dstPath, err)
		}
		return nil
	case info.Mode()&os.ModeDir != 0:
		return fmt.Errorf("expected file, got directory: %s", srcPath)
	case info.Mode().IsRegular():
		return copyRegular(srcPath, dstPath, info.Mode().Perm())
	default:
		return fmt.Errorf("unsupported file mode %v at %s", info.Mode(), srcPath)
	}
}

func copyRegular(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer in.Close()

	tmp := dst + ".patchrun.tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("open dst: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close dst: %w", err)
	}
	if err := os.Chmod(tmp, perm); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// CopyTree recursively copies a directory tree.
func CopyTree(srcRoot, dstRoot string) error {
	return filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(srcRoot, path)
		if relErr != nil {
			return relErr
		}
		dst := filepath.Join(dstRoot, rel)
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			target, lerr := os.Readlink(path)
			if lerr != nil {
				return lerr
			}
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			_ = os.Remove(dst)
			return os.Symlink(target, dst)
		case d.IsDir():
			return os.MkdirAll(dst, info.Mode().Perm()|0o700)
		default:
			return CopyFilePreserve(path, dst)
		}
	})
}

// EnsureWritableDir creates the directory (and parents) if missing.
func EnsureWritableDir(path string) error {
	err := os.MkdirAll(path, 0o755)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return nil
}

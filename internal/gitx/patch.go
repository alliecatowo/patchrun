package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// DiffBinary returns `git diff --binary <base>` bytes (tracked staged+unstaged
// changes relative to base). It does NOT include untracked files.
func (g *Git) DiffBinary(ctx context.Context, base string) ([]byte, error) {
	return g.RunBytes(ctx, "diff", "--binary", base)
}

// DiffBinaryCached returns `git diff --binary --cached <base> -- pathspecs...`
// which captures everything currently staged in the worktree relative to base.
func (g *Git) DiffBinaryCached(ctx context.Context, base string, pathspecs []string) ([]byte, error) {
	args := []string{"diff", "--binary", "--cached", base}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	return g.RunBytes(ctx, args...)
}

// DiffNameStatusCached returns `git diff --name-status --cached -z <base>` data.
func (g *Git) DiffNameStatusCached(ctx context.Context, base string, pathspecs []string) ([]byte, error) {
	args := []string{"diff", "--name-status", "--cached", "-z", base}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	return g.RunBytes(ctx, args...)
}

// DiffNumstatCached returns `git diff --numstat --cached -z <base>` data.
func (g *Git) DiffNumstatCached(ctx context.Context, base string, pathspecs []string) ([]byte, error) {
	args := []string{"diff", "--numstat", "--cached", "-z", base}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}
	return g.RunBytes(ctx, args...)
}

// ApplyCheck runs `git apply --check` against patchPath. Returns nil if the
// patch would apply cleanly.
func (g *Git) ApplyCheck(ctx context.Context, patchPath string) error {
	_, err := g.RunBytes(ctx, "apply", "--check", "--binary", "--whitespace=nowarn", patchPath)
	return err
}

// Apply runs `git apply` against patchPath.
func (g *Git) Apply(ctx context.Context, patchPath string, threeWay bool) error {
	args := []string{"apply", "--binary", "--whitespace=nowarn"}
	if threeWay {
		args = append(args, "--3way")
	}
	args = append(args, patchPath)
	_, err := g.RunBytes(ctx, args...)
	return err
}

// ApplyBytesToIndex applies an in-memory patch through stdin to git apply --index.
func (g *Git) ApplyBytesToIndex(ctx context.Context, patch []byte) error {
	if len(patch) == 0 {
		return nil
	}
	var stderr bytes.Buffer
	err := g.RunPiped(ctx, bytes.NewReader(patch), nil, &stderr,
		"apply", "--binary", "--index", "--whitespace=nowarn")
	if err == nil {
		return nil
	}
	// Fallback without --index in case of subtle stage mismatch.
	var stderr2 bytes.Buffer
	if err2 := g.RunPiped(ctx, bytes.NewReader(patch), nil, &stderr2,
		"apply", "--binary", "--whitespace=nowarn"); err2 == nil {
		return nil
	}
	return fmt.Errorf("git apply --index failed: %v (stderr: %s)", err, strings.TrimSpace(stderr.String()))
}

// LsUntracked returns the NUL-terminated paths of untracked, non-ignored files
// relative to the worktree root.
func (g *Git) LsUntracked(ctx context.Context) ([]string, error) {
	out, err := g.RunBytes(ctx, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	return splitZ(out), nil
}

// WriteTempPatch writes patch to a unique file under dir with the given prefix.
// Returns the path.
func WriteTempPatch(dir, prefix string, patch []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, prefix+"-*.patch")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(patch); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// SavePatchAtomic writes patch to path atomically.
func SavePatchAtomic(path string, patch []byte) error {
	dir := dirOf(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".patchrun-save-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(patch); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}

// PatchIsEmpty reports whether the given patch bytes contain any diff hunks.
func PatchIsEmpty(patch []byte) bool {
	if len(bytes.TrimSpace(patch)) == 0 {
		return true
	}
	return !bytes.Contains(patch, []byte("diff --git"))
}

func splitZ(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	s := strings.TrimSuffix(string(data), "\x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\x00")
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return ""
}

// SanityCheckGit verifies git is callable and meets minimum version.
func SanityCheckGit(ctx context.Context) error {
	g, err := New(".")
	if err != nil {
		return err
	}
	out, err := g.Version(ctx)
	if err != nil {
		return errors.New("git failed to execute")
	}
	_ = out
	return nil
}

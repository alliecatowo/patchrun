package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AddDetachedWorktree creates a new detached worktree at path checking out ref.
// Parent directories of path will be created.
func (g *Git) AddDetachedWorktree(ctx context.Context, path, ref string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}
	_, err := g.RunBytes(ctx, "worktree", "add", "--detach", path, ref)
	return err
}

// RemoveWorktree removes a worktree (best-effort fallback to filesystem rm if
// the path lives under tempParent and starts with the patchrun prefix).
func (g *Git) RemoveWorktree(ctx context.Context, path, tempParent, prefix string) error {
	_, err := g.RunBytes(ctx, "worktree", "remove", "--force", path)
	if err == nil {
		return nil
	}
	if !safeToRemove(path, tempParent, prefix) {
		return fmt.Errorf("git worktree remove failed: %w (path %q is outside expected temp prefix; not falling back to rm -rf)", err, path)
	}
	if rmErr := os.RemoveAll(path); rmErr != nil {
		return fmt.Errorf("git worktree remove failed: %v; rm fallback: %w", err, rmErr)
	}
	// Try a prune so git forgets the worktree handle.
	_, _ = g.RunBytes(ctx, "worktree", "prune")
	return nil
}

// safeToRemove returns true if path is under tempParent and uses the given prefix.
func safeToRemove(path, tempParent, prefix string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	parentAbs, err := filepath.Abs(tempParent)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(parentAbs, abs)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return false
	}
	return strings.HasPrefix(filepath.Base(abs), prefix)
}

// CommitBaseline creates a non-signed commit with the given message using a
// fixed local identity. Use only inside disposable worktrees.
func (g *Git) CommitBaseline(ctx context.Context, message string) (string, error) {
	args := []string{
		"-c", "user.name=patchrun",
		"-c", "user.email=patchrun@localhost",
		"-c", "commit.gpgsign=false",
		"-c", "core.hooksPath=/dev/null",
		"commit",
		"--no-gpg-sign",
		"--allow-empty",
		"-m", message,
	}
	_, err := g.RunBytes(ctx, args...)
	if err != nil {
		return "", err
	}
	sha, err := g.RunString(ctx, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return sha, nil
}

// StageAll runs `git add -A` (optionally with `-f` for ignored inclusion).
func (g *Git) StageAll(ctx context.Context, force bool) error {
	args := []string{"add", "-A"}
	if force {
		args = []string{"add", "-A", "-f"}
	}
	_, err := g.RunBytes(ctx, args...)
	return err
}

// IsInsideWorkTree reports whether g.WorkDir is inside a git working tree.
func (g *Git) IsInsideWorkTree(ctx context.Context) (bool, error) {
	out, err := g.RunString(ctx, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return false, nil
		}
		return false, err
	}
	return out == "true", nil
}

// HasSubmodulesFile reports whether the repository has a `.gitmodules` file.
func HasSubmodulesFile(repoRoot string) bool {
	_, err := os.Stat(filepath.Join(repoRoot, ".gitmodules"))
	return err == nil
}

package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// RepoRoot returns the absolute path to the working tree root for workDir.
func RepoRoot(ctx context.Context, workDir string) (string, error) {
	g, err := New(workDir)
	if err != nil {
		return "", err
	}
	out, err := g.RunString(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return "", fmt.Errorf("not inside a git working tree: %w", err)
		}
		return "", err
	}
	return out, nil
}

// HeadSHA returns the full SHA at HEAD. Returns ErrUnbornHead if no commits exist.
func (g *Git) HeadSHA(ctx context.Context) (string, error) {
	out, err := g.RunString(ctx, "rev-parse", "--verify", "HEAD")
	if err != nil {
		var ce *CommandError
		if errors.As(err, &ce) {
			return "", ErrUnbornHead
		}
		return "", err
	}
	return out, nil
}

// HeadSHAShort returns the abbreviated SHA at HEAD.
func (g *Git) HeadSHAShort(ctx context.Context) (string, error) {
	return g.RunString(ctx, "rev-parse", "--short", "HEAD")
}

// ErrUnbornHead is returned when HEAD points to an unborn branch.
var ErrUnbornHead = errors.New("repository has no commits yet")

// BranchOrDetached returns either the current branch name or "(detached <sha>)".
func (g *Git) BranchOrDetached(ctx context.Context) (string, error) {
	br, err := g.RunString(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil && br != "" {
		return br, nil
	}
	short, err := g.HeadSHAShort(ctx)
	if err != nil {
		return "", err
	}
	return "(detached " + short + ")", nil
}

// StatusEntry is a single porcelain v1 record.
type StatusEntry struct {
	IndexStatus    byte // staged change marker
	WorkTreeStatus byte // working tree change marker
	Path           string
	OldPath        string // populated for R/C entries
}

// IsDirty reports whether the entry represents any change.
func (e StatusEntry) IsDirty() bool {
	if e.IndexStatus == ' ' && e.WorkTreeStatus == ' ' {
		return false
	}
	if e.IndexStatus == '!' && e.WorkTreeStatus == '!' {
		return false // ignored, only seen with --ignored
	}
	return true
}

// IsUntracked reports whether the entry is an untracked file (??).
func (e StatusEntry) IsUntracked() bool {
	return e.IndexStatus == '?' && e.WorkTreeStatus == '?'
}

// IsIgnored reports whether the entry is an ignored file (!!).
func (e StatusEntry) IsIgnored() bool {
	return e.IndexStatus == '!' && e.WorkTreeStatus == '!'
}

// Status returns the parsed porcelain v1 -z status of the working tree.
// Untracked files are listed, ignored files are not (use StatusIgnored).
func (g *Git) Status(ctx context.Context) ([]StatusEntry, error) {
	out, err := g.RunBytes(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parsePorcelainZ(out)
}

// StatusIgnored returns ignored entries (status flag `!!`).
func (g *Git) StatusIgnored(ctx context.Context) ([]StatusEntry, error) {
	out, err := g.RunBytes(ctx, "status", "--porcelain=v1", "-z", "--ignored=traditional", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	entries, err := parsePorcelainZ(out)
	if err != nil {
		return nil, err
	}
	var ig []StatusEntry
	for _, e := range entries {
		if e.IsIgnored() {
			ig = append(ig, e)
		}
	}
	return ig, nil
}

// IsDirty reports whether the worktree has any tracked/untracked changes.
func (g *Git) IsDirty(ctx context.Context) (bool, []StatusEntry, error) {
	entries, err := g.Status(ctx)
	if err != nil {
		return false, nil, err
	}
	for _, e := range entries {
		if e.IsDirty() {
			return true, entries, nil
		}
	}
	return false, entries, nil
}

// StatusFingerprint produces a stable byte signature for a status set.
// Used to detect whether the original working tree changed mid-run.
func StatusFingerprint(entries []StatusEntry) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteByte(e.IndexStatus)
		b.WriteByte(e.WorkTreeStatus)
		b.WriteByte(' ')
		b.WriteString(e.Path)
		if e.OldPath != "" {
			b.WriteByte(0)
			b.WriteString(e.OldPath)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func parsePorcelainZ(data []byte) ([]StatusEntry, error) {
	if len(data) == 0 {
		return nil, nil
	}
	s := string(data)
	if strings.HasSuffix(s, "\x00") {
		s = s[:len(s)-1]
	}
	fields := strings.Split(s, "\x00")
	var out []StatusEntry
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		if f == "" {
			continue
		}
		if len(f) < 3 {
			return nil, fmt.Errorf("malformed porcelain record %q", f)
		}
		entry := StatusEntry{
			IndexStatus:    f[0],
			WorkTreeStatus: f[1],
			Path:           f[3:],
		}
		if entry.IndexStatus == 'R' || entry.IndexStatus == 'C' ||
			entry.WorkTreeStatus == 'R' || entry.WorkTreeStatus == 'C' {
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("missing rename old path for %q", f)
			}
			entry.OldPath = fields[i+1]
			i++
		}
		out = append(out, entry)
	}
	return out, nil
}

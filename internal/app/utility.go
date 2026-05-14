package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alliecatowo/patchrun/internal/gitx"
)

// runUtilitySubcommand handles --completion, --list-runs, --prune.
// Returns (exitCode, true) if a utility subcommand was handled, otherwise
// (0, false).
func runUtilitySubcommand(opts *Options, io IO) (int, bool) {
	switch {
	case opts.CompletionShell != "":
		if err := WriteCompletion(opts.CompletionShell, io.Stdout); err != nil {
			fmt.Fprintf(io.Stderr, "error: %v\n", err)
			return ExitGeneralFailure, true
		}
		return ExitOK, true
	case opts.ListRuns:
		return listRuns(opts, io), true
	case opts.Prune:
		return pruneRuns(opts, io), true
	}
	return 0, false
}

func worktreeParent(opts *Options) string {
	if opts.WorktreeDir != "" {
		return opts.WorktreeDir
	}
	return filepath.Join(os.TempDir(), "patchrun")
}

// listRuns enumerates kept patchrun worktrees under the temp parent.
func listRuns(opts *Options, io IO) int {
	parent := worktreeParent(opts)
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return ExitOK
		}
		fmt.Fprintf(io.Stderr, "error: %v\n", err)
		return ExitGeneralFailure
	}
	type row struct {
		name    string
		modTime time.Time
	}
	var rows []row
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "patchrun-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rows = append(rows, row{name: e.Name(), modTime: info.ModTime()})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].modTime.Before(rows[j].modTime) })
	for _, r := range rows {
		fmt.Fprintf(io.Stdout, "%s\t%s\n", r.modTime.Format(time.RFC3339), filepath.Join(parent, r.name))
	}
	return ExitOK
}

// pruneRuns removes every patchrun-* directory under the temp parent.
// Uses `git worktree remove` first, falling back to rm under the same safety
// constraints as cleanup (must be inside parent and have the patchrun prefix).
func pruneRuns(opts *Options, io IO) int {
	parent := worktreeParent(opts)
	entries, err := os.ReadDir(parent)
	if err != nil {
		if os.IsNotExist(err) {
			return ExitOK
		}
		fmt.Fprintf(io.Stderr, "error: %v\n", err)
		return ExitGeneralFailure
	}
	removed, failed := 0, 0
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "patchrun-") {
			continue
		}
		path := filepath.Join(parent, e.Name())
		// Try `git -C <containing-repo> worktree remove --force` first;
		// if the repo this worktree belonged to is unknown, just rm.
		// Read the .git pointer file to find the original repo.
		repoRoot := repoOfWorktree(path)
		if repoRoot != "" {
			if g, err := gitx.New(repoRoot); err == nil {
				if err := g.RemoveWorktree(context.Background(), path, parent, "patchrun-"); err == nil {
					removed++
					continue
				}
			}
		}
		if err := os.RemoveAll(path); err != nil {
			fmt.Fprintf(io.Stderr, "warning: failed to remove %s: %v\n", path, err)
			failed++
			continue
		}
		removed++
	}
	fmt.Fprintf(io.Stdout, "pruned %d worktree(s)", removed)
	if failed > 0 {
		fmt.Fprintf(io.Stdout, ", %d failed", failed)
	}
	fmt.Fprintln(io.Stdout)
	if failed > 0 {
		return ExitGeneralFailure
	}
	return ExitOK
}

// repoOfWorktree reads the .git pointer file inside a worktree to find the
// original repository's git dir, then derives the repo root.
func repoOfWorktree(worktreePath string) string {
	pointer, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(pointer))
	// Expect "gitdir: /abs/path/to/original/.git/worktrees/<id>"
	const prefix = "gitdir:"
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	// .git/worktrees/<id> -> go up three levels to the original repo root.
	for i := 0; i < 3; i++ {
		gitdir = filepath.Dir(gitdir)
	}
	return gitdir
}

// SidecarMetadata is the JSON blob written next to a saved patch.
type SidecarMetadata struct {
	Version      string   `json:"patchrun_version"`
	Repo         string   `json:"repo"`
	HeadSHA      string   `json:"head_sha"`
	Branch       string   `json:"branch"`
	Dirty        bool     `json:"dirty"`
	BaselineRef  string   `json:"baseline_ref"`
	Command      []string `json:"command"`
	ExitCode     int      `json:"exit_code"`
	DurationMs   int64    `json:"duration_ms"`
	TimedOut     bool     `json:"timed_out"`
	GeneratedAt  string   `json:"generated_at"`
	FilesChanged int      `json:"files_changed"`
	Insertions   int      `json:"insertions"`
	Deletions    int      `json:"deletions"`
}

// WriteSidecar writes a JSON sidecar next to patchPath at patchPath + ".meta.json".
func WriteSidecar(patchPath string, meta SidecarMetadata, w io.Writer) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	side := patchPath + ".meta.json"
	if err := os.WriteFile(side, data, 0o644); err != nil {
		return err
	}
	return nil
}

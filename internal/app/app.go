package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alliecatowo/patchrun/internal/copyx"
	"github.com/alliecatowo/patchrun/internal/gitx"
	"github.com/alliecatowo/patchrun/internal/run"
	"github.com/alliecatowo/patchrun/internal/textui"
)

// IO collects the streams the application uses. Tests inject this.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// DefaultIO returns process stdio.
func DefaultIO() IO {
	return IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
}

// Result is the structured outcome reported in JSON mode.
type Result struct {
	Repo         string        `json:"repo"`
	Cwd          string        `json:"cwd"`
	TempWorktree string        `json:"temp_worktree"`
	Kept         bool          `json:"kept"`
	Base         baseResult    `json:"base"`
	Command      commandResult `json:"command"`
	Patch        patchResult   `json:"patch"`
	Error        *errorResult  `json:"error"`
}

type baseResult struct {
	Head           string `json:"head"`
	Branch         string `json:"branch"`
	Dirty          bool   `json:"dirty"`
	BaselineCommit string `json:"baseline_commit"`
}

type commandResult struct {
	Args       []string `json:"args"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
	TimedOut   bool     `json:"timed_out"`
}

type patchResult struct {
	Empty        bool         `json:"empty"`
	SavedPath    string       `json:"saved_path,omitempty"`
	Applied      bool         `json:"applied"`
	FilesChanged int          `json:"files_changed"`
	Insertions   int          `json:"insertions"`
	Deletions    int          `json:"deletions"`
	NameStatus   []nameStatus `json:"name_status"`
}

type nameStatus struct {
	Status string `json:"status"`
	Path   string `json:"path"`
}

type errorResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Run is the entry point. It returns an exit code suitable for os.Exit.
func Run(parentCtx context.Context, argv []string, io IO, version string) int {
	opts, err := ParseOptions(argv, io.Stderr, version)
	if err != nil {
		switch e := err.(type) {
		case HelpError:
			fmt.Fprint(io.Stderr, helpText(version))
			_ = e
			return ExitOK
		case VersionError:
			fmt.Fprintf(io.Stdout, "patchrun %s\n", version)
			_ = e
			return ExitOK
		case *UsageError:
			fmt.Fprintf(io.Stderr, "error: %s\n\n", e.Msg)
			fmt.Fprint(io.Stderr, helpText(version))
			return ExitInvalidUsage
		default:
			fmt.Fprintf(io.Stderr, "error: %s\n", err.Error())
			return ExitInvalidUsage
		}
	}

	if exit, handled := runUtilitySubcommand(opts, io); handled {
		return exit
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	colorMode := textui.ColorAuto
	switch opts.Color {
	case "always":
		colorMode = textui.ColorAlways
	case "never":
		colorMode = textui.ColorNever
	}

	rr := &runner{opts: opts, io: io, version: version}
	rr.colorOut = textui.NewColorizer(colorMode, io.Stdout)
	rr.colorErr = textui.NewColorizer(colorMode, io.Stderr)
	if opts.JSON {
		// Always strip color on stdout in JSON mode regardless of --color.
		rr.colorOut = textui.NewColorizer(textui.ColorNever, io.Stdout)
	}
	return rr.execute(ctx)
}

type runner struct {
	opts     *Options
	io       IO
	version  string
	colorOut *textui.Colorizer
	colorErr *textui.Colorizer

	originalRoot string
	originalCwd  string
	relCwd       string
	tempWorktree string
	tempParent   string
	keep         bool
	runID        string

	repoGit *gitx.Git
	tempGit *gitx.Git

	originalHead      string
	branch            string
	originalDirty     bool
	originalStatusSig string
	baselineCommitSHA string
	baselineRef       string
	cmdResult         run.Result
	patchBytes        []byte
	nameStatusEntries []textui.NameStatusEntry
	numstatEntries    []textui.NumstatEntry
	totals            textui.Totals
	savedPath         string
	applied           bool
}

// writeSidecar writes a .meta.json next to the saved patch unless --no-sidecar.
func (r *runner) writeSidecar(patchPath string) {
	if r.opts.NoSidecar {
		return
	}
	meta := SidecarMetadata{
		Version:      r.version,
		Repo:         r.originalRoot,
		HeadSHA:      r.originalHead,
		Branch:       r.branch,
		Dirty:        r.originalDirty,
		BaselineRef:  r.baselineRef,
		Command:      r.opts.Command,
		ExitCode:     r.cmdResult.ExitCode,
		DurationMs:   r.cmdResult.Duration.Milliseconds(),
		TimedOut:     r.cmdResult.TimedOut,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		FilesChanged: r.totals.Files,
		Insertions:   r.totals.Insertions,
		Deletions:    r.totals.Deletions,
	}
	if err := WriteSidecar(patchPath, meta, r.io.Stderr); err != nil {
		r.verboseLog("sidecar write failed: %v", err)
	}
}

func (r *runner) logHuman(format string, args ...interface{}) {
	if r.opts.Quiet {
		return
	}
	if r.opts.JSON {
		// keep human logs on stderr in JSON mode
	}
	fmt.Fprintf(r.io.Stderr, format+"\n", args...)
}

func (r *runner) verboseLog(format string, args ...interface{}) {
	if !r.opts.Verbose {
		return
	}
	fmt.Fprintf(r.io.Stderr, "[patchrun] "+format+"\n", args...)
}

func (r *runner) execute(ctx context.Context) int {
	exit := r.run(ctx)
	if r.opts.JSON {
		r.emitJSON(exit)
	}
	r.cleanup(ctx)
	return exit
}

func (r *runner) run(ctx context.Context) int {
	// 1. Resolve cwd and repo root.
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: getwd: %v\n", err)
		return ExitGeneralFailure
	}
	if r.opts.Cwd != "" {
		abs, err := filepath.Abs(r.opts.Cwd)
		if err != nil {
			fmt.Fprintf(r.io.Stderr, "error: --cwd: %v\n", err)
			return ExitGeneralFailure
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			fmt.Fprintf(r.io.Stderr, "error: --cwd %q is not a directory\n", abs)
			return ExitGeneralFailure
		}
		cwd = abs
	}
	r.originalCwd = cwd

	// 2. Sanity-check git.
	if err := gitx.SanityCheckGit(ctx); err != nil {
		fmt.Fprintf(r.io.Stderr, "error: %v\n", err)
		return ExitGitMissing
	}

	// 3. Find repo root.
	root, err := gitx.RepoRoot(ctx, cwd)
	if err != nil {
		fmt.Fprintln(r.io.Stderr, "error: patchrun must be run inside a Git working tree.")
		return ExitNotInRepo
	}
	r.originalRoot = root
	rel, err := filepath.Rel(root, cwd)
	if err != nil {
		rel = "."
	}
	r.relCwd = rel

	r.repoGit, err = gitx.NewWithBin(root, r.opts.GitBin)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: %v\n", err)
		return ExitGitMissing
	}
	r.repoGit.Verbose = r.opts.Verbose
	r.repoGit.Logger = func(format string, args ...interface{}) { r.verboseLog(format, args...) }

	// 4. HEAD.
	head, err := r.repoGit.HeadSHA(ctx)
	if err != nil {
		if errors.Is(err, gitx.ErrUnbornHead) {
			fmt.Fprintln(r.io.Stderr, "error: patchrun requires at least one commit. Try: git commit --allow-empty -m 'init'")
			return ExitGeneralFailure
		}
		fmt.Fprintf(r.io.Stderr, "error: resolve HEAD: %v\n", err)
		return ExitGeneralFailure
	}
	r.originalHead = head
	br, err := r.repoGit.BranchOrDetached(ctx)
	if err != nil {
		br = "(unknown)"
	}
	r.branch = br

	// 5. Dirty check.
	dirty, entries, err := r.repoGit.IsDirty(ctx)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: status: %v\n", err)
		return ExitGeneralFailure
	}
	r.originalDirty = dirty
	r.originalStatusSig = gitx.StatusFingerprint(entries)

	if dirty {
		allowDirty := r.opts.AllowDirty
		if !allowDirty && !r.opts.FailOnDirty {
			// Default: ask if interactive TTY, else fail.
			if r.canPrompt() {
				fmt.Fprintln(r.io.Stderr, r.colorErr.Yellow("Your working tree has existing changes."))
				fmt.Fprintln(r.io.Stderr, "patchrun can include them as a baseline so only new command changes appear in the final patch.")
				prompter := NewPrompter(r.io.Stdin, r.io.Stderr)
				ok, perr := prompter.Confirm("Continue with dirty baseline?", false)
				if perr != nil {
					fmt.Fprintf(r.io.Stderr, "error: %v\n", perr)
					return ExitDirty
				}
				if !ok {
					fmt.Fprintln(r.io.Stderr, "aborted.")
					return ExitDirty
				}
				allowDirty = true
			} else {
				fmt.Fprintln(r.io.Stderr, "error: working tree has unstaged or untracked changes (pass --allow-dirty to include them as baseline).")
				return ExitDirty
			}
		}
		if r.opts.FailOnDirty {
			fmt.Fprintln(r.io.Stderr, "error: working tree is dirty (--fail-on-dirty).")
			return ExitDirty
		}
		if !allowDirty {
			fmt.Fprintln(r.io.Stderr, "error: working tree is dirty.")
			return ExitDirty
		}
	}

	// 6. Submodule warning.
	if gitx.HasSubmodulesFile(root) {
		r.logHuman("%s submodules detected. patchrun captures superproject changes; run patchrun inside a submodule to capture submodule file changes.",
			r.colorErr.Yellow("warning:"))
	}

	// 7. Compute temp paths and create disposable worktree.
	if err := r.setupTempWorktree(ctx); err != nil {
		fmt.Fprintf(r.io.Stderr, "error: %v\n", err)
		return ExitGeneralFailure
	}

	// 8. Replay baseline (dirty state) into temp worktree.
	if dirty {
		baseSHA, err := r.replayBaseline(ctx)
		if err != nil {
			fmt.Fprintf(r.io.Stderr, "error: replay baseline: %v\n", err)
			return ExitGeneralFailure
		}
		r.baselineCommitSHA = baseSHA
		r.baselineRef = baseSHA
	} else {
		r.baselineRef = r.originalHead
	}

	// 9. Banner.
	r.printBanner()

	// 10. Run the command in temp worktree.
	r.cmdResult = r.runChildCommand(ctx)

	exitFromChild := r.cmdResult.ExitCode
	if r.cmdResult.TimedOut {
		exitFromChild = ExitTimeout
	} else if r.cmdResult.Err != nil && r.cmdResult.ExitCode == 0 {
		fmt.Fprintf(r.io.Stderr, "error: %v\n", r.cmdResult.Err)
		exitFromChild = ExitGeneralFailure
	}

	r.logHuman("\nCommand exited: %d (%s)", r.cmdResult.ExitCode, r.cmdResult.Duration.Round(time.Millisecond))
	if r.cmdResult.TimedOut {
		r.logHuman("%s command timed out", r.colorErr.Red("warning:"))
	}

	// 11. Capture patch.
	patch, err := r.capturePatch(ctx)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: capture patch: %v\n", err)
		return ExitGeneralFailure
	}
	r.patchBytes = patch

	// 12. Empty patch?
	if gitx.PatchIsEmpty(patch) {
		r.logHuman(r.colorErr.Dim("No repo changes."))
		if r.cmdResult.TimedOut {
			return ExitTimeout
		}
		if r.cmdResult.ExitCode != 0 {
			return ExitChildFailed
		}
		return ExitOK
	}

	// 13. Parse name-status/numstat for summary.
	if err := r.parseSummary(ctx); err != nil {
		r.verboseLog("summary parse: %v", err)
	}

	// 14. Show summary in human/JSON-with-logs mode.
	if !r.opts.Quiet {
		r.printSummary()
	}

	// 15. Dispatch action.
	exit := r.dispatchActions(ctx)
	if exit == ExitOK {
		if r.cmdResult.TimedOut {
			return ExitTimeout
		}
		if r.cmdResult.ExitCode != 0 {
			// Even if patch was saved/applied successfully, surface the child failure.
			return ExitChildFailed
		}
	}
	_ = exitFromChild
	return exit
}

// willPromptAfterChild reports whether the post-run flow will read from
// r.io.Stdin. If so, the child must not be given that reader, otherwise the
// child's pipe drain goroutine swallows the menu input.
func (r *runner) willPromptAfterChild() bool {
	if r.opts.NoInteractive {
		return false
	}
	if r.opts.Apply || r.opts.SavePath != "" || r.opts.Stdout || r.opts.JSON {
		return false
	}
	if r.opts.Interactive {
		return true
	}
	return StdinIsTTY(r.io.Stdin)
}

func (r *runner) canPrompt() bool {
	if r.opts.NoInteractive {
		return false
	}
	if r.opts.JSON && !r.opts.Interactive {
		return false
	}
	if r.opts.Interactive {
		return true
	}
	return StdinIsTTY(r.io.Stdin)
}

func (r *runner) setupTempWorktree(ctx context.Context) error {
	parent := r.opts.WorktreeDir
	if parent == "" {
		parent = filepath.Join(os.TempDir(), "patchrun")
	}
	if err := copyx.EnsureWritableDir(parent); err != nil {
		return fmt.Errorf("create temp parent: %w", err)
	}
	r.tempParent = parent

	r.runID = newRunID(r.opts.Name, r.originalRoot)
	r.tempWorktree = filepath.Join(parent, r.runID)

	if err := r.repoGit.AddDetachedWorktree(ctx, r.tempWorktree, r.originalHead); err != nil {
		return fmt.Errorf("git worktree add: %w", err)
	}

	g, err := gitx.NewWithBin(r.tempWorktree, r.opts.GitBin)
	if err != nil {
		return err
	}
	g.Verbose = r.opts.Verbose
	g.Logger = func(format string, args ...interface{}) { r.verboseLog(format, args...) }
	r.tempGit = g
	return nil
}

func (r *runner) replayBaseline(ctx context.Context) (string, error) {
	// A. Tracked staged+unstaged changes
	trackedPatch, err := r.repoGit.DiffBinary(ctx, "HEAD")
	if err != nil {
		return "", fmt.Errorf("diff HEAD: %w", err)
	}
	if len(trackedPatch) > 0 {
		if err := r.tempGit.ApplyBytesToIndex(ctx, trackedPatch); err != nil {
			return "", fmt.Errorf("apply baseline patch: %w", err)
		}
	}

	// B. Untracked non-ignored files
	untracked, err := r.repoGit.LsUntracked(ctx)
	if err != nil {
		return "", fmt.Errorf("ls-untracked: %w", err)
	}
	for _, rel := range untracked {
		if rel == "" {
			continue
		}
		src := filepath.Join(r.originalRoot, rel)
		dst := filepath.Join(r.tempWorktree, rel)
		if err := copyx.CopyFilePreserve(src, dst); err != nil {
			return "", fmt.Errorf("copy untracked %s: %w", rel, err)
		}
	}

	// C. Stage and commit baseline
	if err := r.tempGit.StageAll(ctx, false); err != nil {
		return "", fmt.Errorf("stage baseline: %w", err)
	}
	sha, err := r.tempGit.CommitBaseline(ctx, "patchrun baseline")
	if err != nil {
		return "", fmt.Errorf("commit baseline: %w", err)
	}
	return sha, nil
}

func (r *runner) printBanner() {
	if r.opts.Quiet || r.opts.JSON {
		return
	}
	c := r.colorErr
	fmt.Fprintln(r.io.Stderr, c.Bold("patchrun"))
	fmt.Fprintf(r.io.Stderr, "repo: %s\n", r.originalRoot)
	shortHead := r.originalHead
	if len(shortHead) > 7 {
		shortHead = shortHead[:7]
	}
	fmt.Fprintf(r.io.Stderr, "base: %s %s%s\n",
		shortHead,
		r.branch,
		dirtyTag(r.originalDirty, c))
	fmt.Fprintf(r.io.Stderr, "temp: %s\n", r.tempWorktree)
	fmt.Fprintf(r.io.Stderr, "\nRunning:\n  %s\n\n", strings.Join(r.opts.Command, " "))
}

func dirtyTag(dirty bool, c *textui.Colorizer) string {
	if !dirty {
		return ""
	}
	return " " + c.Yellow("(dirty baseline)")
}

func (r *runner) runChildCommand(ctx context.Context) run.Result {
	tempCwd := filepath.Join(r.tempWorktree, filepath.FromSlash(r.relCwd))
	if r.relCwd == "." {
		tempCwd = r.tempWorktree
	}
	// Mirror an empty subdirectory if the user's cwd isn't represented in temp
	// (e.g. they ran inside an empty or untracked-empty directory).
	if _, err := os.Stat(tempCwd); os.IsNotExist(err) {
		_ = os.MkdirAll(tempCwd, 0o755)
	}
	env := os.Environ()
	env = append(env,
		"PATCHRUN=1",
		"PATCHRUN_WORKTREE="+r.tempWorktree,
		"PATCHRUN_ORIGINAL_ROOT="+r.originalRoot,
		"PATCHRUN_BASE="+r.baselineRef,
		"PATCHRUN_RUN_ID="+r.runID,
	)

	// Share stdin with the child only when we know we won't read from it for
	// the post-run menu. exec.Cmd starts a goroutine that drains the supplied
	// Reader into the child's pipe even if the child never reads — so if we
	// pass r.io.Stdin to the child and then try to prompt, the prompter sees
	// EOF.
	var stdin io.Reader
	if !r.willPromptAfterChild() {
		stdin = r.io.Stdin
	}

	return run.Run(ctx, run.Spec{
		Args:    r.opts.Command,
		Dir:     tempCwd,
		Env:     env,
		Stdin:   stdin,
		Stdout:  r.io.Stderr, // stream child stdout to our stderr so stdout is reserved for json/patch
		Stderr:  r.io.Stderr,
		Timeout: r.opts.CommandTimeout,
	})
}

// pathspecs builds the include/exclude pathspec slice for git diff -- args.
func (r *runner) pathspecs() []string {
	if len(r.opts.Includes) == 0 && len(r.opts.Excludes) == 0 {
		return nil
	}
	var specs []string
	if len(r.opts.Includes) == 0 {
		specs = append(specs, ".")
	} else {
		specs = append(specs, r.opts.Includes...)
	}
	for _, e := range r.opts.Excludes {
		specs = append(specs, ":(exclude)"+e)
	}
	return specs
}

func (r *runner) capturePatch(ctx context.Context) ([]byte, error) {
	addArgs := []string{"add", "-A"}
	if r.opts.IncludeIgnored {
		// `git add -A -f` would force-add ignored files. We only want command-created
		// files that happen to be ignored, but git's add cannot easily distinguish
		// "ignored existed at baseline" from "ignored created by command"; baseline
		// however already captured pre-existing tracked state, and ignored files
		// were never copied into temp. So any ignored file present now was created
		// by the command. Force-add them.
		addArgs = []string{"add", "-A", "-f"}
	}
	if _, err := r.tempGit.RunBytes(ctx, addArgs...); err != nil {
		return nil, fmt.Errorf("stage temp changes: %w", err)
	}
	patch, err := r.tempGit.DiffBinaryCached(ctx, r.baselineRef, r.pathspecs())
	if err != nil {
		return nil, fmt.Errorf("diff cached: %w", err)
	}
	return patch, nil
}

func (r *runner) parseSummary(ctx context.Context) error {
	ns, err := r.tempGit.DiffNameStatusCached(ctx, r.baselineRef, r.pathspecs())
	if err != nil {
		return err
	}
	nsEntries, err := textui.ParseNameStatusZ(ns)
	if err != nil {
		return err
	}
	r.nameStatusEntries = nsEntries

	num, err := r.tempGit.DiffNumstatCached(ctx, r.baselineRef, r.pathspecs())
	if err != nil {
		return err
	}
	numEntries, err := textui.ParseNumstatZ(num)
	if err != nil {
		return err
	}
	r.numstatEntries = numEntries
	r.totals = textui.Sum(numEntries)
	return nil
}

func (r *runner) printSummary() {
	if r.opts.JSON {
		return
	}
	textui.WriteSummary(r.io.Stderr, r.colorErr, r.nameStatusEntries, r.totals, r.opts.Stat)
	fmt.Fprintln(r.io.Stderr)
}

// dispatchActions returns the resulting exit code.
func (r *runner) dispatchActions(ctx context.Context) int {
	// Non-interactive paths first.
	if r.opts.JSON {
		// JSON implies "save patch automatically and report path", unless
		// the user asked for apply/save/stdout in combination.
		return r.dispatchNonInteractive(ctx)
	}
	if !r.canPrompt() || r.opts.Apply || r.opts.SavePath != "" || r.opts.Stdout || r.opts.NoInteractive {
		return r.dispatchNonInteractive(ctx)
	}
	return r.dispatchInteractive(ctx)
}

func (r *runner) dispatchNonInteractive(ctx context.Context) int {
	// 1. Save?
	savedPath := r.opts.SavePath
	if savedPath != "" {
		if err := gitx.SavePatchAtomic(savedPath, r.patchBytes); err != nil {
			fmt.Fprintf(r.io.Stderr, "error: save patch: %v\n", err)
			return ExitGeneralFailure
		}
		r.savedPath = savedPath
		r.writeSidecar(savedPath)
		r.logHuman("saved patch: %s", savedPath)
	}

	// 2. Stdout?
	if r.opts.Stdout {
		if _, err := r.io.Stdout.Write(r.patchBytes); err != nil {
			fmt.Fprintf(r.io.Stderr, "error: write stdout: %v\n", err)
			return ExitGeneralFailure
		}
	}

	// 3. Apply?
	if r.opts.Apply {
		return r.applyToOriginal(ctx)
	}

	// 4. Default fallback: save to .patchrun if no action was requested
	if !r.opts.Stdout && savedPath == "" && !r.opts.Apply && !r.opts.JSON {
		defaultPath := defaultSavePath(r.originalRoot)
		if err := gitx.SavePatchAtomic(defaultPath, r.patchBytes); err != nil {
			fmt.Fprintf(r.io.Stderr, "error: save patch: %v\n", err)
			return ExitGeneralFailure
		}
		r.savedPath = defaultPath
		r.writeSidecar(defaultPath)
		r.logHuman("saved patch: %s", relativePath(r.originalRoot, defaultPath))
	}
	if r.opts.JSON && r.savedPath == "" {
		// Always save in JSON mode if no action was requested.
		defaultPath := defaultSavePath(r.originalRoot)
		if err := gitx.SavePatchAtomic(defaultPath, r.patchBytes); err != nil {
			fmt.Fprintf(r.io.Stderr, "error: save patch: %v\n", err)
			return ExitGeneralFailure
		}
		r.savedPath = defaultPath
		r.writeSidecar(defaultPath)
	}

	if r.opts.ShowDiff {
		_ = textui.ShowPatch(r.patchBytes, r.io.Stderr, false)
	}
	return ExitOK
}

func (r *runner) dispatchInteractive(ctx context.Context) int {
	prompter := NewPrompter(r.io.Stdin, r.io.Stderr)
	for {
		action, err := prompter.AskMenu(ActionView)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ExitUserDiscard
			}
			fmt.Fprintf(r.io.Stderr, "error: %v\n", err)
			return ExitGeneralFailure
		}
		switch action {
		case ActionApply:
			return r.applyToOriginal(ctx)
		case ActionSave:
			defaultPath := defaultSavePath(r.originalRoot)
			path, perr := prompter.AskPath(relativePath(r.originalRoot, defaultPath))
			if perr != nil {
				fmt.Fprintf(r.io.Stderr, "error: %v\n", perr)
				return ExitGeneralFailure
			}
			abs := path
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(r.originalCwd, path)
			}
			if err := gitx.SavePatchAtomic(abs, r.patchBytes); err != nil {
				fmt.Fprintf(r.io.Stderr, "error: save: %v\n", err)
				return ExitGeneralFailure
			}
			r.savedPath = abs
			r.writeSidecar(abs)
			fmt.Fprintf(r.io.Stderr, "saved: %s\n", abs)
		case ActionView:
			if len(r.patchBytes) > 100*1024*1024 {
				fmt.Fprintln(r.io.Stderr, r.colorErr.Yellow("patch is very large; skipping view. Use save."))
				continue
			}
			if len(r.patchBytes) > 10*1024*1024 {
				ok, _ := prompter.Confirm(fmt.Sprintf("patch is %.1f MB. View?", float64(len(r.patchBytes))/1024/1024), false)
				if !ok {
					continue
				}
			}
			_ = textui.ShowPatch(r.patchBytes, r.io.Stderr, true)
		case ActionKeep:
			r.keep = true
			fmt.Fprintf(r.io.Stderr, "keeping worktree: %s\n", r.tempWorktree)
		case ActionDiscard, ActionQuit:
			if r.cmdResult.ExitCode != 0 {
				return ExitChildFailed
			}
			if len(r.patchBytes) > 0 {
				return ExitUserDiscard
			}
			return ExitOK
		case ActionNone:
			// re-prompt
		}
	}
}

func (r *runner) applyToOriginal(ctx context.Context) int {
	// Re-read original repo state to detect drift.
	_, entries, err := r.repoGit.IsDirty(ctx)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: status: %v\n", err)
		return ExitGeneralFailure
	}
	newHead, err := r.repoGit.HeadSHA(ctx)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: re-read HEAD: %v\n", err)
		return ExitGeneralFailure
	}
	newSig := gitx.StatusFingerprint(entries)
	if newHead != r.originalHead || newSig != r.originalStatusSig {
		path := r.savedPath
		if path == "" {
			path = defaultSavePath(r.originalRoot)
			if err := gitx.SavePatchAtomic(path, r.patchBytes); err == nil {
				r.savedPath = path
				r.writeSidecar(path)
			}
		}
		fmt.Fprintf(r.io.Stderr,
			"%s original working tree changed while patchrun was running. Saved patch instead: %s\n",
			r.colorErr.Red("error:"), relativePath(r.originalRoot, path))
		return ExitApplyFailed
	}

	// Write the patch to a temp file for application.
	patchPath, err := gitx.WriteTempPatch(filepath.Join(r.tempParent, ".applied"), "patchrun-apply", r.patchBytes)
	if err != nil {
		fmt.Fprintf(r.io.Stderr, "error: write temp patch: %v\n", err)
		return ExitGeneralFailure
	}
	defer os.Remove(patchPath)

	if err := r.repoGit.ApplyCheck(ctx, patchPath); err != nil {
		// Save patch automatically if not already.
		if r.savedPath == "" {
			autoPath := defaultSavePath(r.originalRoot)
			if serr := gitx.SavePatchAtomic(autoPath, r.patchBytes); serr == nil {
				r.savedPath = autoPath
				r.writeSidecar(autoPath)
			}
		}
		if r.opts.Apply3Way {
			if applyErr := r.repoGit.Apply(ctx, patchPath, true); applyErr != nil {
				fmt.Fprintf(r.io.Stderr, "error: 3-way apply failed: %v\n", applyErr)
				if r.savedPath != "" {
					fmt.Fprintf(r.io.Stderr, "saved patch: %s\n", relativePath(r.originalRoot, r.savedPath))
				}
				return ExitApplyFailed
			}
			r.applied = true
			fmt.Fprintln(r.io.Stderr, "applied patch with 3-way merge.")
			return ExitOK
		}
		fmt.Fprintf(r.io.Stderr, "%s patch did not apply cleanly.\n", r.colorErr.Red("error:"))
		if r.savedPath != "" {
			fmt.Fprintf(r.io.Stderr, "saved patch: %s\n", relativePath(r.originalRoot, r.savedPath))
			fmt.Fprintf(r.io.Stderr, "try: git apply --3way %s\n", relativePath(r.originalRoot, r.savedPath))
		}
		return ExitApplyFailed
	}

	if err := r.repoGit.Apply(ctx, patchPath, false); err != nil {
		fmt.Fprintf(r.io.Stderr, "error: apply: %v\n", err)
		return ExitApplyFailed
	}
	r.applied = true
	fmt.Fprintln(r.io.Stderr, "applied patch to original repo.")
	return ExitOK
}

func (r *runner) cleanup(ctx context.Context) {
	if r.tempWorktree == "" {
		return
	}
	if r.opts.Keep || r.keep {
		fmt.Fprintf(r.io.Stderr, "kept worktree: %s\n", r.tempWorktree)
		return
	}
	if err := r.repoGit.RemoveWorktree(ctx, r.tempWorktree, r.tempParent, runPrefix(r.opts.Name)); err != nil {
		fmt.Fprintf(r.io.Stderr, "warning: cleanup failed: %v\n", err)
	}
}

func (r *runner) emitJSON(exit int) {
	res := Result{
		Repo:         r.originalRoot,
		Cwd:          r.originalCwd,
		TempWorktree: r.tempWorktree,
		Kept:         r.keep || r.opts.Keep,
		Base: baseResult{
			Head:           r.originalHead,
			Branch:         r.branch,
			Dirty:          r.originalDirty,
			BaselineCommit: r.baselineCommitSHA,
		},
		Command: commandResult{
			Args:       r.opts.Command,
			ExitCode:   r.cmdResult.ExitCode,
			DurationMs: r.cmdResult.Duration.Milliseconds(),
			TimedOut:   r.cmdResult.TimedOut,
		},
		Patch: patchResult{
			Empty:        gitx.PatchIsEmpty(r.patchBytes),
			SavedPath:    r.savedPath,
			Applied:      r.applied,
			FilesChanged: r.totals.Files,
			Insertions:   r.totals.Insertions,
			Deletions:    r.totals.Deletions,
			NameStatus:   nameStatusToJSON(r.nameStatusEntries),
		},
	}
	if exit != ExitOK {
		res.Error = &errorResult{Code: exit, Message: exitMessage(exit)}
	}
	enc := json.NewEncoder(r.io.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(res)
}

func nameStatusToJSON(entries []textui.NameStatusEntry) []nameStatus {
	out := make([]nameStatus, 0, len(entries))
	for _, e := range entries {
		out = append(out, nameStatus{Status: e.Status, Path: e.Path})
	}
	return out
}

func exitMessage(code int) string {
	switch code {
	case ExitGeneralFailure:
		return "general failure"
	case ExitNotInRepo:
		return "not inside Git repo"
	case ExitGitMissing:
		return "git missing"
	case ExitDirty:
		return "dirty working tree"
	case ExitChildFailed:
		return "child command failed"
	case ExitApplyFailed:
		return "patch failed to apply"
	case ExitUserDiscard:
		return "user discarded patch"
	case ExitInvalidUsage:
		return "invalid usage"
	case ExitTimeout:
		return "command timed out"
	}
	return ""
}

func defaultSavePath(repoRoot string) string {
	ts := time.Now().Format("20060102-150405")
	return filepath.Join(repoRoot, ".patchrun", "patchrun-"+ts+".patch")
}

func relativePath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return rel
}

func runPrefix(name string) string {
	if name == "" {
		return "patchrun-"
	}
	return "patchrun-" + sanitizeLabel(name) + "-"
}

func newRunID(name, repoRoot string) string {
	ts := time.Now().Format("20060102-150405")
	suffix := randSuffix(6)
	base := filepath.Base(repoRoot)
	if name != "" {
		return fmt.Sprintf("patchrun-%s-%s-%s-%s", sanitizeLabel(name), base, ts, suffix)
	}
	return fmt.Sprintf("patchrun-%s-%s-%s", base, ts, suffix)
}

func sanitizeLabel(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		out = "run"
	}
	return out
}

var randSrc = rand.New(rand.NewSource(time.Now().UnixNano()))

func randSuffix(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[randSrc.Intn(len(alphabet))]
	}
	return string(b)
}

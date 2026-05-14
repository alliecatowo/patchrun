// Package app implements the patchrun command orchestration.
package app

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// ExitCode values mirror the documented contract in the README.
const (
	ExitOK             = 0
	ExitGeneralFailure = 1
	ExitNotInRepo      = 2
	ExitGitMissing     = 3
	ExitDirty          = 4
	ExitChildFailed    = 5
	ExitApplyFailed    = 6
	ExitUserDiscard    = 7
	ExitInvalidUsage   = 8
	ExitTimeout        = 9
)

// Options is the parsed CLI configuration.
type Options struct {
	Apply            bool
	Apply3Way        bool
	SavePath         string
	Stdout           bool
	JSON             bool
	Keep             bool
	WorktreeDir      string
	Name             string
	AllowDirty       bool
	FailOnDirty      bool
	IncludeIgnored   bool
	Includes         []string
	Excludes         []string
	ShowDiff         bool
	Stat             bool
	StatExplicit     bool // user passed --stat or --no-stat
	Interactive      bool
	NoInteractive    bool
	CommandTimeout   time.Duration
	Quiet            bool
	Verbose          bool
	VersionRequested bool
	HelpRequested    bool

	// Color controls ANSI output. "auto" (default), "always", or "never".
	Color string

	// CompletionShell, if non-empty, asks patchrun to print a shell completion
	// script to stdout and exit. One of "bash", "zsh", "fish".
	CompletionShell string

	// GitBin lets users override the git executable path.
	GitBin string

	// Cwd lets users point patchrun at a different repo without `cd`.
	Cwd string

	// ListRuns prints kept worktrees in --worktree-dir and exits.
	ListRuns bool

	// Prune removes patchrun worktrees in --worktree-dir and exits.
	Prune bool

	// NoSidecar disables the .meta.json sidecar next to saved patches.
	NoSidecar bool

	// Reverse, when true, prints/saves the reverse of the captured patch
	// (so applying it would undo the command's effect).
	Reverse bool

	// Snapshot is a path to dump the entire post-run temp worktree into.
	Snapshot string

	// Execs are additional commands to run inside the temp worktree after
	// the main command. Failures still produce a patch but exit non-zero.
	Execs []string

	// CheckOnly stops after verifying the patch applies cleanly to the
	// original repo. Saves the patch (or stdouts/JSONs it) but does not
	// modify the working tree even when combined with --apply.
	CheckOnly bool

	// IgnoreWhitespace passes --ignore-whitespace to git apply.
	IgnoreWhitespace bool

	// Command is the user command after `--`. May be empty for utility
	// subcommands like --completion or --list-runs.
	Command []string
}

// UsageError indicates a CLI usage problem.
type UsageError struct{ Msg string }

func (e *UsageError) Error() string { return e.Msg }

// HelpRequested signals that --help or `help` was requested.
type HelpError struct{}

func (HelpError) Error() string { return "help requested" }

// VersionRequested signals --version.
type VersionError struct{}

func (VersionError) Error() string { return "version requested" }

// ParseOptions parses argv (without the program name) into Options.
//
// Anything after the first "--" token is collected into Options.Command.
// pflag is used for the flag part. Help and version requests are surfaced as
// typed errors so the caller can format output appropriately.
func ParseOptions(argv []string, helpWriter io.Writer, version string) (*Options, error) {
	flagArgs, commandArgs := splitOnSeparator(argv)

	opts := &Options{}
	fs := pflag.NewFlagSet("patchrun", pflag.ContinueOnError)
	fs.SetOutput(helpWriter)
	fs.SortFlags = false

	fs.BoolVar(&opts.Apply, "apply", false, "Apply patch to original repo after command succeeds")
	fs.BoolVar(&opts.Apply3Way, "apply-3way", false, "Use git apply --3way if normal apply fails")
	fs.StringVar(&opts.SavePath, "save", "", "Save patch to path")
	fs.BoolVar(&opts.Stdout, "stdout", false, "Print patch to stdout")
	fs.BoolVar(&opts.JSON, "json", false, "Print machine-readable result JSON to stdout")
	fs.BoolVar(&opts.Keep, "keep", false, "Keep disposable worktree")
	fs.StringVar(&opts.WorktreeDir, "worktree-dir", "", "Parent directory for temporary worktrees")
	fs.StringVar(&opts.Name, "name", "", "Label this run")
	fs.BoolVar(&opts.AllowDirty, "allow-dirty", false, "Use current dirty working tree as baseline")
	fs.BoolVar(&opts.FailOnDirty, "fail-on-dirty", false, "Refuse to run on a dirty working tree")
	fs.BoolVar(&opts.IncludeIgnored, "include-ignored", false, "Include ignored files created by command")
	fs.StringSliceVar(&opts.Includes, "include", nil, "Include only pathspec, repeatable")
	fs.StringSliceVar(&opts.Excludes, "exclude", nil, "Exclude pathspec, repeatable")
	fs.BoolVar(&opts.ShowDiff, "diff", false, "Show patch after command")
	statFlag := fs.Bool("stat", true, "Show diffstat")
	noStatFlag := fs.Bool("no-stat", false, "Hide diffstat")
	fs.BoolVar(&opts.Interactive, "interactive", false, "Force interactive menu")
	fs.BoolVar(&opts.NoInteractive, "no-interactive", false, "Disable prompts")
	fs.DurationVar(&opts.CommandTimeout, "command-timeout", 0, "Kill command after duration")
	fs.BoolVar(&opts.Quiet, "quiet", false, "Less output")
	fs.BoolVar(&opts.Verbose, "verbose", false, "More output")
	fs.BoolVar(&opts.VersionRequested, "version", false, "Print version")
	fs.StringVar(&opts.Color, "color", "auto", "Color output: auto|always|never")
	fs.StringVar(&opts.CompletionShell, "completion", "", "Print shell completion script and exit: bash|zsh|fish")
	fs.StringVar(&opts.GitBin, "git-bin", "", "Override path to the git executable")
	fs.StringVar(&opts.Cwd, "cwd", "", "Run as if patchrun were invoked from <path>")
	fs.BoolVar(&opts.ListRuns, "list-runs", false, "List kept worktrees under --worktree-dir and exit")
	fs.BoolVar(&opts.Prune, "prune", false, "Remove patchrun worktrees under --worktree-dir and exit")
	fs.BoolVar(&opts.NoSidecar, "no-sidecar", false, "Do not write a .meta.json next to saved patches")
	fs.BoolVar(&opts.Reverse, "reverse", false, "Print/save the reverse of the captured patch")
	fs.StringVar(&opts.Snapshot, "snapshot", "", "Dump the post-run temp worktree to this directory")
	fs.StringSliceVar(&opts.Execs, "exec", nil, "Additional command to run in the worktree, repeatable")
	fs.BoolVar(&opts.CheckOnly, "check", false, "Verify the patch applies cleanly; do not apply")
	fs.BoolVar(&opts.IgnoreWhitespace, "ignore-whitespace", false, "Pass --ignore-whitespace to git apply")
	help := fs.BoolP("help", "h", false, "Show help")

	fs.Usage = func() {
		fmt.Fprint(helpWriter, helpText(version))
	}

	if err := fs.Parse(flagArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return nil, HelpError{}
		}
		return nil, &UsageError{Msg: err.Error()}
	}

	if *help {
		return nil, HelpError{}
	}
	if opts.VersionRequested {
		return nil, VersionError{}
	}

	// Resolve --stat / --no-stat. Defaults to true (show stat in human mode).
	if fs.Changed("no-stat") && *noStatFlag {
		opts.Stat = false
		opts.StatExplicit = true
	} else if fs.Changed("stat") {
		opts.Stat = *statFlag
		opts.StatExplicit = true
	} else {
		opts.Stat = true
	}

	if opts.Quiet && opts.Verbose {
		return nil, &UsageError{Msg: "--quiet and --verbose are mutually exclusive"}
	}
	if opts.AllowDirty && opts.FailOnDirty {
		return nil, &UsageError{Msg: "--allow-dirty and --fail-on-dirty are mutually exclusive"}
	}
	if opts.Interactive && opts.NoInteractive {
		return nil, &UsageError{Msg: "--interactive and --no-interactive are mutually exclusive"}
	}
	switch opts.Color {
	case "auto", "always", "never":
	default:
		return nil, &UsageError{Msg: fmt.Sprintf("invalid --color value %q: want auto|always|never", opts.Color)}
	}
	if opts.CompletionShell != "" {
		switch opts.CompletionShell {
		case "bash", "zsh", "fish":
		default:
			return nil, &UsageError{Msg: fmt.Sprintf("invalid --completion value %q: want bash|zsh|fish", opts.CompletionShell)}
		}
	}

	// Any leftover positional args before `--` are not allowed.
	if leftover := fs.Args(); len(leftover) > 0 {
		return nil, &UsageError{
			Msg: fmt.Sprintf("unexpected positional argument(s) before '--': %s", strings.Join(leftover, " ")),
		}
	}

	// Utility subcommands don't require a command after `--`.
	utility := opts.CompletionShell != "" || opts.ListRuns || opts.Prune
	if len(commandArgs) == 0 && !utility {
		return nil, &UsageError{Msg: "missing command: use 'patchrun [options] -- <command> [args...]'"}
	}
	opts.Command = commandArgs
	return opts, nil
}

func splitOnSeparator(argv []string) (flags []string, command []string) {
	for i, a := range argv {
		if a == "--" {
			return argv[:i], argv[i+1:]
		}
	}
	return argv, nil
}

// helpText returns the patchrun --help screen.
func helpText(version string) string {
	return `patchrun ` + version + `

Run any repo-mutating command in a disposable Git worktree and review the
patch before applying it.

Usage:
  patchrun [options] -- <command> [args...]

Examples:
  patchrun -- npm install
  patchrun -- pnpm dlx shadcn@latest add button
  patchrun --apply -- prettier . --write
  patchrun --save changes.patch -- python scripts/codemod.py
  patchrun --json -- npm install

Options:
  --apply                       Apply patch to original repo after command succeeds
  --apply-3way                  Use git apply --3way if normal apply fails
  --save <path>                 Save patch to path
  --stdout                      Print patch to stdout
  --json                        Print machine-readable result JSON to stdout
  --keep                        Keep disposable worktree
  --worktree-dir <path>         Parent directory for temporary worktrees
  --name <label>                Label this run
  --allow-dirty                 Use current dirty working tree as baseline
  --fail-on-dirty               Refuse dirty working tree
  --include-ignored             Include ignored files created by command
  --include <pathspec>          Include only pathspec, repeatable
  --exclude <pathspec>          Exclude pathspec, repeatable
  --diff                        Show patch after command
  --stat                        Show diffstat (default)
  --no-stat                     Hide diffstat
  --interactive                 Force interactive menu
  --no-interactive              Disable prompts
  --command-timeout <duration>  Kill command after duration (e.g. 30s, 5m)
  --color <mode>                Color output: auto|always|never
  --no-sidecar                  Skip the .meta.json sidecar next to saved patches
  --reverse                     Print/save the reverse of the captured patch
  --snapshot <dir>              Dump the post-run worktree into <dir>
  --exec <command>              Additional command to run in worktree (repeatable)
  --check                       Verify patch applies; do not modify the working tree
  --ignore-whitespace           Pass --ignore-whitespace to git apply
  --git-bin <path>              Override git binary
  --cwd <path>                  Run as if invoked from <path>
  --list-runs                   List kept worktrees under --worktree-dir
  --prune                       Remove patchrun worktrees under --worktree-dir
  --completion <shell>          Print shell completion (bash|zsh|fish)
  --quiet                       Less output
  --verbose                     More output
  --version                     Print version
  -h, --help                    Show help

patchrun is not a sandbox. The command still runs on your machine with your
user permissions. patchrun only protects your Git working tree from repo-local
file mutations by running inside a disposable copy and returning a patch.
`
}

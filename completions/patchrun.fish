# fish completion for patchrun
#
# Install: copy this file to ~/.config/fish/completions/patchrun.fish

# Helper: did the user already type `--`?
function __patchrun_after_separator
    set -l tokens (commandline -opc)
    for tok in $tokens
        if test "$tok" = "--"
            return 0
        end
    end
    return 1
end

# Suppress patchrun option completion once we are past `--`.
complete -c patchrun -n 'not __patchrun_after_separator' -l apply -d 'Apply patch to original repo after command succeeds'
complete -c patchrun -n 'not __patchrun_after_separator' -l apply-3way -d 'Use git apply --3way if normal apply fails'
complete -c patchrun -n 'not __patchrun_after_separator' -l save -r -d 'Save patch to path'
complete -c patchrun -n 'not __patchrun_after_separator' -l stdout -d 'Print patch to stdout'
complete -c patchrun -n 'not __patchrun_after_separator' -l json -d 'Print machine-readable JSON to stdout'
complete -c patchrun -n 'not __patchrun_after_separator' -l keep -d 'Keep disposable worktree'
complete -c patchrun -n 'not __patchrun_after_separator' -l worktree-dir -r -d 'Parent directory for temporary worktrees'
complete -c patchrun -n 'not __patchrun_after_separator' -l name -r -d 'Label this run'
complete -c patchrun -n 'not __patchrun_after_separator' -l allow-dirty -d 'Use current dirty working tree as baseline'
complete -c patchrun -n 'not __patchrun_after_separator' -l fail-on-dirty -d 'Refuse dirty working tree'
complete -c patchrun -n 'not __patchrun_after_separator' -l include-ignored -d 'Include ignored files created by command'
complete -c patchrun -n 'not __patchrun_after_separator' -l include -r -d 'Include only pathspec, repeatable'
complete -c patchrun -n 'not __patchrun_after_separator' -l exclude -r -d 'Exclude pathspec, repeatable'
complete -c patchrun -n 'not __patchrun_after_separator' -l diff -d 'Show patch after command'
complete -c patchrun -n 'not __patchrun_after_separator' -l stat -d 'Show diffstat'
complete -c patchrun -n 'not __patchrun_after_separator' -l no-stat -d 'Hide diffstat'
complete -c patchrun -n 'not __patchrun_after_separator' -l interactive -d 'Force interactive menu'
complete -c patchrun -n 'not __patchrun_after_separator' -l no-interactive -d 'Disable prompts'
complete -c patchrun -n 'not __patchrun_after_separator' -l command-timeout -r -d 'Kill command after duration'
complete -c patchrun -n 'not __patchrun_after_separator' -l quiet -d 'Less output'
complete -c patchrun -n 'not __patchrun_after_separator' -l verbose -d 'More output'
complete -c patchrun -n 'not __patchrun_after_separator' -l version -d 'Print version'
complete -c patchrun -n 'not __patchrun_after_separator' -s h -l help -d 'Show help'

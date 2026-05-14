#compdef patchrun
# zsh completion for patchrun
#
# Install:
#   cp patchrun.zsh ~/.zsh/completions/_patchrun
#   echo 'fpath=(~/.zsh/completions $fpath)' >> ~/.zshrc
#   autoload -Uz compinit && compinit

_patchrun() {
    local -a opts
    opts=(
        '--apply[apply patch to original repo after command succeeds]'
        '--apply-3way[use git apply --3way if normal apply fails]'
        '--save=[save patch to path]:patch file:_files'
        '--stdout[print patch to stdout]'
        '--json[print machine-readable JSON to stdout]'
        '--keep[keep disposable worktree]'
        '--worktree-dir=[parent directory for temporary worktrees]:directory:_directories'
        '--name=[label this run]:label:'
        '--allow-dirty[use current dirty working tree as baseline]'
        '--fail-on-dirty[refuse dirty working tree]'
        '--include-ignored[include ignored files created by command]'
        '*--include=[include only pathspec, repeatable]:pathspec:_files'
        '*--exclude=[exclude pathspec, repeatable]:pathspec:_files'
        '--diff[show patch after command]'
        '--stat[show diffstat]'
        '--no-stat[hide diffstat]'
        '--interactive[force interactive menu]'
        '--no-interactive[disable prompts]'
        '--command-timeout=[kill command after duration]:duration (e.g. 30s, 5m):'
        '--quiet[less output]'
        '--verbose[more output]'
        '--version[print version]'
        '(-h --help)'{-h,--help}'[show help]'
    )

    # Stop completing patchrun options once we see `--`; delegate to the command.
    local idx
    idx=$words[(i)--]
    if (( idx < CURRENT )); then
        words=( "${(@)words[idx+1,$#words]}" )
        (( CURRENT -= idx ))
        _normal
        return
    fi

    _arguments -s -S "${opts[@]}" '*::command and args:->cmd'
    case $state in
        cmd) _normal ;;
    esac
}

_patchrun "$@"

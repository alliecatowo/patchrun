# bash completion for patchrun
#
# Install:
#   sudo cp patchrun.bash /etc/bash_completion.d/patchrun
# or:
#   source /path/to/patchrun.bash  # in your ~/.bashrc

_patchrun() {
    local cur prev words cword
    _init_completion || return

    local opts="
        --apply --apply-3way
        --save --stdout --json
        --keep --worktree-dir
        --name
        --allow-dirty --fail-on-dirty
        --include-ignored --include --exclude
        --diff --stat --no-stat
        --interactive --no-interactive
        --command-timeout
        --quiet --verbose
        --version --help
    "

    case "$prev" in
        --save|--worktree-dir)
            _filedir
            return 0
            ;;
        --include|--exclude|--name|--command-timeout)
            return 0
            ;;
    esac

    # Once we see `--`, hand off to default command completion.
    local i
    for ((i=1; i<COMP_CWORD; i++)); do
        if [[ "${COMP_WORDS[i]}" == "--" ]]; then
            COMP_WORDS=("${COMP_WORDS[@]:i+1}")
            COMP_CWORD=$((COMP_CWORD - i - 1))
            _command_offset 0
            return 0
        fi
    done

    if [[ "$cur" == --* ]]; then
        COMPREPLY=($(compgen -W "$opts" -- "$cur"))
        return 0
    fi
    COMPREPLY=($(compgen -W "$opts --" -- "$cur"))
}

complete -F _patchrun patchrun

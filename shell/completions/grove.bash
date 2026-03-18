# Bash completion for grove
# Source this file or copy it to /etc/bash_completion.d/ or /usr/local/etc/bash_completion.d/

_grove_completion() {
    local cur prev words cword
    # Check if bash-completion is available
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion || return
    else
        # Fallback for systems without bash-completion
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        words=("${COMP_WORDS[@]}")
        cword=$COMP_CWORD
    fi
    
    local commands="ls new to rm here last freeze resume time fetch issues prs up down logs restart config version init"
    
    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return 0
    fi
    
    case "${words[1]}" in
        to|rm|resume)
            # Complete with worktree names
            local worktrees=$(grove ls -q 2>/dev/null)
            COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
            ;;
        freeze)
            # Complete with worktree names or --all flag
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--all" -- "$cur"))
            else
                local worktrees=$(grove ls -q 2>/dev/null)
                COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
            fi
            ;;
        time)
            if [[ $cword -eq 2 ]]; then
                if [[ "$cur" == -* ]]; then
                    COMPREPLY=($(compgen -W "--all --json" -- "$cur"))
                else
                    COMPREPLY=($(compgen -W "week" -- "$cur"))
                fi
            else
                COMPREPLY=($(compgen -W "--json" -- "$cur"))
            fi
            ;;
        fetch)
            if [[ $cword -eq 2 ]]; then
                COMPREPLY=($(compgen -W "pr/ issue/ is/" -- "$cur"))
            fi
            ;;
        issues|prs)
            if [[ "$cur" == -* ]]; then
                COMPREPLY=($(compgen -W "--state --label --assignee --author --limit" -- "$cur"))
            elif [[ "$prev" == "--state" ]]; then
                COMPREPLY=($(compgen -W "open closed all" -- "$cur"))
            fi
            ;;
        init)
            COMPREPLY=($(compgen -W "zsh bash" -- "$cur"))
            ;;
        logs|restart)
            # Service names would require context
            ;;
    esac
}

complete -F _grove_completion grove
complete -F _grove_completion w

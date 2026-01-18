# Grove function wrapper for bash
grove() {
    local output
    output=$("$__GROVE_BIN" "$@" 2>&1)
    local exit_code=$?
    
    # Check if output contains directory change instruction
    # Extract only the first line starting with cd:
    if [[ "$output" == cd:* ]]; then
        local target_dir
        target_dir=$(echo "$output" | grep "^cd:" | head -n1 | sed 's/^cd://')
        
        # Change directory
        cd "$target_dir" || return 1
        
        # Print any output that's not the cd: line
        local other_output
        other_output=$(echo "$output" | grep -v "^cd:")
        if [[ -n "$other_output" ]]; then
            echo "$other_output"
        fi
    else
        # Just print the output
        echo "$output"
    fi
    
    return $exit_code
}

# Tab completion for grove
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
    
    local commands="ls new to rm here last config version init"
    
    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return 0
    fi
    
    case "${words[1]}" in
        to|rm)
            # Complete with worktree names
            local worktrees=$(git worktree list --porcelain 2>/dev/null | awk '/^worktree / {print $2}' | xargs -n1 basename)
            COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
            ;;
        init)
            COMPREPLY=($(compgen -W "zsh bash" -- "$cur"))
            ;;
    esac
}

complete -F _grove_completion grove

# Alias
alias w=grove

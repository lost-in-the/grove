# Grove function wrapper for bash
grove() {
    local output
    output=$("$__GROVE_BIN" "$@" 2>&1)
    local exit_code=$?
    
    # Check if output contains directory change instruction
    if [[ "$output" == cd:* ]]; then
        local target_dir="${output#cd:}"
        # Remove the cd: line from output
        output=$(echo "$output" | grep -v "^cd:")
        
        # Print any remaining output
        if [[ -n "$output" ]]; then
            echo "$output"
        fi
        
        # Change directory
        cd "$target_dir" || return 1
    else
        # Just print the output
        echo "$output"
    fi
    
    return $exit_code
}

# Tab completion for grove
_grove_completion() {
    local cur prev words cword
    _init_completion || return
    
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

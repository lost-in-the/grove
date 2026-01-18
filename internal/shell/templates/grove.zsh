# Grove function wrapper for zsh
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
    local -a worktrees
    worktrees=($(git worktree list --porcelain 2>/dev/null | awk '/^worktree / {print $2}' | xargs -n1 basename))
    
    local -a commands
    commands=(
        'ls:List all worktrees'
        'new:Create a new worktree'
        'to:Switch to a worktree'
        'rm:Remove a worktree'
        'here:Show current worktree'
        'last:Switch to previous worktree'
        'config:Show configuration'
        'version:Show version'
        'init:Generate shell integration'
    )
    
    if (( CURRENT == 2 )); then
        _describe 'command' commands
    elif (( CURRENT == 3 )); then
        case "${words[2]}" in
            to|rm)
                _describe 'worktree' worktrees
                ;;
            init)
                _values 'shell' 'zsh' 'bash'
                ;;
        esac
    fi
}

compdef _grove_completion grove

# Alias
alias w=grove

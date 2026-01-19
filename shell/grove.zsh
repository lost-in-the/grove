# Grove function wrapper for zsh
grove() {
    # Set environment variable to indicate we're in the shell wrapper
    local output exit_code
    output=$(GROVE_SHELL=1 "$__GROVE_BIN" "$@" 2>&1)
    exit_code=$?
    
    # Parse output line by line for directives
    local should_cd=0
    local cd_target=""
    local other_lines=""
    
    while IFS= read -r line; do
        if [[ "$line" == cd:* ]]; then
            # Extract directory path from cd: directive
            cd_target="${line#cd:}"
            should_cd=1
        else
            # Collect non-directive output
            if [[ -n "$other_lines" ]]; then
                other_lines="${other_lines}"$'\n'"${line}"
            else
                other_lines="$line"
            fi
        fi
    done <<< "$output"
    
    # Execute directory change if directive was found
    if [[ $should_cd -eq 1 && -n "$cd_target" ]]; then
        cd "$cd_target" || return 1
    fi
    
    # Print any non-directive output
    if [[ -n "$other_lines" ]]; then
        echo "$other_lines"
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

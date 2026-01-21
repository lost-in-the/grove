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
        if [[ "$line" == GROVE_CD:* ]]; then
            # Extract directory path from GROVE_CD: directive (V2)
            cd_target="${line#GROVE_CD:}"
            should_cd=1
        elif [[ "$line" == cd:* ]]; then
            # Extract directory path from cd: directive (legacy)
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
    # Use grove ls -q for short names (consistent with display)
    worktrees=($(GROVE_SHELL=1 "$__GROVE_BIN" ls -q 2>/dev/null))

    local -a commands
    commands=(
        'ls:List all worktrees'
        'new:Create a new worktree'
        'to:Switch to a worktree'
        'rm:Remove a worktree'
        'here:Show current worktree'
        'last:Switch to previous worktree'
        'fork:Fork current worktree'
        'compare:Compare worktrees'
        'apply:Apply changes from another worktree'
        'sync:Sync environment worktrees'
        'clean:Remove old unused worktrees'
        'repair:Repair state inconsistencies'
        'setup:Initialize grove project'
        'fetch:Create worktree from issue/PR'
        'issues:Browse GitHub issues'
        'prs:Browse GitHub PRs'
        'up:Start Docker containers'
        'down:Stop Docker containers'
        'logs:View Docker logs'
        'restart:Restart Docker containers'
        'config:Show configuration'
        'version:Show version'
        'init:Generate shell integration'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
    elif (( CURRENT == 3 )); then
        case "${words[2]}" in
            to|rm|compare|sync)
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

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
        'freeze:Freeze a worktree'
        'resume:Resume a frozen worktree'
        'time:Show time tracking information'
        'fetch:Fetch PR or issue as worktree'
        'issues:Browse and select issues'
        'prs:Browse and select pull requests'
        'up:Start Docker containers'
        'down:Stop Docker containers'
        'logs:View container logs'
        'restart:Restart containers'
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
            resume)
                _describe 'worktree' worktrees
                ;;
            freeze)
                _describe 'worktree' worktrees
                _arguments \
                    '--all[Freeze all worktrees except current]'
                ;;
            time)
                _arguments \
                    '--all[Show time for all worktrees]' \
                    '--json[Output in JSON format]'
                _describe 'command' '(week:Show weekly summary)'
                ;;
            fetch)
                _describe 'type' '(pr:Fetch pull request issue:Fetch issue)'
                ;;
            issues|prs)
                _arguments \
                    '--state=[Filter by state]:state:(open closed all)' \
                    '--label=[Filter by label]:label:' \
                    '--assignee=[Filter by assignee]:assignee:' \
                    '--author=[Filter by author]:author:' \
                    '--limit=[Limit results]:limit:'
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

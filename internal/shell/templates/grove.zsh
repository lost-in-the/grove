# Grove function wrapper for zsh
grove() {
    # Bare "grove" with no args launches TUI — run directly, no capture
    if [[ $# -eq 0 ]]; then
        local cd_file=$(mktemp "${TMPDIR:-/tmp}/grove-cd.XXXXXX")
        GROVE_SHELL=1 GROVE_CD_FILE="$cd_file" "$__GROVE_BIN"
        local exit_code=$?
        if [[ -s "$cd_file" ]]; then
            local target=$(cat "$cd_file")
            cd "$target" 2>/dev/null
        fi
        rm -f "$cd_file"
        return $exit_code
    fi

    # Only directive-producing commands need output capture.
    # All other commands run directly for streaming support.
    case "$1" in
        to|last|fork|fetch|attach)
            # Capture output and parse for cd:/tmux-attach: directives
            local output exit_code
            output=$(GROVE_SHELL=1 "$__GROVE_BIN" "$@" 2>&1)
            exit_code=$?

            local should_cd=0
            local cd_target=""
            local tmux_session=""
            local other_lines=""

            while IFS= read -r line; do
                if [[ "$line" == GROVE_CD:* ]]; then
                    cd_target="${line#GROVE_CD:}"
                    should_cd=1
                elif [[ "$line" == cd:* ]]; then
                    cd_target="${line#cd:}"
                    should_cd=1
                elif [[ "$line" == tmux-attach:* ]]; then
                    tmux_session="${line#tmux-attach:}"
                else
                    if [[ -n "$other_lines" ]]; then
                        other_lines="${other_lines}"$'\n'"${line}"
                    else
                        other_lines="$line"
                    fi
                fi
            done <<< "$output"

            if [[ $should_cd -eq 1 && -n "$cd_target" ]]; then
                cd "$cd_target" || return 1
            fi

            if [[ -n "$other_lines" ]]; then
                echo "$other_lines"
            fi

            if [[ -n "$tmux_session" ]]; then
                tmux attach -t "$tmux_session"
            fi

            return $exit_code
            ;;
        *)
            # All other commands: run directly (streaming-safe)
            GROVE_SHELL=1 "$__GROVE_BIN" "$@"
            return $?
            ;;
    esac
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
        'init:Initialize grove project'
        'setup:Initialize grove project (alias for init)'
        'attach:Attach to tmux session for a worktree'
        'fetch:Create worktree from issue/PR'
        'issues:Browse GitHub issues'
        'prs:Browse GitHub PRs'
        'up:Start Docker containers'
        'down:Stop Docker containers'
        'logs:View Docker logs'
        'restart:Restart Docker containers'
        'test:Run tests in a worktree'
        'config:Show configuration'
        'doctor:Check system health and configuration'
        'agent-status:Show active isolated stacks'
        'version:Show version'
        'install:Generate shell integration'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
    elif (( CURRENT == 3 )); then
        case "${words[2]}" in
            to|rm|compare|sync|test|apply|attach)
                _describe 'worktree' worktrees
                ;;
            install)
                _values 'shell' 'zsh' 'bash'
                ;;
        esac
    fi
}

compdef _grove_completion grove

# Alias
alias w=grove

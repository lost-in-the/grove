# Grove function wrapper for zsh
grove() {
    # Recursion guard: if __GROVE_BIN is empty or the bare word "grove",
    # calling it would invoke this function again — infinite recursion.
    if [[ -z "$__GROVE_BIN" || "$__GROVE_BIN" == "grove" ]]; then
        echo "grove: binary not found (is grove on your PATH?)" >&2
        return 127
    fi

    # Bare "grove" with no args launches TUI — run directly, no capture
    if [[ $# -eq 0 ]]; then
        local cd_file=$(mktemp "${TMPDIR:-/tmp}/grove-cd.XXXXXX")
        GROVE_SHELL=1 GROVE_SHELL_VERSION="$__GROVE_SHELL_VERSION" GROVE_CD_FILE="$cd_file" "$__GROVE_BIN"
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
        to|t|switch|last|la|fork|fo|split|fetch|f|attach|join|a|j|open|o|up|u|run|kick|k|restart)
            # Capture output and parse for cd:/tmux-attach:/env: directives
            local output exit_code
            output=$(GROVE_SHELL=1 GROVE_SHELL_VERSION="$__GROVE_SHELL_VERSION" "$__GROVE_BIN" "$@")
            exit_code=$?

            local should_cd=0
            local cd_target=""
            local tmux_session=""
            local tmux_cc=0
            local other_lines=""

            while IFS= read -r line; do
                if [[ "$line" == cd:* ]]; then
                    cd_target="${line#cd:}"
                    should_cd=1
                elif [[ "$line" == tmux-attach-cc:* ]]; then
                    tmux_session="${line#tmux-attach-cc:}"
                    tmux_cc=1
                elif [[ "$line" == tmux-attach:* ]]; then
                    tmux_session="${line#tmux-attach:}"
                    tmux_cc=0
                elif [[ "$line" == env:* ]]; then
                    export "${line#env:}"
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
                if [[ "$tmux_cc" -eq 1 ]]; then
                    tmux -CC attach -t "$tmux_session"
                else
                    tmux attach -t "$tmux_session"
                fi
            fi

            return $exit_code
            ;;
        *)
            # All other commands: run directly (streaming-safe)
            GROVE_SHELL=1 GROVE_SHELL_VERSION="$__GROVE_SHELL_VERSION" "$__GROVE_BIN" "$@"
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
        'l:List all worktrees (alias)'
        'new:Create a new worktree'
        'n:Create a new worktree (alias)'
        'to:Switch to a worktree'
        't:Switch to a worktree (alias)'
        'rm:Remove a worktree'
        'here:Show current worktree'
        'h:Show current worktree (alias)'
        'last:Switch to previous worktree'
        'la:Switch to previous worktree (alias)'
        'fork:Fork current worktree'
        'fo:Fork current worktree (alias)'
        'diff:Diff worktrees'
        'd:Diff worktrees (alias)'
        'graft:Graft changes from another worktree'
        'g:Graft changes (alias)'
        'sync:Sync environment worktrees'
        's:Sync environment worktrees (alias)'
        'trim:Trim old unused worktrees'
        'tm:Trim old unused worktrees (alias)'
        'repair:Repair state inconsistencies'
        'init:Initialize grove project'
        'setup:Set up shell integration'
        'join:Join tmux session for a worktree'
        'j:Join tmux session (alias)'
        'fetch:Create worktree from issue/PR'
        'f:Create worktree from issue/PR (alias)'
        'issues:Browse GitHub issues'
        'prs:Browse GitHub PRs'
        'up:Start Docker containers'
        'u:Start Docker containers (alias)'
        'down:Stop Docker containers'
        'do:Stop Docker containers (alias)'
        'logs:View Docker logs'
        'lo:View Docker logs (alias)'
        'kick:Kick (restart) Docker containers'
        'k:Kick Docker containers (alias)'
        'test:Run tests in a worktree'
        'tt:Run tests (alias)'
        'which:Show current worktree and service status'
        'config:Show configuration'
        'doctor:Check system health and configuration'
        'open:Open a worktree session'
        'o:Open a worktree session (alias)'
        'ps:Show active stacks'
        'agent-status:Show active isolated stacks'
        'version:Show version'
        'install:Generate shell integration'
    )

    if (( CURRENT == 2 )); then
        _describe 'command' commands
    elif (( CURRENT == 3 )); then
        case "${words[2]}" in
            to|t|switch|rm|diff|d|compare|sync|s|test|tt|graft|g|apply|join|j|attach|a|open|o)
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

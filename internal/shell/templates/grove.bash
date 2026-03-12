# Grove function wrapper for bash
grove() {
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

    local commands="ls l new n to t rm here h last la fork fo diff d graft g sync s trim tm repair init setup join j fetch f open o issues prs up u down do logs lo kick k test tt which config doctor ps agent-status version install"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return 0
    fi

    case "${words[1]}" in
        to|t|switch|rm|diff|d|compare|sync|s|test|tt|graft|g|apply|join|j|attach|a|open|o)
            # Complete with worktree short names (using grove ls -q for consistency)
            local worktrees=$(GROVE_SHELL=1 "$__GROVE_BIN" ls -q 2>/dev/null)
            COMPREPLY=($(compgen -W "$worktrees" -- "$cur"))
            ;;
        install)
            COMPREPLY=($(compgen -W "zsh bash" -- "$cur"))
            ;;
    esac
}

complete -F _grove_completion grove

# Alias
alias w=grove

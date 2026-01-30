# Grove function wrapper for bash
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

    local commands="ls new to rm here last fork compare apply sync clean repair init setup fetch issues prs up down logs restart config version install"

    if [[ $cword -eq 1 ]]; then
        COMPREPLY=($(compgen -W "$commands" -- "$cur"))
        return 0
    fi

    case "${words[1]}" in
        to|rm|compare|sync)
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

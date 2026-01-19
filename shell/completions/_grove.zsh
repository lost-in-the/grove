#compdef grove w

# Zsh completion for grove
# Copy this file to your zsh completions directory or use:
#   fpath=(~/.zsh/completions $fpath)
#   autoload -Uz compinit && compinit

_grove() {
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
            to|rm|resume)
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
                _describe 'type' '(pr:Fetch pull request issue:Fetch issue is:Fetch issue)'
                ;;
            issues|prs)
                _arguments \
                    '--state=[Filter by state]:state:(open closed all)' \
                    '--label=[Filter by label]:label:' \
                    '--assignee=[Filter by assignee]:assignee:' \
                    '--author=[Filter by author]:author:' \
                    '--limit=[Limit results]:limit:'
                ;;
            logs|restart)
                # Could complete with service names, but requires context
                ;;
            init)
                _values 'shell' 'zsh' 'bash'
                ;;
        esac
    fi
}

_grove "$@"

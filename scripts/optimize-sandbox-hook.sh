#!/usr/bin/env bash
# PreToolUse hook for the optimization loop sandbox.
# Blocks: git push, file access outside worktree, network tools.
# Receives JSON on stdin with tool_name and tool_input.
set -euo pipefail

WORKTREE_DIR="${GROVE_OPTIMIZE_DIR:-$(pwd)}"
input=$(cat)

tool_name=$(printf '%s' "$input" | python3 -c "import json,sys; print(json.load(sys.stdin).get('tool_name',''))" 2>/dev/null || echo "")

case "$tool_name" in
  Bash)
    command=$(printf '%s' "$input" | python3 -c "import json,sys; print(json.load(sys.stdin).get('tool_input',{}).get('command',''))" 2>/dev/null || echo "")

    # Block git push (any form)
    if printf '%s' "$command" | grep -qEi 'git\s+(push|remote\s+set-url)'; then
      echo "BLOCKED: git push/remote modification not allowed in optimization loop"
      exit 2
    fi

    # Block curl/wget/ssh (no network access needed)
    if printf '%s' "$command" | grep -qEi '(curl|wget|ssh|scp|rsync)\s'; then
      echo "BLOCKED: network commands not allowed in optimization loop"
      exit 2
    fi

    # Block rm -rf with absolute paths or parent traversal
    if printf '%s' "$command" | grep -qE 'rm\s+-[a-zA-Z]*r[a-zA-Z]*f?.*(/|\.\./)'; then
      echo "BLOCKED: recursive delete with absolute/parent path not allowed"
      exit 2
    fi
    ;;

  Write|Edit)
    file_path=$(printf '%s' "$input" | python3 -c "import json,sys; print(json.load(sys.stdin).get('tool_input',{}).get('file_path',''))" 2>/dev/null || echo "")

    # Resolve symlinks and check containment
    resolved=$(python3 -c "import os; print(os.path.realpath('$file_path'))" 2>/dev/null || echo "$file_path")
    if [[ "$resolved" != "$WORKTREE_DIR"* ]]; then
      echo "BLOCKED: write to $file_path resolves outside worktree ($WORKTREE_DIR)"
      exit 2
    fi
    ;;

  Read|Glob|Grep)
    # Allow reads — the OS sandbox handles filesystem boundaries
    ;;

  *)
    # Block any tool not in the allowlist (shouldn't happen with --allowedTools but defense in depth)
    echo "BLOCKED: tool $tool_name not allowed in optimization loop"
    exit 2
    ;;
esac

exit 0

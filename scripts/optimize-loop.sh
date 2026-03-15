#!/usr/bin/env bash
# Autonomous optimization loop for Grove.
# Creates a worktree, runs Claude in headless mode for N iterations,
# each making one focused improvement.
#
# Usage:
#   ./scripts/optimize-loop.sh [options]
#
# Options:
#   -n NUM    Number of iterations (default: 20)
#   -b NAME   Branch name (default: optimize/auto-YYYYMMDD-HHMM)
#   -d DIR    Worktree directory (default: ../grove-optimize)
#   -t SECS   Timeout per iteration in seconds (default: 600)
#   -c        Continue from existing worktree (don't create new one)
#   -h        Show this help
#
# Prerequisites:
#   - claude CLI in PATH
#   - go, golangci-lint in PATH
#   - git worktree support
set -euo pipefail

# Ensure common tool paths are available (homebrew, go, local bins)
export PATH="/opt/homebrew/bin:/usr/local/bin:$HOME/.local/bin:$HOME/go/bin:$(go env GOPATH 2>/dev/null)/bin:$PATH"

# macOS doesn't have GNU timeout — use a fallback that kills the process group
if ! command -v timeout > /dev/null; then
    timeout() {
        local duration=$1; shift
        # Run command in background, kill it if it exceeds duration
        "$@" &
        local pid=$!
        (
            sleep "$duration"
            kill -TERM "$pid" 2>/dev/null
            sleep 5
            kill -KILL "$pid" 2>/dev/null
        ) &
        local watchdog=$!
        wait "$pid" 2>/dev/null
        local exit_code=$?
        kill "$watchdog" 2>/dev/null
        wait "$watchdog" 2>/dev/null
        return $exit_code
    }
fi

# --- Defaults ---
MAX_ITERATIONS=20
BRANCH_NAME=""
WORKTREE_DIR=""
TIMEOUT=600
CONTINUE=false
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Parse args ---
while getopts "n:b:d:t:ch" opt; do
    case $opt in
        n) MAX_ITERATIONS="$OPTARG" ;;
        b) BRANCH_NAME="$OPTARG" ;;
        d) WORKTREE_DIR="$OPTARG" ;;
        t) TIMEOUT="$OPTARG" ;;
        c) CONTINUE=true ;;
        h)
            head -17 "$0" | tail -15
            exit 0
            ;;
        *) exit 1 ;;
    esac
done

# --- Derived values ---
TIMESTAMP=$(date +%Y%m%d-%H%M)
BRANCH_NAME="${BRANCH_NAME:-optimize/auto-$TIMESTAMP}"
WORKTREE_DIR="${WORKTREE_DIR:-$(dirname "$REPO_ROOT")/grove-optimize}"
LOG_FILE="$WORKTREE_DIR/optimize-run.log"

# --- Colors ---
green() { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }
red() { printf '\033[31m%s\033[0m\n' "$*"; }
dim() { printf '\033[2m%s\033[0m\n' "$*"; }

# --- Logging ---
log() {
    local msg="[$(date +%H:%M:%S)] $*"
    echo "$msg"
    echo "$msg" >> "$LOG_FILE" 2>/dev/null || true
}

# --- Setup worktree ---
setup_worktree() {
    if [ "$CONTINUE" = true ] && [ -d "$WORKTREE_DIR" ]; then
        green "Continuing in existing worktree: $WORKTREE_DIR"
        cd "$WORKTREE_DIR"
        BRANCH_NAME=$(git branch --show-current)
        return
    fi

    if [ -d "$WORKTREE_DIR" ]; then
        red "Worktree directory already exists: $WORKTREE_DIR"
        echo "Use -c to continue, or remove it first:"
        echo "  git worktree remove $WORKTREE_DIR"
        exit 1
    fi

    green "Creating worktree..."
    dim "  Branch: $BRANCH_NAME"
    dim "  Directory: $WORKTREE_DIR"

    cd "$REPO_ROOT"
    git worktree add -b "$BRANCH_NAME" "$WORKTREE_DIR" HEAD
    cd "$WORKTREE_DIR"

    # Trust mise config in the new worktree
    if command -v mise > /dev/null && [ -f mise.toml ]; then
        mise trust 2>/dev/null || true
    fi

    # Copy the optimization scripts and config
    cp "$REPO_ROOT/scripts/optimize-metrics.sh" scripts/
    cp "$REPO_ROOT/scripts/OPTIMIZE_PROMPT.md" scripts/
    cp "$REPO_ROOT/scripts/optimize-sandbox-hook.sh" scripts/
    cp "$REPO_ROOT/.golangci-optimize.yml" .

    # Create sandboxed Claude settings for this worktree
    mkdir -p .claude
    cat > .claude/settings.local.json <<SETTINGS
{
  "permissions": {
    "allow": [
      "Read",
      "Edit",
      "Write",
      "Glob",
      "Grep",
      "Bash"
    ],
    "deny": [
      "Agent",
      "Skill",
      "WebFetch",
      "WebSearch"
    ]
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "GROVE_OPTIMIZE_DIR=$WORKTREE_DIR $WORKTREE_DIR/scripts/optimize-sandbox-hook.sh"
          }
        ]
      }
    ]
  }
}
SETTINGS

    green "Worktree ready (sandboxed)."
}

# --- Verify prerequisites ---
check_prereqs() {
    local missing=()
    command -v claude > /dev/null || missing+=("claude")
    command -v go > /dev/null || missing+=("go")

    if [ ${#missing[@]} -gt 0 ]; then
        red "Missing prerequisites: ${missing[*]}"
        exit 1
    fi

    # Install golangci-lint if missing
    if ! command -v golangci-lint > /dev/null; then
        yellow "Installing golangci-lint..."
        go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
        export PATH="$(go env GOPATH)/bin:$PATH"
        if ! command -v golangci-lint > /dev/null; then
            red "Failed to install golangci-lint"
            exit 1
        fi
        green "golangci-lint installed."
    fi

    # Verify go test works
    if ! go build ./cmd/grove 2>/dev/null; then
        red "go build failed — fix build errors first"
        exit 1
    fi
    green "Prerequisites OK."
}

# --- Capture baseline ---
capture_baseline() {
    log "Capturing baseline metrics..."
    chmod +x scripts/optimize-metrics.sh
    if ./scripts/optimize-metrics.sh "$WORKTREE_DIR/baseline-metrics.json"; then
        log "Baseline captured."
        cat "$WORKTREE_DIR/baseline-metrics.json"
    else
        yellow "Baseline capture had errors (continuing anyway)"
    fi
}

# --- Run one iteration ---
run_iteration() {
    local i=$1
    local total=$2

    echo ""
    green "━━━ Iteration $i/$total ━━━"
    log "Starting iteration $i"

    local before_sha
    before_sha=$(git rev-parse --short HEAD)

    # Run Claude in headless mode with the optimization prompt.
    # Sandbox layers:
    #   1. Script cd's to worktree before invoking claude
    #   2. --allowedTools restricts to file ops + bash (no agents, web, MCP)
    #   3. PreToolUse hook validates paths stay in worktree, blocks git push/network
    #   4. OS-level sandbox restricts filesystem writes to worktree + tmp
    #   5. --dangerously-skip-permissions auto-approves within the above constraints
    local claude_exit=0
    timeout "${TIMEOUT}" claude -p \
        --dangerously-skip-permissions \
        --allowedTools "Read" --allowedTools "Write" --allowedTools "Edit" \
        --allowedTools "Glob" --allowedTools "Grep" --allowedTools "Bash" \
        -- \
        "$(cat scripts/OPTIMIZE_PROMPT.md)

## Session context
- This is iteration $i of $total in the optimization loop.
- Working directory: $(pwd)
- Current branch: $(git branch --show-current)
- Previous commit: $before_sha
- Read optimize-activity.log to see what has already been done.
- Focus on the highest-impact change you can find that hasn't been attempted yet.
- IMPORTANT: All file paths must be within $(pwd). Do not access files outside this directory." \
        2>> "$LOG_FILE" || claude_exit=$?

    local after_sha
    after_sha=$(git rev-parse --short HEAD)

    if [ "$before_sha" = "$after_sha" ]; then
        yellow "  Iteration $i: no commit produced"
        log "Iteration $i: no commit (exit code $claude_exit)"
        return 1
    fi

    # Safety check: verify no remote push occurred
    local remote_check
    remote_check=$(git reflog --since="5 minutes ago" | grep -c "push" || true)
    if [ "$remote_check" -gt 0 ]; then
        red "  WARNING: detected git push in reflog — stopping loop"
        log "Iteration $i: STOPPED — git push detected"
        exit 1
    fi

    # Verify the commit didn't break anything
    if ! go test -count=1 -timeout 120s ./... > "$WORKTREE_DIR/verify-test.out" 2>&1; then
        red "  Iteration $i: commit $after_sha broke tests — reverting"
        log "Iteration $i: REVERTED $after_sha (tests failed)"
        log "  Test output: $(grep -E 'FAIL|panic|Error' "$WORKTREE_DIR/verify-test.out" | head -5)"
        git reset --hard HEAD~1
        return 1
    fi

    green "  Iteration $i: committed $after_sha"
    log "Iteration $i: OK $after_sha — $(git log -1 --format='%s')"
    return 0
}

# --- Summary ---
print_summary() {
    local start_sha=$1
    echo ""
    green "━━━ Optimization Complete ━━━"
    echo ""

    local commits
    commits=$(git log --oneline "$start_sha"..HEAD 2>/dev/null | wc -l | tr -d ' ')
    echo "Commits: $commits"
    echo "Branch:  $(git branch --show-current)"
    echo "Worktree: $WORKTREE_DIR"
    echo ""

    if [ "$commits" -gt 0 ]; then
        echo "Changes:"
        git log --oneline "$start_sha"..HEAD
        echo ""
        echo "Review with:"
        echo "  cd $WORKTREE_DIR"
        echo "  git log --stat $start_sha..HEAD"
        echo "  git diff $start_sha..HEAD"
        echo ""
        echo "Merge back:"
        echo "  cd $REPO_ROOT"
        echo "  git merge $BRANCH_NAME"
    else
        yellow "No improvements were committed."
    fi

    # Capture final metrics if any commits were made
    if [ "$commits" -gt 0 ]; then
        echo ""
        echo "Final metrics:"
        ./scripts/optimize-metrics.sh "$WORKTREE_DIR/final-metrics.json" 2>/dev/null
        cat "$WORKTREE_DIR/final-metrics.json"
    fi
}

# --- Main ---
main() {
    echo ""
    green "Grove Optimization Loop"
    dim "Iterations: $MAX_ITERATIONS | Timeout: ${TIMEOUT}s each"
    echo ""

    setup_worktree
    mkdir -p "$WORKTREE_DIR"
    check_prereqs

    local start_sha
    start_sha=$(git rev-parse --short HEAD)

    capture_baseline

    local successes=0
    local failures=0
    local consecutive_failures=0

    for i in $(seq 1 "$MAX_ITERATIONS"); do
        if run_iteration "$i" "$MAX_ITERATIONS"; then
            successes=$((successes + 1))
            consecutive_failures=0
        else
            failures=$((failures + 1))
            consecutive_failures=$((consecutive_failures + 1))
        fi

        # Stop after 3 consecutive failures — agent is stuck
        if [ "$consecutive_failures" -ge 3 ]; then
            yellow "3 consecutive failures — stopping early"
            log "Stopped: 3 consecutive failures"
            break
        fi
    done

    log "Finished: $successes successes, $failures failures"
    print_summary "$start_sha"
}

main

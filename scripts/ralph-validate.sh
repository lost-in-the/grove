#!/usr/bin/env bash
set -euo pipefail

# Ralph Wiggum Validation Loop for grove-cli
# Usage: ./scripts/ralph-validate.sh [max_iterations]
#
# Validates the entire grove-cli system per IMPLEMENTATION_PLAN.md
# Fresh Claude context each iteration prevents hallucination
#
# Based on: https://github.com/JeredBlu/guides/blob/main/Ralph_Wiggum_Guide.md

# Parse arguments
show_help() {
    echo "Usage: ./scripts/ralph-validate.sh [OPTIONS] [max_iterations]"
    echo ""
    echo "Validates grove-cli using the Ralph Wiggum iteration pattern."
    echo "Each iteration launches a fresh Claude context to validate one task."
    echo ""
    echo "Arguments:"
    echo "  max_iterations    Maximum validation iterations (default: 20)"
    echo ""
    echo "Options:"
    echo "  -h, --help        Show this help message"
    echo "  --reset           Reset plan.md and activity.md to initial state"
    echo ""
    echo "Exit codes:"
    echo "  0    All validations passed"
    echo "  1    User action required (issues found)"
    echo "  2    Blocked (cannot continue)"
    echo ""
    echo "State files:"
    echo "  scripts/plan.md      Task tracking (JSON)"
    echo "  scripts/activity.md  Progress log"
    echo "  scripts/PROMPT.md    Claude instructions"
    exit 0
}

# Handle arguments
case "${1:-}" in
    -h|--help)
        show_help
        ;;
    --reset)
        rm -f scripts/plan.md scripts/activity.md
        echo "Reset state files - they will be regenerated on next run"
        exit 0
        ;;
esac

MAX_ITERATIONS="${1:-20}"

# Validate max_iterations is a number
if ! [[ "$MAX_ITERATIONS" =~ ^[0-9]+$ ]]; then
    echo "Error: max_iterations must be a positive integer"
    echo "Run with --help for usage"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Initialize state files if needed
if [[ ! -f "scripts/plan.md" ]]; then
    cat > "scripts/plan.md" << 'PLAN'
[
  {"category": "phase0", "description": "Validate Phase 0: Foundation commands", "steps": ["Check grove ls", "Check grove new", "Check grove to", "Check grove rm", "Check grove here", "Check grove last", "Check shell integration", "Check hook system"], "passes": false},
  {"category": "phase1", "description": "Validate Phase 1: Docker plugin", "steps": ["Check grove up", "Check grove down", "Check grove logs", "Check grove restart"], "passes": false},
  {"category": "phase2", "description": "Validate Phase 2: State management", "steps": ["Check grove freeze", "Check grove resume", "Check dirty detection", "Check state persistence"], "passes": false},
  {"category": "phase3", "description": "Validate Phase 3: Time tracking", "steps": ["Check grove time", "Check grove time --all", "Check grove time week"], "passes": false},
  {"category": "phase4", "description": "Validate Phase 4: Issue integration", "steps": ["Check grove fetch", "Check grove issues", "Check grove prs", "Check grove browse"], "passes": false},
  {"category": "phase5", "description": "Validate Phase 5: Polish", "steps": ["Check README.md", "Check CONTRIBUTING.md", "Check GoReleaser", "Check completions"], "passes": false},
  {"category": "coverage", "description": "Validate test coverage meets 80% target", "steps": ["Run go test -cover", "Parse coverage per package", "Flag packages below 80%"], "passes": false},
  {"category": "tests", "description": "Validate all tests pass", "steps": ["Run make test", "Check for failures", "Verify race detector passes"], "passes": false},
  {"category": "lint", "description": "Validate linting passes", "steps": ["Run go vet", "Run golangci-lint", "Check gofmt compliance"], "passes": false},
  {"category": "ci", "description": "Validate CI configuration", "steps": ["Check ci.yml exists", "Check release.yml exists", "Verify workflows have required jobs"], "passes": false},
  {"category": "cleanup", "description": "Identify unnecessary files", "steps": ["Find orphaned files", "Check for temp/debug files", "Verify .gitignore coverage"], "passes": false},
  {"category": "practices", "description": "Verify best practices", "steps": ["Check error handling", "Check documentation", "Verify no panic() usage"], "passes": false}
]
PLAN
    echo -e "${BLUE}Created scripts/plan.md with 12 validation tasks${NC}"
fi

if [[ ! -f "scripts/activity.md" ]]; then
    cat > "scripts/activity.md" << 'ACTIVITY'
# Grove-CLI Validation - Activity Log

**Last Updated:** (not started)
**Tasks Completed:** 0/12
**Current Task:** (none)

## Session Log

ACTIVITY
    echo -e "${BLUE}Created scripts/activity.md${NC}"
fi

# Check if Claude CLI is available
if ! command -v claude &> /dev/null; then
    echo -e "${RED}Error: Claude CLI not found${NC}"
    echo "Please install Claude Code: https://claude.ai/code"
    exit 1
fi

# Check if PROMPT.md exists
if [[ ! -f "scripts/PROMPT.md" ]]; then
    echo -e "${RED}Error: scripts/PROMPT.md not found${NC}"
    echo "Please create the prompt file first"
    exit 1
fi

echo ""
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  GROVE-CLI RALPH WIGGUM VALIDATION${NC}"
echo -e "${BLUE}  Max iterations: $MAX_ITERATIONS${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Show current progress
if command -v jq &> /dev/null; then
    COMPLETED=$(jq '[.[] | select(.passes == true)] | length' scripts/plan.md 2>/dev/null || echo "?")
    TOTAL=$(jq 'length' scripts/plan.md 2>/dev/null || echo "?")
    echo -e "Progress: ${GREEN}$COMPLETED${NC}/${TOTAL} tasks complete"
    echo ""
fi

for ((i=1; i<=$MAX_ITERATIONS; i++)); do
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}  Iteration $i of $MAX_ITERATIONS${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # Run Claude with fresh context
    # Using --print for non-interactive mode
    # --dangerously-skip-permissions bypasses all permission checks
    # Safe when running in a sandboxed environment
    result=$(claude -p "$(cat scripts/PROMPT.md)" \
        --output-format text \
        --dangerously-skip-permissions \
        2>&1) || true

    echo "$result"

    # Check for completion promise
    if [[ "$result" == *"<promise>COMPLETE</promise>"* ]]; then
        echo ""
        echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
        echo -e "${GREEN}  VALIDATION COMPLETE - All checks passed!${NC}"
        echo -e "${GREEN}═══════════════════════════════════════════════════════════════${NC}"
        exit 0
    fi

    # Check for user action required
    if [[ "$result" == *"<promise>USER_ACTION_REQUIRED</promise>"* ]]; then
        echo ""
        echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
        echo -e "${YELLOW}  USER ACTION REQUIRED${NC}"
        echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
        echo ""
        echo "Review scripts/activity.md for details on what needs fixing."
        echo "After making fixes, re-run: ./scripts/ralph-validate.sh"
        exit 1
    fi

    # Check for blocked state
    if [[ "$result" == *"<promise>BLOCKED</promise>"* ]]; then
        echo ""
        echo -e "${RED}═══════════════════════════════════════════════════════════════${NC}"
        echo -e "${RED}  BLOCKED - Cannot continue${NC}"
        echo -e "${RED}═══════════════════════════════════════════════════════════════${NC}"
        echo ""
        echo "Review scripts/activity.md for details"
        exit 2
    fi

    # Brief pause between iterations
    sleep 1
done

echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}  Max iterations reached ($MAX_ITERATIONS)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo "Review scripts/plan.md and scripts/activity.md for current state"
exit 1

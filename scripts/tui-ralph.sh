#!/usr/bin/env bash
set -euo pipefail

#######################################################################
# TUI Ralph Wiggum - Autonomous TUI Implementation Loop
#
# Usage:
#   ./scripts/tui-ralph.sh [max_iterations] [--dry-run]
#
# Examples:
#   ./scripts/tui-ralph.sh           # Default 25 iterations
#   ./scripts/tui-ralph.sh 50        # 50 iterations
#   ./scripts/tui-ralph.sh --dry-run # Show prompt without running
#
#######################################################################

# Configuration
MAX_ITERATIONS="${1:-25}"
DRY_RUN=false
PROMPT_FILE="prompts/tui-agent.md"
LOG_DIR="logs/tui-ralph"
COMPLETION_PROMISE="TUI_REDESIGN_COMPLETE"

# Check for dry run flag
if [[ "${1:-}" == "--dry-run" ]] || [[ "${2:-}" == "--dry-run" ]]; then
    DRY_RUN=true
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# Ensure we're in the project root
if [[ ! -f "CLAUDE.md" ]]; then
    echo -e "${RED}Error: Must run from project root (where CLAUDE.md exists)${NC}"
    exit 1
fi

# Ensure prompt file exists
if [[ ! -f "$PROMPT_FILE" ]]; then
    echo -e "${RED}Error: Prompt file not found: $PROMPT_FILE${NC}"
    exit 1
fi

# Create log directory
mkdir -p "$LOG_DIR"

# Load the prompt
PROMPT=$(cat "$PROMPT_FILE")

# Dry run mode
if [[ "$DRY_RUN" == true ]]; then
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}DRY RUN - Would use this prompt:${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo "$PROMPT"
    exit 0
fi

# Header
echo -e "${PURPLE}"
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           TUI Ralph Wiggum Implementation Loop            ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo -e "${NC}"
echo -e "Max iterations: ${YELLOW}$MAX_ITERATIONS${NC}"
echo -e "Log directory:  ${YELLOW}$LOG_DIR${NC}"
echo -e "Completion:     ${YELLOW}$COMPLETION_PROMISE${NC}"
echo ""

# Track progress
TASKS_COMPLETED=0
START_TIME=$(date +%s)

# Main loop
for ITERATION in $(seq 1 "$MAX_ITERATIONS"); do
    TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')
    LOG_FILE="$LOG_DIR/iteration-$(printf '%03d' $ITERATION).log"

    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Iteration $ITERATION of $MAX_ITERATIONS${NC} - $TIMESTAMP"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Run Claude with the prompt
    echo -e "${YELLOW}Running Claude...${NC}"

    # Capture output and exit code
    set +e
    OUTPUT=$(claude -p "$PROMPT" 2>&1)
    EXIT_CODE=$?
    set -e

    # Log full output
    echo "$OUTPUT" > "$LOG_FILE"
    echo -e "${GREEN}Logged to: $LOG_FILE${NC}"

    # Check for task completion
    if echo "$OUTPUT" | grep -q "<task-complete>"; then
        TASKS_COMPLETED=$((TASKS_COMPLETED + 1))
        TASK_INFO=$(echo "$OUTPUT" | sed -n '/<task-complete>/,/<\/task-complete>/p' | head -5)
        echo -e "${GREEN}✓ Task completed!${NC}"
        echo "$TASK_INFO" | grep -v "task-complete" | head -3
        echo ""
    fi

    # Check for completion promise
    if echo "$OUTPUT" | grep -q "<promise>$COMPLETION_PROMISE</promise>"; then
        END_TIME=$(date +%s)
        DURATION=$((END_TIME - START_TIME))

        echo ""
        echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
        echo -e "${GREEN}║              TUI REDESIGN COMPLETE!                       ║${NC}"
        echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
        echo ""
        echo -e "Iterations:      ${YELLOW}$ITERATION${NC}"
        echo -e "Tasks completed: ${YELLOW}$TASKS_COMPLETED${NC}"
        echo -e "Duration:        ${YELLOW}$((DURATION / 60))m $((DURATION % 60))s${NC}"
        echo ""
        exit 0
    fi

    # Check for blocked state
    if echo "$OUTPUT" | grep -q "<promise>BLOCKED</promise>"; then
        echo -e "${RED}⚠ Agent is blocked${NC}"
        echo "$OUTPUT" | grep -A 5 "<promise>BLOCKED</promise>" | head -5
        echo ""
        echo -e "${YELLOW}Continuing to next iteration to try different approach...${NC}"
    fi

    # Check for errors
    if [[ $EXIT_CODE -ne 0 ]]; then
        echo -e "${RED}⚠ Claude exited with code $EXIT_CODE${NC}"
        echo -e "${YELLOW}Continuing...${NC}"
    fi

    # Brief pause between iterations
    echo ""
    echo -e "${BLUE}Pausing 3s before next iteration...${NC}"
    sleep 3
done

# Max iterations reached
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

echo ""
echo -e "${YELLOW}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║              MAX ITERATIONS REACHED                       ║${NC}"
echo -e "${YELLOW}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "Iterations:      ${YELLOW}$MAX_ITERATIONS${NC}"
echo -e "Tasks completed: ${YELLOW}$TASKS_COMPLETED${NC}"
echo -e "Duration:        ${YELLOW}$((DURATION / 60))m $((DURATION % 60))s${NC}"
echo ""
echo -e "Check ${BLUE}docs/TUI_IMPLEMENTATION_SPEC.md${NC} for current progress."
echo -e "Logs available in ${BLUE}$LOG_DIR/${NC}"
echo ""
exit 1

#!/usr/bin/env bash
set -euo pipefail

# Ralph Wiggum Validation - Docker Sandboxed Runner
#
# Runs validation in an isolated Docker container where:
# - Only the project directory is mounted (no access to ~/Work or other repos)
# - Claude CLI runs inside the container
# - Results are written back to the mounted volume
#
# Usage: ./scripts/ralph-docker.sh [max_iterations]
#
# Prerequisites:
#   - Docker Desktop installed and running
#   - ANTHROPIC_API_KEY environment variable set

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
MAX_ITERATIONS="${1:-20}"
IMAGE_NAME="grove-validate"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

show_help() {
    echo "Usage: ./scripts/ralph-docker.sh [OPTIONS] [max_iterations]"
    echo ""
    echo "Run Ralph Wiggum validation in a Docker sandbox."
    echo "Container only has access to the project directory - no host filesystem access."
    echo ""
    echo "Arguments:"
    echo "  max_iterations    Maximum validation iterations (default: 20)"
    echo ""
    echo "Options:"
    echo "  -h, --help        Show this help"
    echo "  --rebuild         Force rebuild the Docker image"
    echo "  --shell           Drop into container shell instead of running validation"
    echo ""
    echo "Authentication (one of):"
    echo "  Claude Code subscription - uses ~/.claude/ credentials automatically"
    echo "  ANTHROPIC_API_KEY        - for API users (optional)"
    echo ""
    echo "Security:"
    echo "  - Container runs as non-root user"
    echo "  - Only /workspace is mounted (project directory)"
    echo "  - No access to host home directory or other projects"
    exit 0
}

# Parse arguments
REBUILD=false
SHELL_MODE=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)
            show_help
            ;;
        --rebuild)
            REBUILD=true
            shift
            ;;
        --shell)
            SHELL_MODE=true
            shift
            ;;
        *)
            MAX_ITERATIONS="$1"
            shift
            ;;
    esac
done

# Validate max_iterations
if ! [[ "$MAX_ITERATIONS" =~ ^[0-9]+$ ]]; then
    echo -e "${RED}Error: max_iterations must be a positive integer${NC}"
    exit 1
fi

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker not found${NC}"
    echo "Install Docker Desktop: https://www.docker.com/products/docker-desktop"
    exit 1
fi

if ! docker info &> /dev/null 2>&1; then
    echo -e "${RED}Error: Docker daemon not running${NC}"
    echo "Please start Docker Desktop"
    exit 1
fi

# Check authentication
# Claude Code subscription uses ~/.claude/ credentials
# API users can set ANTHROPIC_API_KEY
AUTH_MODE=""
if [[ -n "${ANTHROPIC_API_KEY:-}" ]]; then
    AUTH_MODE="api"
elif [[ -d "$HOME/.claude" ]]; then
    AUTH_MODE="subscription"
else
    echo -e "${RED}Error: No Claude authentication found${NC}"
    echo ""
    echo "For Claude Code subscription:"
    echo "  Run 'claude' once to authenticate, then retry"
    echo ""
    echo "For API users:"
    echo "  export ANTHROPIC_API_KEY='your-key-here'"
    exit 1
fi

echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  DOCKER SANDBOXED VALIDATION${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "Project:    ${GREEN}$PROJECT_ROOT${NC}"
echo -e "Image:      ${GREEN}$IMAGE_NAME${NC}"
echo -e "Iterations: ${GREEN}$MAX_ITERATIONS${NC}"
echo ""
echo -e "${YELLOW}Security: Container has NO access to files outside project${NC}"
echo ""

# Build image if needed
if [[ "$REBUILD" == "true" ]] || ! docker image inspect "$IMAGE_NAME" &> /dev/null; then
    echo -e "${BLUE}Building Docker image...${NC}"
    docker build -f "$SCRIPT_DIR/Dockerfile.validate" -t "$IMAGE_NAME" "$PROJECT_ROOT"
    echo ""
fi

# Reset state files for clean run
if [[ -f "$PROJECT_ROOT/scripts/plan.md" ]]; then
    echo -e "${YELLOW}Resetting state files for clean validation run...${NC}"
    rm -f "$PROJECT_ROOT/scripts/plan.md" "$PROJECT_ROOT/scripts/activity.md"
    echo ""
fi

echo -e "${GREEN}Starting sandboxed validation...${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Run container
# Security notes:
#   --rm                    Remove container after exit
#   --read-only             Root filesystem is read-only (except mounted volume)
#   --tmpfs /tmp            Writable /tmp in memory only
#   -v project:/workspace   Only project dir is accessible
#   -e ANTHROPIC_API_KEY    Pass API key to container
#   --network host          Allow API access (required for Claude)

# Build docker run arguments based on auth mode
DOCKER_AUTH_ARGS=()
if [[ "$AUTH_MODE" == "api" ]]; then
    echo -e "Auth: ${GREEN}API Key${NC}"
    DOCKER_AUTH_ARGS+=(-e "ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY")
else
    echo -e "Auth: ${GREEN}Claude Code Subscription (~/.claude/)${NC}"
    # Mount Claude credentials directory (read-write required for debug logs, cache, etc.)
    DOCKER_AUTH_ARGS+=(-v "$HOME/.claude:/home/validator/.claude:rw")
fi
echo ""

if [[ "$SHELL_MODE" == "true" ]]; then
    echo "Dropping into container shell..."
    docker run --rm -it \
        --tmpfs /tmp \
        -v "$PROJECT_ROOT:/workspace:rw" \
        "${DOCKER_AUTH_ARGS[@]}" \
        --network host \
        --entrypoint /bin/bash \
        "$IMAGE_NAME"
else
    docker run --rm -it \
        --tmpfs /tmp \
        -v "$PROJECT_ROOT:/workspace:rw" \
        "${DOCKER_AUTH_ARGS[@]}" \
        --network host \
        "$IMAGE_NAME" "$MAX_ITERATIONS"
fi

exit_code=$?

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ $exit_code -eq 0 ]]; then
    echo -e "${GREEN}Validation completed successfully!${NC}"
else
    echo -e "${YELLOW}Validation stopped with exit code: $exit_code${NC}"
    echo "Check scripts/activity.md for details"
fi

exit $exit_code

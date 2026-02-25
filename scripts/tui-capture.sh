#!/usr/bin/env bash
# tui-capture.sh — Capture TUI output via tmux for agent visual iteration.
# Usage: scripts/tui-capture.sh [-w WIDTH] [-h HEIGHT] [-k KEYS] [-o OUTPUT] [-d DELAY] [--no-build]
#
# Requires: tmux, test fixture (make test-fixture)

set -euo pipefail

# Defaults
WIDTH=80
HEIGHT=24
KEYS=""
OUTPUT="tmp/tui-capture.txt"
DELAY=2
BUILD=true
FIXTURE_DIR="/tmp/grove-test-fixture/rails-app"
SESSION="grove-capture-$$"

usage() {
  cat <<EOF
Usage: $(basename "$0") [options]

Options:
  -w WIDTH    Terminal width (default: 80)
  -h HEIGHT   Terminal height (default: 24)
  -k KEYS     Space-separated key sequence (e.g. "j j Enter")
  -o OUTPUT   Output file (default: tmp/tui-capture.txt)
  -d DELAY    Render delay in seconds (default: 2)
  --no-build  Skip building the binary
  --help      Show this help
EOF
  exit 0
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    -w) WIDTH="$2"; shift 2 ;;
    -h) HEIGHT="$2"; shift 2 ;;
    -k) KEYS="$2"; shift 2 ;;
    -o) OUTPUT="$2"; shift 2 ;;
    -d) DELAY="$2"; shift 2 ;;
    --no-build) BUILD=false; shift ;;
    --help) usage ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

# Cleanup on exit
cleanup() {
  tmux kill-session -t "$SESSION" 2>/dev/null || true
}
trap cleanup EXIT

# Validate fixture exists
if [ ! -d "$FIXTURE_DIR" ]; then
  echo "Test fixture not found at $FIXTURE_DIR" >&2
  echo "Run: make test-fixture" >&2
  exit 1
fi

# Build if requested
if [ "$BUILD" = true ]; then
  echo "Building grove..." >&2
  make -C "$(dirname "$0")/.." build >/dev/null 2>&1
fi

BINARY="$(dirname "$0")/../bin/grove"
if [ ! -x "$BINARY" ]; then
  echo "Binary not found at $BINARY" >&2
  exit 1
fi
BINARY="$(cd "$(dirname "$BINARY")" && pwd)/$(basename "$BINARY")"

# Create output directory
mkdir -p "$(dirname "$OUTPUT")"

# Start tmux session with specified size
tmux new-session -d -s "$SESSION" -x "$WIDTH" -y "$HEIGHT" -c "$FIXTURE_DIR"

# Launch grove in the tmux session
tmux send-keys -t "$SESSION" "$BINARY" Enter

# Wait for initial render
sleep "$DELAY"

# Send key sequence if provided
if [ -n "$KEYS" ]; then
  for key in $KEYS; do
    case "$key" in
      Enter)  tmux send-keys -t "$SESSION" Enter ;;
      Escape) tmux send-keys -t "$SESSION" Escape ;;
      Space)  tmux send-keys -t "$SESSION" Space ;;
      Tab)    tmux send-keys -t "$SESSION" Tab ;;
      Up)     tmux send-keys -t "$SESSION" Up ;;
      Down)   tmux send-keys -t "$SESSION" Down ;;
      Left)   tmux send-keys -t "$SESSION" Left ;;
      Right)  tmux send-keys -t "$SESSION" Right ;;
      *)      tmux send-keys -t "$SESSION" "$key" ;;
    esac
    sleep 0.3
  done
  # Wait for render after keys
  sleep 1
fi

# Capture the pane content
tmux capture-pane -t "$SESSION" -p > "$OUTPUT"

echo "$OUTPUT" >&2
